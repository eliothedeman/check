package check

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestNoErr(t *testing.T) {
	NoErr(nil)
	msg := panicText(t, func() { NoErr(errors.New("kaput"), "loading") })
	if !strings.HasPrefix(msg, "loading: ") || !strings.Contains(msg, "kaput") {
		t.Fatalf("message = %q", msg)
	}
	var e *nilErr
	NoErr(e) // typed nil pointer counts as nil
}

func TestErrContains(t *testing.T) {
	ErrContains(fmt.Errorf("wrap: %w", errors.New("disk full")), "disk")
	Panics(func() { ErrContains(nil, "x") })
	Panics(func() { ErrContains(errors.New("a"), "b") })
	msg := panicText(t, func() { ErrContains(errors.New("a"), "b", "ctx") })
	if !strings.Contains(msg, `"a"`) || !strings.Contains(msg, `"b"`) || !strings.HasPrefix(msg, "ctx: ") {
		t.Fatalf("message = %q", msg)
	}
}

func TestInDelta(t *testing.T) {
	InDelta(1.0, 1.05, 0.1)
	InDelta(1.05, 1.0, 0.1)
	InDelta(5, 5, 0)
	InDelta(uint(3), uint(5), 2)
	InDelta(uint8(250), uint8(5), 255)
	InDelta(100*time.Millisecond, 130*time.Millisecond, 50*time.Millisecond)
	InDelta(130*time.Millisecond, 100*time.Millisecond, 30*time.Millisecond)

	Panics(func() { InDelta(1.0, 1.2, 0.1) })
	Panics(func() { InDelta(1.2, 1.0, 0.1) })
	Panics(func() { InDelta(uint(3), uint(10), 2) })
	Panics(func() { InDelta(math.NaN(), 1.0, 10) })
	Panics(func() { InDelta(1.0, 1.0, math.NaN()) })
	Panics(func() { InDelta(1, 2, -5) })
	// the difference overflows int64 and must not wrap into range
	Panics(func() { InDelta(int64(math.MinInt64), int64(math.MaxInt64), 5) })
	Panics(func() { InDelta(time.Second, 3*time.Second, time.Second) })
	msg := panicText(t, func() { InDelta(1, 9, 2, "latency") })
	if !strings.HasPrefix(msg, "latency: ") || !strings.Contains(msg, "1 and 9") {
		t.Fatalf("message = %q", msg)
	}
}

func TestContains(t *testing.T) {
	Contains("data: hello", "hello")
	NotContains("data: hello", "bye")
	Panics(func() { Contains("data: hello", "bye") })
	Panics(func() { NotContains("data: hello", "hello") })
	msg := panicText(t, func() { Contains("abc", "z", "frame") })
	if !strings.HasPrefix(msg, "frame: ") {
		t.Fatalf("message = %q", msg)
	}
}
