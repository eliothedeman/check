package check

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
)

// httpKind identifies an HTTP fault. The zero value means "no HTTP fault".
type httpKind int

const (
	httpNone httpKind = iota
	httpFailBeforeHeaders
	httpStatus
	httpStallHeaders
	httpTruncateBody
	httpResetConn
	httpMalformedSSE
)

// HTTPFault describes how a fault point misbehaves at the HTTP level. Build one
// with FailBeforeHeaders, StatusFault, StallHeaders, TruncateBody, ResetConn or
// MalformedSSE and attach it to a fault point with HTTP. The zero value is "no
// HTTP fault".
//
// An HTTPFault only takes effect through FaultTransport and FaultHandler. Both
// consult the fault point once per request, so Prob, Seed, WithSeed, Delay, ErrStats and
// WithFaults all apply exactly as they do for ErrPoint.
type HTTPFault struct {
	kind httpKind
	// code is the status of a StatusFault.
	code int
	// d is the stall of a StallHeaders fault.
	d time.Duration
	// n is the byte count of a TruncateBody or the event count of a MalformedSSE.
	n int64
}

// String describes the fault, for test logs.
func (f HTTPFault) String() string {
	switch f.kind {
	case httpFailBeforeHeaders:
		return "FailBeforeHeaders"
	case httpStatus:
		return fmt.Sprintf("StatusFault(%d)", f.code)
	case httpStallHeaders:
		return fmt.Sprintf("StallHeaders(%s)", f.d)
	case httpTruncateBody:
		return fmt.Sprintf("TruncateBody(%d)", f.n)
	case httpResetConn:
		return "ResetConn"
	case httpMalformedSSE:
		return fmt.Sprintf("MalformedSSE(%d)", f.n)
	}
	return "none"
}

// FailBeforeHeaders fails the request before any response header is sent.
//
// Transport: RoundTrip does not call the base transport and returns the point's
// error (an error wrapping ErrFault, or the error given to WithError).
//
// Handler: the connection is hijacked and closed without writing a byte, so the
// client sees an EOF. next is not called. If the connection cannot be hijacked
// (HTTP/2), the handler panics with http.ErrAbortHandler, which aborts the stream.
func FailBeforeHeaders() HTTPFault {
	return HTTPFault{kind: httpFailBeforeHeaders}
}

// StatusFault answers with a synthetic response with the given status code and
// a short plain-text body. The upstream (the base transport, or next) is not
// called. code must be between 200 and 599; it is meant for 5xx.
func StatusFault(code int) HTTPFault {
	BetweenInclusive(code, 200, 599)
	return HTTPFault{kind: httpStatus, code: code}
}

// StallHeaders waits d before the request proceeds, so no response header
// arrives in the meantime. The wait ends early when the request context is
// done: the transport then returns the context's error, and the handler returns
// without calling next. After the stall the request goes upstream normally.
func StallHeaders(d time.Duration) HTTPFault {
	return HTTPFault{kind: httpStallHeaders, d: d}
}

// TruncateBody lets the response headers through and cuts the body after
// afterBytes bytes. A body that is afterBytes bytes or shorter is not cut.
//
// Transport: the response body returns io.ErrUnexpectedEOF after afterBytes bytes.
//
// Handler: after afterBytes body bytes are written and flushed, the connection is
// closed without finishing the response, so the client's body read returns
// io.ErrUnexpectedEOF. Later writes by next fail with http.ErrHijacked.
func TruncateBody(afterBytes int64) HTTPFault {
	GTE(afterBytes, 0)
	return HTTPFault{kind: httpTruncateBody, n: afterBytes}
}

// ResetConn simulates a TCP connection reset before any response header.
//
// Transport: RoundTrip returns an error that wraps syscall.ECONNRESET, so
// errors.Is(err, syscall.ECONNRESET) is true. The base transport is not called.
//
// Handler: the connection is hijacked and closed with SO_LINGER 0, which sends a
// TCP RST. The client sees "connection reset by peer" (syscall.ECONNRESET). If the
// connection is not a TCP connection, or cannot be hijacked, this degrades to
// FailBeforeHeaders.
func ResetConn() HTTPFault {
	return HTTPFault{kind: httpResetConn}
}

