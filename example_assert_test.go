package check_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/eliothedeman/check"
)

// Assertions panic when they fail and return nothing when they pass.
func Example() {
	check.Eq("ok", "ok")
	check.GT(3, 2)
	check.Between(5, 1, 10)
	check.InDelta(0.30000000000000004, 0.3, 1e-9)
	check.Contains("data: hello", "hello")

	var err error
	check.NoErr(err)

	// A failed assertion panics with a message that says what was compared.
	check.Panics(func() { check.Eq(1, 2, "retries") })
	fmt.Println("done")

	// Output:
	// done
}

// InDelta compares any integer or float type, including time.Duration.
func ExampleInDelta() {
	check.InDelta(100*time.Millisecond, 130*time.Millisecond, 50*time.Millisecond)
	check.InDelta(1.0, 1.05, 0.1)

	check.Panics(func() { check.InDelta(1.0, 1.2, 0.1) })
	fmt.Println("ok")

	// Output:
	// ok
}

// Nil and NotNil see through typed nils.
func ExampleNil() {
	var p *int
	var err error = (*typedNil)(nil) // a non-nil interface that holds a nil pointer

	check.Nil(p)
	check.Nil(err)
	check.Panics(func() { check.NotNil(p) })
	fmt.Println("typed nils are nil")

	// Output:
	// typed nils are nil
}

type typedNil struct{}

func (*typedNil) Error() string { return "typedNil" }

func ExampleErrContains() {
	err := fmt.Errorf("load config: %w", io.ErrUnexpectedEOF)
	check.ErrContains(err, "load config")
	check.ErrIs(err, io.ErrUnexpectedEOF)
	check.Panics(func() { check.ErrContains(nil, "anything") })
	fmt.Println("ok")

	// Output:
	// ok
}

func ExamplePanics() {
	check.Panics(func() { panic("boom") })
	fmt.Println("it panicked")

	// Output:
	// it panicked
}

// Catch turns a panic into an error that carries the stack.
func ExampleCatch() {
	_, err := check.Catch(func() int {
		check.GT(0, 1)
		return 1
	})
	fmt.Println(strings.SplitN(err.Error(), "\n", 2)[0])

	// With catchable errors only a matching error panic is caught.
	_, err = check.Catch(func() int { panic(io.EOF) }, io.EOF)
	fmt.Println(errors.Is(err, io.EOF))

	// Output:
	// 0 > 1
	// true
}

// Eventually polls until the condition holds.
func ExampleEventually() {
	var ready atomic.Bool
	go func() {
		time.Sleep(5 * time.Millisecond)
		ready.Store(true)
	}()
	check.Eventually(time.Second, time.Millisecond, ready.Load, "server ready")
	fmt.Println("ready")

	// Output:
	// ready
}

// Never panics if the condition becomes true within the window.
func ExampleNever() {
	var retries atomic.Int32
	check.Never(30*time.Millisecond, time.Millisecond, func() bool {
		return retries.Load() > 0
	}, "no retry")
	fmt.Println("no retry seen")

	// Output:
	// no retry seen
}
