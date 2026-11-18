package check

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// testTimeout bounds every request in these tests. It is generous: a passing
// test never waits for it.
const testTimeout = 10 * time.Second

// sseEvents is the stream the SSE upstream writes: three JSON events and a
// terminator, each flushed on its own.
var sseEvents = []string{
	"data: {\"n\":1}\n\n",
	"data: {\"n\":2}\n\n",
	"data: {\"n\":3}\n\n",
	"data: [DONE]\n\n",
}

// upstream is a test backend. It counts the requests that reach it.
type upstream struct {
	hits atomic.Int64
}

// ServeHTTP answers /sse with a flushed event stream, and everything else with
// a fixed 64-byte text body.
func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.hits.Add(1)
	if r.URL.Path == "/sse" {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, e := range sseEvents {
			_, _ = io.WriteString(w, e)
			http.NewResponseController(w).Flush() //nolint:errcheck // not every writer flushes
		}
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = io.WriteString(w, plainBody)
}

var plainBody = strings.Repeat("0123456789abcdef", 4)

// newClient returns a client with its own transport, so no connection outlives
// the test.
func newClient(t *testing.T, wrap func(http.RoundTripper) http.RoundTripper) *http.Client {
	t.Helper()
	base := &http.Transport{DisableKeepAlives: true}
	t.Cleanup(base.CloseIdleConnections)
	var rt http.RoundTripper = base
	if wrap != nil {
		rt = wrap(base)
	}
	return &http.Client{Transport: rt}
}

// transportSide returns a client whose transport is the fault transport on
// point "p", talking to a plain server, and the upstream behind it.
func transportSide(t *testing.T) (*http.Client, string, *upstream) {
	t.Helper()
	WithFaults(t)
	up := &upstream{}
	srv := httptest.NewServer(up)
	t.Cleanup(srv.Close)
	c := newClient(t, func(b http.RoundTripper) http.RoundTripper { return FaultTransport("p", b) })
	return c, srv.URL, up
}

// handlerSide returns a plain client talking to a server wrapped in the fault
// handler on point "p", and the upstream behind it.
func handlerSide(t *testing.T) (*http.Client, string, *upstream) {
	t.Helper()
	WithFaults(t)
	up := &upstream{}
	srv := httptest.NewServer(FaultHandler("p", up))
	t.Cleanup(srv.Close)
	return newClient(t, nil), srv.URL, up
}

type side struct {
	name string
	make func(*testing.T) (*http.Client, string, *upstream)
}

var bothSides = []side{{"transport", transportSide}, {"handler", handlerSide}}

func get(ctx context.Context, c *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)
	return ctx
}

// readAll reads a body to its end and closes it.
func readAll(t *testing.T, resp *http.Response) (string, error) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func isReset(err error) bool {
	return errors.Is(err, syscall.ECONNRESET)
}

func TestHTTPFailBeforeHeaders(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, up := s.make(t)
			ErrCfg("p", HTTP(FailBeforeHeaders()))
			resp, err := get(testCtx(t), c, url)
			if err == nil {
				_ = resp.Body.Close()
				t.Fatal("request succeeded")
			}
			if s.name == "transport" {
				if !errors.Is(err, ErrFault) {
					t.Errorf("error does not wrap ErrFault: %v", err)
				}
			} else if !errors.Is(err, io.EOF) && !isReset(err) {
				t.Errorf("want EOF (or reset), got %v", err)
			}
			Eq(up.hits.Load(), 0, "upstream was called")
			Eq(ErrStats("p"), FaultStats{Checks: 1, Fired: 1})
		})
	}
}

func TestHTTPFailBeforeHeadersWithError(t *testing.T) {
	c, url, up := transportSide(t)
	boom := errors.New("boom")
	ErrCfg("p", HTTP(FailBeforeHeaders()), WithError(boom))
	_, err := get(testCtx(t), c, url)
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	Eq(up.hits.Load(), 0)
}

func TestHTTPStatus(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, up := s.make(t)
			ErrCfg("p", HTTP(StatusFault(503)))
			resp, err := get(testCtx(t), c, url)
			if err != nil {
				t.Fatal(err)
			}
			Eq(resp.StatusCode, 503)
			body, err := readAll(t, resp)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body, "503") || !strings.Contains(body, `"p"`) {
				t.Errorf("unexpected body %q", body)
			}
			Eq(up.hits.Load(), 0, "upstream was called")
			Eq(ErrStats("p").Fired, 1)
		})
	}
}