// MalformedSSEFrame is the garbage frame MalformedSSE injects: a data line
// whose JSON is not valid.
const MalformedSSEFrame = "data: {\"malformed\": \n\n"

// MalformedSSE injects MalformedSSEFrame into a text/event-stream response
// after afterEvents complete events (afterEvents 0 injects it before the first
// event). It is injected once, and the rest of the stream passes through, so a
// client sees valid events, one malformed frame, then valid events again.
//
// Responses whose Content-Type is not text/event-stream, and streams that end
// before afterEvents events, are passed through unchanged. Comment lines
// (starting with ':') do not count as events.
//
// Transport: the response body is rewritten as it is read. Handler: the bytes
// next writes are rewritten.
func MalformedSSE(afterEvents int) HTTPFault {
	GTE(afterEvents, 0)
	return HTTPFault{kind: httpMalformedSSE, n: int64(afterEvents)}
}

// HTTP attaches an HTTP-level fault to a fault point. Use it with ErrCfg:
//
//	check.ErrCfg("backend", check.Prob(0.1), check.HTTP(check.StatusFault(503)))
//
// FaultTransport and FaultHandler apply f when the point fires. A point with an
// HTTP fault that is reached through ErrPoint or ErrPointCtx instead behaves
// like any other point: it fires with the usual error (after any Delay) and the
// HTTP fault is ignored.
//
// A point without an HTTP fault that is reached through FaultTransport or
// FaultHandler behaves as FailBeforeHeaders, unless it was configured with Delay
// and without WithError. Then it only adds latency, and the request goes upstream.
func HTTP(f HTTPFault) ErrOpt {
	return func(e *errCfg) {
		e.httpFault = f
	}
}

// httpPlan consults the fault point name for one request. The returned fault has
// kind httpNone when the request should pass through untouched. A non-nil error
// means ctx ended while a Delay was running.
func httpPlan(ctx context.Context, name string) (HTTPFault, *errCfg, error) {
	cfg, ok := lookup(name)
	if !ok {
		return HTTPFault{}, nil, nil
	}
	fired, err := cfg.fire(ctx)
	if err != nil {
		return HTTPFault{}, cfg, err
	}
	if !fired {
		return HTTPFault{}, cfg, nil
	}
	if cfg.httpFault.kind != httpNone {
		return cfg.httpFault, cfg, nil
	}
	if cfg.delaySet && !cfg.errSet {
		return HTTPFault{}, cfg, nil
	}
	return HTTPFault{kind: httpFailBeforeHeaders}, cfg, nil
}

// sleepCtx waits d or until ctx is done, and returns ctx.Err() in the second case.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isSSE(h http.Header) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(h.Get("Content-Type"))), "text/event-stream")
}

func faultBody(name string, code int) string {
	return fmt.Sprintf("check: injected fault %q: HTTP %d\n", name, code)
}

// sseInjector inserts MalformedSSEFrame into a byte stream of SSE events.
type sseInjector struct {
	after int64
	// events counts complete events seen so far.
	events int64
	done   bool
	// lineLen is the number of non-CR/LF bytes on the current line.
	lineLen int
	comment bool
	// hasLine reports whether the current event has a non-comment line.
	hasLine bool
}

