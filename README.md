# check [![Go](https://github.com/eliothedeman/check/actions/workflows/go.yml/badge.svg)](https://github.com/eliothedeman/check/actions/workflows/go.yml)

A panic-based assertion library for Go, with fault injection for tests.

- **Typed assertions** built on generics.
- **Fail-fast**: a failed assertion panics. `check.Recover` and `check.Run` turn
  that panic into a normal test failure.
- **Fault injection**: make a function, an HTTP client or an HTTP server fail,
  stall or misbehave, with seeds so you can replay a run.
- **Zero dependencies**: it uses only the Go standard library.

> **v0.x:** the API may change between minor versions; every change is listed in
> [CHANGELOG.md](CHANGELOG.md).

## Install

```bash
go get github.com/eliothedeman/check
```

## Quick start

```go
func TestMyFunction(t *testing.T) {
	defer check.Recover(t) // turn a failed assertion into a test failure

	result := MyFunction()
	check.Eq(result.Status, "success")
	check.GT(result.Count, 0)
	check.NotNil(result.Data)
}
```

Every assertion takes optional message strings as its last arguments. They
become a prefix: `check.Eq(1, 2, "retries")` panics with `retries: 1 == 2`.

## Assertions

```go
check.Eq(a, b)                     // a == b          (comparable types)
check.NotEq(a, b)                  // a != b
check.GT(a, b)                     // a >  b          (ordered types)
check.GTE(a, b)                    // a >= b
check.LT(a, b)                     // a <  b
check.LTE(a, b)                    // a <= b
check.Between(a, low, high)        // low <  a <  high
check.BetweenInclusive(a, low, high) // low <= a <= high

check.Nil(x)                       // x is nil, including a typed nil
check.NotNil(x)
check.Is[T](x)                     // x holds a value of type T
check.ErrIs(err, target)           // errors.Is(err, target)
check.NoErr(err)                   // err is nil
check.ErrContains(err, "text")     // err is not nil and its message has "text"
check.Must(v, err)                 // returns v; panics if err is not nil

check.InDelta(a, b, delta)         // |a - b| <= delta, any int or float type
check.Contains(s, "text")          // strings
check.NotContains(s, "text")

check.SliceEq(a, b)
check.SliceContains(s, v)
check.SliceSorted(s)

check.Panics(func() { ... })       // the function must panic
```

`Nil` and `NotNil` see through typed nils: a nil `*T`, map, slice, channel, func
or interface counts as nil.

`InDelta` works for `time.Duration` and for floats. NaN is never within a delta.

```go
check.InDelta(100*time.Millisecond, 130*time.Millisecond, 50*time.Millisecond)
check.InDelta(1.0, 1.05, 0.1)
check.Contains("data: hello", "hello")
```

## Using it with `testing.TB`

An assertion that panics in a goroutine kills the whole test binary. Two
helpers turn the panic into a failure of one test. They accept anything with
`Helper()`, `Errorf` and `FailNow()`, which includes `*testing.T`, `*testing.B`
and `*testing.F`.

```go
// Recover: defer it at the top of a test. A panic becomes t.Errorf + t.FailNow.
func TestX(t *testing.T) {
	defer check.Recover(t)
	check.Eq(1, 2, "count")
}

// Run: calls f and returns false if it panicked. It calls t.Errorf only,
// never t.FailNow, so it is safe in any goroutine.
func TestY(t *testing.T) {
	done := make(chan bool)
	go func() {
		done <- check.Run(t, func() { check.GT(1, 2) })
	}()
	if !<-done {
		t.Log("the assertion failed in the goroutine")
	}
}
```

Both report the panic value and a final line `at file:line`. Read that line for
the place of the failure: it is the first frame outside package `check`. The
file:line that `testing` prints before the message points into this library,
because that is where `t.Errorf` is called. If a helper function of yours calls
the assertion, the helper's line is reported, not the line in the test that
called the helper.

`Recover` must be deferred directly (`defer check.Recover(t)`) and must run on
the test goroutine. In any other goroutine use `Run`.

## Eventually and Never

```go
// Eventually: cond must become true within the timeout.
check.Eventually(time.Second, time.Millisecond, ready.Load, "server ready")

// Never: cond must stay false for the whole duration.
check.Never(30*time.Millisecond, time.Millisecond, func() bool {
	return retries.Load() > 0
}, "no retry")
```