func TestHTTPStallHeadersDeadline(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, up := s.make(t)
			ErrCfg("p", HTTP(StallHeaders(time.Hour)))
			ctx, cancel := context.WithTimeout(testCtx(t), 100*time.Millisecond)
			defer cancel()
			start := time.Now()
			resp, err := get(ctx, c, url)
			if err == nil {
				_ = resp.Body.Close()
				t.Fatal("request succeeded")
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("want DeadlineExceeded, got %v", err)
			}
			LT(time.Since(start), 5*time.Second, "stall ignored the deadline")
			Eq(up.hits.Load(), 0, "upstream was called during the stall")
		})
	}
}

func TestHTTPStallHeadersThenProceeds(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, up := s.make(t)
			const stall = 150 * time.Millisecond
			ErrCfg("p", HTTP(StallHeaders(stall)))
			start := time.Now()
			resp, err := get(testCtx(t), c, url)
			if err != nil {
				t.Fatal(err)
			}
			GTE(time.Since(start), stall, "headers arrived before the stall ended")
			body, err := readAll(t, resp)
			if err != nil {
				t.Fatal(err)
			}
			Eq(body, plainBody)
			Eq(up.hits.Load(), 1)
		})
	}
}

func TestHTTPTruncateBody(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, up := s.make(t)
			ErrCfg("p", HTTP(TruncateBody(10)))
			resp, err := get(testCtx(t), c, url)
			if err != nil {
				t.Fatal(err)
			}
			Eq(resp.StatusCode, 200)
			body, err := readAll(t, resp)
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Errorf("want io.ErrUnexpectedEOF, got %v", err)
			}
			Eq(body, plainBody[:10])
			Eq(up.hits.Load(), 1)
		})
	}
}

func TestHTTPTruncateBodyZero(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, _ := s.make(t)
			ErrCfg("p", HTTP(TruncateBody(0)))
			resp, err := get(testCtx(t), c, url)
			if err != nil {
				t.Fatal(err)
			}
			Eq(resp.StatusCode, 200)
			body, err := readAll(t, resp)
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Errorf("want io.ErrUnexpectedEOF, got %v", err)
			}
			Eq(body, "")
		})
	}
}

// A body shorter than the cut is not truncated.
func TestHTTPTruncateBodyLongerThanBody(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, _ := s.make(t)
			ErrCfg("p", HTTP(TruncateBody(int64(len(plainBody)))))
			resp, err := get(testCtx(t), c, url)
			if err != nil {
				t.Fatal(err)
			}
			body, err := readAll(t, resp)
			if err != nil {
				t.Fatal(err)
			}
			Eq(body, plainBody)
		})
	}
}

// Truncating an SSE stream mid-event, as a dying backend does.
func TestHTTPTruncateSSE(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, _ := s.make(t)
			cut := len(sseEvents[0]) + 5
			ErrCfg("p", HTTP(TruncateBody(int64(cut))))
			resp, err := get(testCtx(t), c, url+"/sse")
			if err != nil {
				t.Fatal(err)
			}
			body, err := readAll(t, resp)
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Errorf("want io.ErrUnexpectedEOF, got %v", err)
			}
			Eq(body, strings.Join(sseEvents, "")[:cut])
		})
	}
}

func TestHTTPResetConn(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, up := s.make(t)
			ErrCfg("p", HTTP(ResetConn()))
			resp, err := get(testCtx(t), c, url)
			if err == nil {
				_ = resp.Body.Close()
				t.Fatal("request succeeded")
			}
			// The transport reports ECONNRESET itself. Over a real socket
			// the kernel decides between a reset and an EOF.
			if !isReset(err) && (s.name == "transport" || !errors.Is(err, io.EOF)) {
				t.Errorf("want a connection reset, got %v", err)
			}
			Eq(up.hits.Load(), 0, "upstream was called")
		})
	}
}

// malformed is what the client should see for MalformedSSE(2).
func malformed(after int) string {
	return strings.Join(sseEvents[:after], "") + MalformedSSEFrame + strings.Join(sseEvents[after:], "")
}

func TestHTTPMalformedSSE(t *testing.T) {
	for _, s := range bothSides {
		for _, after := range []int{0, 2, len(sseEvents)} {
			t.Run(fmt.Sprintf("%s/after%d", s.name, after), func(t *testing.T) {
				c, url, _ := s.make(t)
				ErrCfg("p", HTTP(MalformedSSE(after)))
				resp, err := get(testCtx(t), c, url+"/sse")
				if err != nil {
					t.Fatal(err)
				}
				Eq(resp.StatusCode, 200)
				body, err := readAll(t, resp)
				if err != nil {
					t.Fatal(err)
				}
				Eq(body, malformed(after))
			})
		}
	}
}