// process returns p with the frame inserted if its position is inside p. The
// result may alias p.
func (s *sseInjector) process(p []byte) []byte {
	if s.done || len(p) == 0 {
		return p
	}
	if s.after == 0 {
		s.done = true
		out := make([]byte, 0, len(MalformedSSEFrame)+len(p))
		out = append(out, MalformedSSEFrame...)
		return append(out, p...)
	}
	for i, b := range p {
		switch b {
		case '\r':
		case '\n':
			if s.lineLen == 0 {
				if s.hasLine {
					s.events++
					s.hasLine = false
					if s.events == s.after {
						s.done = true
						out := make([]byte, 0, len(MalformedSSEFrame)+len(p))
						out = append(out, p[:i+1]...)
						out = append(out, MalformedSSEFrame...)
						return append(out, p[i+1:]...)
					}
				}
			} else if !s.comment {
				s.hasLine = true
			}
			s.lineLen = 0
			s.comment = false
		default:
			if s.lineLen == 0 && b == ':' {
				s.comment = true
			}
			s.lineLen++
		}
	}
	return p
}

// sseReader is a response body that goes through an sseInjector.
type sseReader struct {
	rc   io.ReadCloser
	inj  sseInjector
	pend []byte
	err  error
}

func (r *sseReader) Read(p []byte) (int, error) {
	if len(r.pend) > 0 {
		n := copy(p, r.pend)
		r.pend = r.pend[n:]
		return n, nil
	}
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.rc.Read(p)
	if n > 0 {
		out := r.inj.process(p[:n])
		if len(out) > len(p) {
			m := copy(p, out)
			r.pend = out[m:]
			r.err = err
			return m, nil
		}
		copy(p, out)
		n = len(out)
	}
	return n, err
}

func (r *sseReader) Close() error { return r.rc.Close() }

// truncReader is a response body that fails with io.ErrUnexpectedEOF after n bytes.
type truncReader struct {
	rc        io.ReadCloser
	remaining int64
}

func (r *truncReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		// Only truncate if the real body has more to give.
		var one [1]byte
		if _, err := io.ReadFull(r.rc, one[:]); err != nil {
			return 0, err
		}
		return 0, io.ErrUnexpectedEOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.rc.Read(p)
	r.remaining -= int64(n)
	return n, err
}

func (r *truncReader) Close() error { return r.rc.Close() }

// errConnReset is what a ResetConn fault returns from a transport.
func errConnReset() error {
	return &net.OpError{Op: "read", Net: "tcp", Err: os.NewSyscallError("read", syscall.ECONNRESET)}
}

// faultTransport is the RoundTripper returned by FaultTransport.
type faultTransport struct {
	name string
	base http.RoundTripper
}

// FaultTransport returns a RoundTripper that consults the fault point name once
// per request and, when it fires, injects the point's HTTP fault (see HTTP).
// Requests the point does not fire for, or when no point named name is
// configured, go to base unchanged. A nil base means http.DefaultTransport.
//
// The point is looked up on every request, so it can be configured, changed or
// removed after the transport is built.
func FaultTransport(name string, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &faultTransport{name: name, base: base}
}

// CloseIdleConnections forwards to the base transport, so
// http.Client.CloseIdleConnections keeps working.
func (t *faultTransport) CloseIdleConnections() {
	if c, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		c.CloseIdleConnections()
	}
}

