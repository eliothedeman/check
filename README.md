# check

A lightweight, panic-based assertion library for Go with built-in fault injection.

## Features

- **Type-safe assertions** using Go generics
- **Panic-based testing** for fail-fast behavior
- **Zero dependencies** - uses only the Go standard library
- **Fault injection** for chaos engineering and reliability testing
- **Stack trace capture** when converting panics to errors

## Installation

```bash
go get github.com/eliothedeman/check
```

## Quick Start

```go
package mypackage_test

import (
    "testing"
    "github.com/eliothedeman/check"
)

func TestMyFunction(t *testing.T) {
    result := MyFunction()

    // Assertions panic if they fail
    check.Eq(result.Status, "success")
    check.GT(result.Count, 0)
    check.NotNil(result.Data)
}
```

## API Reference

### Equality and Comparison

```go
// Equality
check.Eq(a, b)       // Panics if a != b
check.NotEq(a, b)    // Panics if a == b

// Ordered comparisons
check.GT(a, b)       // Panics if a <= b (greater than)
check.LT(a, b)       // Panics if a >= b (less than)
check.GTE(a, b)      // Panics if a < b (greater than or equal)
check.LTE(a, b)      // Panics if a > b (less than or equal)

// Range checks
check.Between(a, low, high)           // Panics if a is not strictly between low and high
check.BetweenInclusive(a, low, high)  // Panics if a < low or a > high
```

### Type and Nil Checks

```go
check.Nil(x)        // Panics if x is not nil
check.NotNil(x)     // Panics if x is nil
check.Is[T](a)      // Panics if a is not of type T
```

### Error Handling

```go
// Error matching (uses errors.Is)
check.ErrIs(err, target)

// Verify a function panics
check.Panics(func() {
    // Code that should panic
})

// Convert panics to errors with stack traces
result, err := check.Catch(func() string {
    check.GT(0, 1)  // This will panic
    return "success"
})
// err will contain the panic with full stack trace
```

### Fault Injection

Inject controlled failures for chaos engineering and reliability testing:

```go
// Configure a fault point with 50% failure rate
check.ErrCfg("database_query", check.Prob[float64](0.5))

// In your code, add fault injection points
func QueryDatabase() error {
    if err := check.ErrPoint("database_query"); err != nil {
        return err
    }
    // Normal database logic
    return nil
}
```

## Examples

### Basic Assertions

```go
func TestUserValidation(t *testing.T) {
    user := CreateUser("alice", 25)

    check.NotNil(user)
    check.Eq(user.Name, "alice")
    check.GT(user.Age, 0)
    check.Between(user.Age, 18, 120)
}
```

### Error Checking

```go
func TestFileOperations(t *testing.T) {
    err := OpenNonexistentFile()
    check.ErrIs(err, os.ErrNotExist)

    // Works with wrapped errors
    wrapped := fmt.Errorf("failed to open: %w", os.ErrNotExist)
    check.ErrIs(wrapped, os.ErrNotExist)
}
```

### Panic Recovery with Stack Traces

```go
func TestDangerousOperation(t *testing.T) {
    result, err := check.Catch(func() int {
        check.GT(0, 1)  // This panics
        return 42
    })

    if err != nil {
        // err contains the panic message and full stack trace
        t.Logf("Operation failed: %v", err)
    }
}
```

### Fault Injection for Reliability Testing (API not stable)

```go
func TestRetryLogic(t *testing.T) {
    // Configure fault point to fail 80% of the time
    check.ErrCfg("api_call", check.Prob[float64](0.8))

    // Test that retry logic handles failures
    var attempts int
    for attempts < 5 {
        if err := MakeAPICall(); err == nil {
            break
        }
        attempts++
    }

    check.GT(attempts, 1)  // Verify retries occurred
}

func MakeAPICall() error {
    if err := check.ErrPoint("api_call"); err != nil {
        return err
    }
    // Normal API logic
    return nil
}
```

## Design Philosophy

**Panic-based assertions**: Unlike traditional assertion libraries that return booleans, `check` uses panics to signal failures. This provides a fail-fast approach that integrates naturally with Go's testing framework through `recover()`.

**Type safety**: Leverages Go generics to provide compile-time type safety for comparisons and assertions.

**Minimal dependencies**: Uses only the Go standard library, keeping the dependency tree clean and reducing maintenance burden.

**Chaos engineering**: Built-in fault injection allows you to test your code's resilience to failures without complex mocking frameworks.

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