// The frame is injected once even if the stream goes on.
func TestHTTPMalformedSSEOnce(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, _ := s.make(t)
			ErrCfg("p", HTTP(MalformedSSE(1)))
			resp, err := get(testCtx(t), c, url+"/sse")
			if err != nil {
				t.Fatal(err)
			}
			body, _ := readAll(t, resp)
			Eq(strings.Count(body, MalformedSSEFrame), 1)
		})
	}
}

// Non-SSE bodies are left alone.
func TestHTTPMalformedSSENotSSE(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, _ := s.make(t)
			ErrCfg("p", HTTP(MalformedSSE(0)))
			resp, err := get(testCtx(t), c, url)
			if err != nil {
				t.Fatal(err)
			}
			body, err := readAll(t, resp)
			if err != nil {
				t.Fatal(err)
			}
			Eq(body, plainBody)
		})
	}
}

// Writes that split events at arbitrary places, with CRLF, comments and one
// byte at a time, still get the frame at an event boundary.
func TestSSEInjectorBoundaries(t *testing.T) {
	stream := ": keepalive\n\ndata: a\r\n\r\nevent: x\ndata: b\n\ndata: c\n\n"
	want := ": keepalive\n\ndata: a\r\n\r\nevent: x\ndata: b\n\n" + MalformedSSEFrame + "data: c\n\n"
	for chunk := 1; chunk <= len(stream); chunk++ {
		inj := sseInjector{after: 2}
		var got strings.Builder
		for i := 0; i < len(stream); i += chunk {
			j := min(i+chunk, len(stream))
			got.Write(inj.process([]byte(stream[i:j])))
		}
		if got.String() != want {
			t.Fatalf("chunk %d: got %q", chunk, got.String())
		}
	}
}

// The transport body hands back injected bytes even through a tiny buffer.
func TestSSEReaderSmallReads(t *testing.T) {
	r := &sseReader{rc: io.NopCloser(strings.NewReader(strings.Join(sseEvents, ""))), inj: sseInjector{after: 0}}
	var got strings.Builder
	buf := make([]byte, 3)
	for {
		n, err := r.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			Eq(err, io.EOF)
			break
		}
	}
	Eq(got.String(), malformed(0))
}

func TestHTTPProbZeroNeverFaults(t *testing.T) {
	faults := []HTTPFault{
		FailBeforeHeaders(), StatusFault(500), StallHeaders(time.Hour),
		TruncateBody(1), ResetConn(), MalformedSSE(0),
	}
	for _, s := range bothSides {
		for _, f := range faults {
			t.Run(s.name+"/"+f.String(), func(t *testing.T) {
				c, url, up := s.make(t)
				ErrCfg("p", Prob(0.0), HTTP(f))
				for range 20 {
					resp, err := get(testCtx(t), c, url+"/sse")
					if err != nil {
						t.Fatal(err)
					}
					body, err := readAll(t, resp)
					if err != nil {
						t.Fatal(err)
					}
					Eq(body, strings.Join(sseEvents, ""))
				}
				Eq(up.hits.Load(), 20)
				Eq(ErrStats("p"), FaultStats{Checks: 20, Fired: 0})
			})
		}
	}
}

func TestHTTPStatsCountFires(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, up := s.make(t)
			ErrCfg("p", Prob(0.5), WithSeed(42), HTTP(StatusFault(502)))
			var faulted int64
			const n = 200
			for range n {
				resp, err := get(testCtx(t), c, url)
				if err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode == 502 {
					faulted++
				}
				_, _ = readAll(t, resp)
			}
			st := ErrStats("p")
			Eq(st.Checks, int64(n))
			Eq(st.Fired, faulted)
			Eq(up.hits.Load(), int64(n)-faulted)
			GT(faulted, 50)
			LT(faulted, 150)
		})
	}
}

// The same seed replays the same sequence of faulted requests.
func TestHTTPSeedReplays(t *testing.T) {
	run := func() []bool {
		c, url, _ := transportSide(t)
		ErrCfg("p", Prob(0.5), WithSeed(7), HTTP(StatusFault(500)))
		var out []bool
		for range 30 {
			resp, err := get(testCtx(t), c, url)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, resp.StatusCode == 500)
			_, _ = readAll(t, resp)
		}
		return out
	}
	a, b := run(), run()
	Eq(fmt.Sprint(a), fmt.Sprint(b))
}

func TestHTTPDelayOnlyPointPassesThrough(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, up := s.make(t)
			const d = 100 * time.Millisecond
			ErrCfg("p", Delay(d))
			start := time.Now()
			resp, err := get(testCtx(t), c, url)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := readAll(t, resp)
			Eq(body, plainBody)
			GTE(time.Since(start), d)
			Eq(up.hits.Load(), 1)
		})
	}
}