func (t *faultTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	closeBody := func() {
		if req.Body != nil {
			_ = req.Body.Close()
		}
	}
	f, cfg, err := httpPlan(req.Context(), t.name)
	if err != nil {
		closeBody()
		return nil, err
	}
	switch f.kind {
	case httpFailBeforeHeaders:
		closeBody()
		return nil, cfg.errFactory(t.name)
	case httpResetConn:
		closeBody()
		return nil, errConnReset()
	case httpStatus:
		closeBody()
		body := faultBody(t.name, f.code)
		h := make(http.Header)
		h.Set("Content-Type", "text/plain; charset=utf-8")
		return &http.Response{
			Status:        fmt.Sprintf("%d %s", f.code, http.StatusText(f.code)),
			StatusCode:    f.code,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        h,
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       req,
		}, nil
	case httpStallHeaders:
		if err := sleepCtx(req.Context(), f.d); err != nil {
			closeBody()
			return nil, err
		}
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	switch f.kind {
	case httpTruncateBody:
		resp.Body = &truncReader{rc: resp.Body, remaining: f.n}
		resp.ContentLength = -1
	case httpMalformedSSE:
		if isSSE(resp.Header) {
			resp.Body = &sseReader{rc: resp.Body, inj: sseInjector{after: f.n}}
			resp.ContentLength = -1
		}
	}
	return resp, nil
}

// FaultHandler returns an http.Handler that consults the fault point name once
// per request and, when it fires, injects the point's HTTP fault (see HTTP) on
// the server side. Requests the point does not fire for, or when no point named
// name is configured, go to next unchanged.
//
// When a fault fires, next sees a wrapped ResponseWriter. It implements
// http.Flusher, and Unwrap for http.ResponseController, but not http.Hijacker.
// Faults that close the connection need a connection that can be hijacked
// (HTTP/1.x). On HTTP/2 they abort the stream with panic(http.ErrAbortHandler).
func FaultHandler(name string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, _, err := httpPlan(r.Context(), name)
		if err != nil {
			return
		}
		switch f.kind {
		case httpFailBeforeHeaders:
			abortConn(w, false)
			return
		case httpResetConn:
			abortConn(w, true)
			return
		case httpStatus:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(f.code)
			_, _ = io.WriteString(w, faultBody(name, f.code))
			return
		case httpStallHeaders:
			if err := sleepCtx(r.Context(), f.d); err != nil {
				return
			}
			next.ServeHTTP(w, r)
		case httpTruncateBody, httpMalformedSSE:
			fw := &faultWriter{ResponseWriter: w, f: f}
			fw.inj.after = f.n
			next.ServeHTTP(fw, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// abortConn closes the connection under w without finishing the response. With
// rst it asks for a TCP RST instead of a FIN. If the connection cannot be
// hijacked it panics with http.ErrAbortHandler, which makes the server drop it.
func abortConn(w http.ResponseWriter, rst bool) {
	conn, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		panic(http.ErrAbortHandler)
	}
	if rst {
		c := conn
		for {
			if tc, ok := c.(*net.TCPConn); ok {
				_ = tc.SetLinger(0)
				break
			}
			u, ok := c.(interface{ NetConn() net.Conn })
			if !ok {
				break
			}
			c = u.NetConn()
		}
	}
	_ = conn.Close()
}

// faultWriter wraps a ResponseWriter for TruncateBody and MalformedSSE.
type faultWriter struct {
	http.ResponseWriter
	f       HTTPFault
	inj     sseInjector
	sniffed bool
	sse     bool
	written int64
	dead    bool
}

// Unwrap lets http.ResponseController reach the real writer.
func (w *faultWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *faultWriter) sniff() {
	if !w.sniffed {
		w.sniffed = true
		w.sse = isSSE(w.Header())
	}
}

func (w *faultWriter) WriteHeader(code int) {
	if w.dead {
		return
	}
	w.sniff()
	w.ResponseWriter.WriteHeader(code)
}

func (w *faultWriter) Flush() {
	if w.dead {
		return
	}
	w.sniff()
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *faultWriter) Write(p []byte) (int, error) {
	if w.dead {
		return 0, http.ErrHijacked
	}
	w.sniff()
	switch w.f.kind {
	case httpMalformedSSE:
		if !w.sse {
			return w.ResponseWriter.Write(p)
		}
		out := w.inj.process(p)
		if _, err := w.ResponseWriter.Write(out); err != nil {
			return 0, err
		}
		return len(p), nil
	case httpTruncateBody:
		if w.written+int64(len(p)) <= w.f.n {
			n, err := w.ResponseWriter.Write(p)
			w.written += int64(n)
			return n, err
		}
		keep := w.f.n - w.written
		if keep > 0 {
			_, _ = w.ResponseWriter.Write(bytes.Clone(p[:keep]))
		}
		w.written = w.f.n
		_ = http.NewResponseController(w.ResponseWriter).Flush()
		w.dead = true
		abortConn(w.ResponseWriter, false)
		return int(keep), http.ErrHijacked
	}
	return w.ResponseWriter.Write(p)
}