The arguments are `(timeout, interval, cond, msg...)`. Both poll `cond` on the
calling goroutine, check once at the start and once at the deadline, and start
no goroutines. The duration and the interval must be positive.

## Catch

`Catch` runs a function and turns a panic into an error.

```go
_, err := check.Catch(func() int {
	check.GT(0, 1)
	return 1
})
// err's message starts with "0 > 1", then the stack of the panic.

// With catchable errors, only a matching error panic is caught.
_, err = check.Catch(func() int { panic(io.EOF) }, io.EOF)
// errors.Is(err, io.EOF) is true
```

- With **no** catchable errors, every panic is caught, whether its value is an
  error or not. The error carries the stack. If the panic value is an error, the
  returned error wraps it with `%w`.
- With catchable errors, only error panics that match one of them with
  `errors.Is` are caught, and that error is returned as is. Every other panic is
  re-panicked with its original value.

## Fault injection

A fault point is a named place in your code. Configure it in a test with
`ErrCfg`, and call `ErrPoint` where the failure should happen.

```go
func QueryDatabase() error {
	if err := check.ErrPoint("db.query"); err != nil {
		return err
	}
	// real work
	return nil
}

func TestQueryFails(t *testing.T) {
	check.WithFaults(t)               // restore the faults when the test ends
	check.ErrCfg("db.query")          // no options: always fires
	err := QueryDatabase()
	check.ErrIs(err, check.ErrFault)  // "fault: db.query"
}
```

A point that was never configured returns nil, so production code that carries
`ErrPoint` calls is unaffected.

| Call | Meaning |
| --- | --- |
| `ErrCfg(name, opts...)` | Configure the point. It replaces any earlier config and resets its stats. |
| `ErrPoint(name) error` | Returns nil, or an error that wraps `ErrFault`, when the fault fires. |
| `ErrPointCtx(ctx, name) error` | Like `ErrPoint`. A `Delay` ends early when `ctx` is done, and the call returns `ctx.Err()`. |
| `Prob(p)` | Fire with probability `p`, from 0 to 1. `Prob(0)` never fires, `Prob(1)` always fires. NaN or a value out of range panics. |
| `Delay(d)` | Sleep `d` when the fault fires. Then `ErrPoint` returns nil, so the fault only adds latency, unless `WithError` is also given. |
| `WithError(err)` | Return exactly `err` (not wrapped) when the fault fires. |
| `Seed(n)` | Set the global seed. Points configured afterwards, without `WithSeed`, derive their source from it and from their name. |
| `WithSeed(n)` | Seed one point. It overrides the global seed. |
| `FaultSeed()` | The current global seed. Log it when a test fails. |
| `ErrStats(name)` | A `FaultStats{Checks, Fired}` snapshot. The zero value if the point is unknown. |
| `ErrResetStats(name)` | Zero the stats and keep the config. |
| `ErrRemove(name)` | Remove one point. |
| `ErrReset()` | Remove every point. |
| `WithFaults(t)` | Save the points, remove them all, and restore them with `t.Cleanup`. |

```go
check.ErrCfg("never", check.Prob(0.0))
check.ErrCfg("always", check.Prob(1.0))

check.ErrCfg("slow", check.Delay(10*time.Millisecond))      // latency only
check.ErrCfg("net", check.WithError(io.ErrUnexpectedEOF))   // a specific error
check.ErrCfg("flaky", check.Prob(0.5), check.WithSeed(42))  // replayable

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
defer cancel()
err := check.ErrPointCtx(ctx, "stuck") // returns the context error if it ends first
```

**Reproduce a flaky run.** The same seed replays the same fired and not-fired
sequence for each name, in any order of configuration:

```go
check.Seed(42)
check.ErrCfg("flaky", check.Prob(0.5))
// ... the same sequence on every run with Seed(42)
```

By default the seed comes from the clock. Log `check.FaultSeed()` in a failing
test, then call `check.Seed` with that value to replay it. `Seed` only affects
points configured after it.

**Assert that the fault fired.** A fault test can pass without the fault ever
running. Check:

```go
check.ErrCfg("db.query")
_ = check.ErrPoint("db.query")
check.Eq(check.ErrStats("db.query"), check.FaultStats{Checks: 1, Fired: 1})
```

