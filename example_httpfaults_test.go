package check_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"syscall"
	"time"

	"github.com/eliothedeman/check"
)

// get sends a GET through a client whose transport injects the faults of the
// point "backend".
func get(ctx context.Context, url string) (*http.Response, error) {
	client := &http.Client{Transport: check.FaultTransport("backend", http.DefaultTransport)}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func ExampleFailBeforeHeaders() {
	defer check.ErrReset()
	check.ErrCfg("backend", check.HTTP(check.FailBeforeHeaders()))

	_, err := get(context.Background(), "http://unused.invalid/")
	fmt.Println(errors.Is(err, check.ErrFault))

	// Output:
	// true
}

func ExampleStatusFault() {
	defer check.ErrReset()
	check.ErrCfg("backend", check.HTTP(check.StatusFault(503)))

	resp, err := get(context.Background(), "http://unused.invalid/")
	if err != nil {
		panic(err)
	}
	defer func() { _ = resp.Body.Close() }()
	fmt.Println(resp.StatusCode)

	// Output:
	// 503
}

// StallHeaders holds the request back until the request context ends.
func ExampleStallHeaders() {
	defer check.ErrReset()
	check.ErrCfg("backend", check.HTTP(check.StallHeaders(time.Hour)))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := get(ctx, "http://unused.invalid/")
	fmt.Println(errors.Is(err, context.DeadlineExceeded))

	// Output:
	// true
}

func ExampleTruncateBody() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "hello world")
	}))
	defer srv.Close()
	defer check.ErrReset()
	check.ErrCfg("backend", check.HTTP(check.TruncateBody(5)))

	resp, err := get(context.Background(), srv.URL)
	if err != nil {
		panic(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	fmt.Printf("%q %v\n", body, errors.Is(err, io.ErrUnexpectedEOF))

	// Output:
	// "hello" true
}

func ExampleResetConn() {
	defer check.ErrReset()
	check.ErrCfg("backend", check.HTTP(check.ResetConn()))

	_, err := get(context.Background(), "http://unused.invalid/")
	fmt.Println(errors.Is(err, syscall.ECONNRESET))

	// Output:
	// true
}

// MalformedSSE injects one bad frame after the given number of events.
func ExampleMalformedSSE() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: one\n\ndata: two\n\n")
	}))
	defer srv.Close()
	defer check.ErrReset()
	check.ErrCfg("backend", check.HTTP(check.MalformedSSE(1)))

	resp, err := get(context.Background(), srv.URL)
	if err != nil {
		panic(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%q\n", body)

	// Output:
	// "data: one\n\ndata: {\"malformed\": \n\ndata: two\n\n"
}
