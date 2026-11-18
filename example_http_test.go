package check_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/eliothedeman/check"
)

// FaultTransport wraps a client's transport. Here every request to the
// "backend" point is answered with a synthetic 503, and the upstream is never
// called.
func ExampleFaultTransport() {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "hello")
	}))
	defer upstream.Close()

	check.ErrCfg("backend", check.HTTP(check.StatusFault(503)))
	defer check.ErrRemove("backend")

	client := &http.Client{Transport: check.FaultTransport("backend", http.DefaultTransport)}
	resp, err := client.Get(upstream.URL)
	if err != nil {
		panic(err)
	}
	defer func() { _ = resp.Body.Close() }()
	fmt.Println(resp.StatusCode)

	// Cut the body after 3 bytes instead.
	check.ErrCfg("backend", check.HTTP(check.TruncateBody(3)))
	resp2, err := client.Get(upstream.URL)
	if err != nil {
		panic(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	body, err := io.ReadAll(resp2.Body)
	fmt.Printf("%q %v\n", body, errors.Is(err, io.ErrUnexpectedEOF))
	fmt.Println(check.ErrStats("backend").Fired)

	// Output:
	// 503
	// "hel" true
	// 1
}

// FaultHandler injects the same faults on the server side.
func ExampleFaultHandler() {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "hello")
	})
	check.ErrCfg("server", check.HTTP(check.StatusFault(500)))
	defer check.ErrRemove("server")

	srv := httptest.NewServer(check.FaultHandler("server", next))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		panic(err)
	}
	defer func() { _ = resp.Body.Close() }()
	fmt.Println(resp.StatusCode)

	// Output:
	// 500
}
