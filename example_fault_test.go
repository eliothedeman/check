package check_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/eliothedeman/check"
)

// A fault point with no options always fires.
func ExampleErrCfg() {
	defer check.ErrReset()

	check.ErrCfg("db.query")
	err := check.ErrPoint("db.query")
	fmt.Println(err, errors.Is(err, check.ErrFault))

	// A point that was never configured returns nil.
	fmt.Println(check.ErrPoint("other"))
	fmt.Printf("%+v\n", check.ErrStats("db.query"))

	// Output:
	// fault: db.query true
	// <nil>
	// {Checks:1 Fired:1}
}

func ExampleProb() {
	defer check.ErrReset()

	check.ErrCfg("never", check.Prob(0.0))
	check.ErrCfg("always", check.Prob(1.0))
	fmt.Println(check.ErrPoint("never"), check.ErrPoint("always"))

	// Output:
	// <nil> fault: always
}

// Delay adds latency. ErrPointCtx ends the wait when the context is done.
func ExampleDelay() {
	defer check.ErrReset()

	check.ErrCfg("slow", check.Delay(10*time.Millisecond))
	start := time.Now()
	err := check.ErrPoint("slow")
	fmt.Println(err, time.Since(start) >= 10*time.Millisecond)

	check.ErrCfg("stuck", check.Delay(time.Hour))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err = check.ErrPointCtx(ctx, "stuck")
	fmt.Println(errors.Is(err, context.DeadlineExceeded))

	// Output:
	// <nil> true
	// true
}

func ExampleWithError() {
	defer check.ErrReset()

	check.ErrCfg("net", check.WithError(io.ErrUnexpectedEOF))
	fmt.Println(check.ErrPoint("net") == io.ErrUnexpectedEOF)

	// Output:
	// true
}

// The same seed gives the same sequence of fired and not-fired calls.
func ExampleSeed() {
	defer check.ErrReset()
	defer check.Seed(check.FaultSeed())

	run := func() (seq [8]bool) {
		check.Seed(42)
		check.ErrCfg("flaky", check.Prob(0.5))
		for i := range seq {
			seq[i] = check.ErrPoint("flaky") != nil
		}
		return seq
	}
	fmt.Println(run() == run(), check.FaultSeed())

	// Output:
	// true 42
}

// cleanups stands in for *testing.T, which runs cleanups when a test ends.
type cleanups []func()

func (c *cleanups) Cleanup(f func()) { *c = append(*c, f) }

// WithFaults hides the faults that are configured outside the test and
// restores them when the test ends.
func ExampleWithFaults() {
	check.ErrCfg("outer")
	defer check.ErrReset()

	var t cleanups
	check.WithFaults(&t)
	fmt.Println(check.ErrPoint("outer"))

	check.ErrCfg("inner")
	for _, f := range t { // the test ends
		f()
	}
	fmt.Println(check.ErrPoint("outer"), check.ErrPoint("inner"))

	// Output:
	// <nil>
	// fault: outer <nil>
}