**Not for parallel tests.** The set of fault points is one global. `WithFaults`,
`ErrReset` and `Seed` change it for the whole process, so do not use them from
tests that call `t.Parallel`: those tests would see and overwrite each other's
faults. Give each parallel test its own point names, and do not call
`WithFaults` or `ErrReset` from it.

## HTTP fault injection

`FaultTransport` wraps a client's `http.RoundTripper`. `FaultHandler` wraps a
server's `http.Handler`. Both read a fault point once per request, so `Prob`,
`Seed`, `Delay`, `ErrStats` and `WithFaults` work as they do for `ErrPoint`.
Attach the fault with `check.HTTP`.

```go
check.ErrCfg("backend", check.HTTP(check.StatusFault(503)))

// Client side: wrap the transport. A nil base means http.DefaultTransport.
client := &http.Client{Transport: check.FaultTransport("backend", http.DefaultTransport)}

// Server side: wrap the handler.
srv := httptest.NewServer(check.FaultHandler("backend", next))
```

The point is looked up on every request, so you can configure, change or remove
it after the transport or handler exists.

| Fault | Effect |
| --- | --- |
| `FailBeforeHeaders()` | No response header is sent. Transport: `RoundTrip` returns the point's error and the base transport is not called. Handler: the connection is closed with no bytes written, so the client sees an EOF. |
| `StatusFault(code)` | A synthetic response with the status `code` (200 to 599, meant for 5xx) and a short text body. The upstream is not called. |
| `StallHeaders(d)` | Wait `d` before the request goes on, so no header arrives. It ends early when the request context is done: the transport returns the context's error, and the handler returns. |
| `TruncateBody(n)` | Headers pass. The body is cut after `n` bytes, and the client's read returns `io.ErrUnexpectedEOF`. A body of `n` bytes or fewer is not cut. |
| `ResetConn()` | A TCP reset before any header. Transport: an error that wraps `syscall.ECONNRESET`. Handler: the connection closes with `SO_LINGER 0`, which sends a RST. |
| `MalformedSSE(n)` | In a `text/event-stream` response, inject `MalformedSSEFrame` once after `n` complete events (0 means before the first). The rest of the stream passes through. Other content types pass through unchanged. |

```go
// Cut the body after 5 bytes.
check.ErrCfg("backend", check.HTTP(check.TruncateBody(5)))
resp, _ := client.Get(url)
body, err := io.ReadAll(resp.Body)
// body is "hello", errors.Is(err, io.ErrUnexpectedEOF) is true

// Stall until the request context ends.
check.ErrCfg("backend", check.HTTP(check.StallHeaders(time.Hour)))
ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
defer cancel()
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
_, err = client.Do(req)
// errors.Is(err, context.DeadlineExceeded) is true
```

Notes:

- A point with no HTTP fault, reached through `FaultTransport` or `FaultHandler`,
  behaves as `FailBeforeHeaders`, unless it has `Delay` and no `WithError`.
  Then it only adds latency. `ErrPoint` ignores HTTP faults.
- Faults that close the connection need a connection that can be hijacked
  (HTTP/1.x). On HTTP/2 the handler aborts the stream with
  `panic(http.ErrAbortHandler)`.
- **Keep-alive retry caveat.** A client's `net/http.Transport` silently retries
  an idempotent request (such as GET) when it reused a keep-alive connection and
  that connection fails before any response byte arrives. A `FailBeforeHeaders`
  or `ResetConn` fault from `FaultHandler` is such a failure. With `Prob` below 1
  the retry can skip the fault, and the point counts two checks for one client
  call. To test it deterministically, send a non-idempotent request such as
  POST, or set `DisableKeepAlives` on the client's transport.
- `TruncateBody` on a transport reads one byte past the cut, to tell a body that
  ends at the cut from a longer one. On an idle stream the error appears when
  the next upstream bytes arrive, not at the cut itself.

## Examples that run

The snippets above are backed by tested examples: see `example_assert_test.go`,
`example_tb_test.go`, `example_fault_test.go`, `example_httpfaults_test.go` and
`example_http_test.go`. They run with `go test`.

## License

MIT

## Contributing

Issues and pull requests are welcome.
