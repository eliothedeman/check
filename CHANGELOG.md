# Changelog

## v0.5.0 — 2026-11-18

### Added

- HTTP fault injection, driven by the same fault points as `ErrPoint`
  (`Prob`, `Seed`/`WithSeed`, `Delay`, `ErrStats` and `WithFaults` all apply):
  - `FaultTransport(name, base)` wraps an `http.RoundTripper`.
  - `FaultHandler(name, next)` wraps an `http.Handler`; it works against a real
    `net/http` server, including streaming with `Flush`.
  - `HTTP(fault)` attaches an `HTTPFault` to a point configured with `ErrCfg`.
  - Faults: `FailBeforeHeaders()`, `StatusFault(code)`, `StallHeaders(d)`
    (ends with the request context), `TruncateBody(n)` (`io.ErrUnexpectedEOF`),
    `ResetConn()` (`syscall.ECONNRESET` / TCP RST) and `MalformedSSE(n)`
    (injects `MalformedSSEFrame` into a `text/event-stream` body).
  - A point without an HTTP fault behaves as `FailBeforeHeaders` when reached
    through `FaultTransport` or `FaultHandler`. `ErrPoint` ignores HTTP faults.

## v0.4.0 — 2026-11-18

### Behaviour changes

These change what existing code does. Check them before upgrading.

- **`Catch` semantics (fixes `Catch` ignoring its documented behaviour).**
  - With no catchable errors, `Catch` now catches *every* panic. Before, an
    error-typed panic was re-thrown because of an always-true
    `len(catchable) >= 0` check.
  - The error it returns now carries the panic stack for every panic, including
    string panics such as those from `check.GT`. Before, string panics lost the
    stack. The original error is still wrapped with `%w`.
  - With catchable errors, only error panics matching via `errors.Is` are caught.
    Every other panic, including non-error panics, is now re-panicked with its
    original value. Before, a non-error panic was converted to an error even
    when a catchable list was given.
- **`Prob(0)` never fires.** Before, it could still fire when the source drew 0.
  `Prob(1)` always fires. `Prob(NaN)` now panics like an out-of-range value.
- **Default seed.** The global fault seed is now `time.Now().UnixNano()` at
  package init (was `time.Now().Unix()`), and each fault point derives its own
  source from that seed and a hash of its name. Sequences for a given seed
  differ from earlier releases.
- **Fault points no longer share a random source.** Each config owns its source.
  Re-configuring a point with `ErrCfg` creates a fresh source and zeroes its stats.

### Added

- `Seed(int64)`, `WithSeed(int64)` and `FaultSeed()` for reproducible faults.
- `ErrReset()`, `ErrRemove(name)` and `WithFaults(t)` (with the `Cleanuper`
  interface) to isolate faults between tests.
- `FaultStats`, `ErrStats(name)` and `ErrResetStats(name)` so a test can assert
  that a fault fired.
- `Delay(d)` for latency faults (composes with `Prob`) and
  `ErrPointCtx(ctx, name)`, which ends the delay when `ctx` is done.
- `WithError(err)` to return a specific error from a fault point.
- Doc comments on the fault API.

### Fixed

- Data race on the fault random source: `ErrPoint` drew from one
  `rand.Source` under a read lock. Each config now has its own source behind its
  own mutex.
- Removed the dead, inconsistent `errCfg.err` method.

### CI

- Run `go vet`, `go test -race` and `golangci-lint` in addition to build and
  test. Bump `actions/setup-go` to v5 and read the Go version from `go.mod`.