// A point with no HTTP fault acts as FailBeforeHeaders.
func TestHTTPPlainPointFailsBeforeHeaders(t *testing.T) {
	c, url, up := transportSide(t)
	ErrCfg("p")
	_, err := get(testCtx(t), c, url)
	if !errors.Is(err, ErrFault) {
		t.Fatalf("want ErrFault, got %v", err)
	}
	Eq(up.hits.Load(), 0)

	c, url, up = handlerSide(t)
	ErrCfg("p")
	_, err = get(testCtx(t), c, url)
	if err == nil {
		t.Fatal("request succeeded")
	}
	Eq(up.hits.Load(), 0)
}

// ErrPoint on a point with an HTTP fault returns the plain error, and the
// HTTP fault is ignored.
func TestErrPointIgnoresHTTPFault(t *testing.T) {
	WithFaults(t)
	ErrCfg("p", HTTP(StatusFault(503)))
	if err := ErrPoint("p"); !errors.Is(err, ErrFault) {
		t.Fatalf("want ErrFault, got %v", err)
	}
	ErrCfg("q", Prob(0.0), HTTP(StatusFault(503)))
	if err := ErrPoint("q"); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPUnconfiguredPassesThrough(t *testing.T) {
	for _, s := range bothSides {
		t.Run(s.name, func(t *testing.T) {
			c, url, up := s.make(t)
			resp, err := get(testCtx(t), c, url)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := readAll(t, resp)
			Eq(body, plainBody)
			Eq(up.hits.Load(), 1)
		})
	}
}

// The point is looked up per request, so it can change under a live server.
func TestHTTPReconfigureLive(t *testing.T) {
	c, url, _ := handlerSide(t)
	for range 3 {
		ErrCfg("p", HTTP(StatusFault(500)))
		resp, err := get(testCtx(t), c, url)
		if err != nil {
			t.Fatal(err)
		}
		Eq(resp.StatusCode, 500)
		_, _ = readAll(t, resp)
		ErrRemove("p")
		resp, err = get(testCtx(t), c, url)
		if err != nil {
			t.Fatal(err)
		}
		Eq(resp.StatusCode, 200)
		_, _ = readAll(t, resp)
	}
}

// Streaming through the handler: events arrive one by one, before the
// handler returns, proving Flush passes through the wrapper.
func TestHTTPHandlerStreamsWithFlush(t *testing.T) {
	WithFaults(t)
	release := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sseEvents[0])
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, sseEvents[1])
	})
	srv := httptest.NewServer(FaultHandler("p", h))
	t.Cleanup(srv.Close)
	// The faulted path (a wrapped writer) and the untouched path both stream.
	for _, prob := range []float64{0, 1} {
		ErrCfg("p", Prob(prob), HTTP(MalformedSSE(1)))
		c := newClient(t, nil)
		resp, err := get(testCtx(t), c, srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, len(sseEvents[0]))
		if _, err := io.ReadFull(resp.Body, buf); err != nil {
			t.Fatal(err)
		}
		Eq(string(buf), sseEvents[0])
		release <- struct{}{}
		rest, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if prob == 1 {
			Eq(string(rest), MalformedSSEFrame+sseEvents[1])
		} else {
			Eq(string(rest), sseEvents[1])
		}
	}
}

// A truncating handler cuts a stream that is flushed event by event.
func TestHTTPHandlerTruncateStreaming(t *testing.T) {
	c, url, _ := handlerSide(t)
	ErrCfg("p", HTTP(TruncateBody(int64(len(sseEvents[0])+len(sseEvents[1])))))
	resp, err := get(testCtx(t), c, url+"/sse")
	if err != nil {
		t.Fatal(err)
	}
	body, err := readAll(t, resp)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("want io.ErrUnexpectedEOF, got %v", err)
	}
	Eq(body, sseEvents[0]+sseEvents[1])
}

func TestFaultTransportNilBase(t *testing.T) {
	WithFaults(t)
	rt := FaultTransport("p", nil).(*faultTransport)
	if rt.base != http.DefaultTransport {
		t.Fatal("nil base did not default to http.DefaultTransport")
	}
	rt.CloseIdleConnections()
}

func TestHTTPFaultString(t *testing.T) {
	Eq(HTTPFault{}.String(), "none")
	Eq(StatusFault(503).String(), "StatusFault(503)")
	Eq(TruncateBody(5).String(), "TruncateBody(5)")
}
