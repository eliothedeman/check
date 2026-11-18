package check

import (
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	tick  = time.Millisecond
	short = 40 * time.Millisecond
)

// noGoroutineLeak fails the test if the goroutine count has not returned to
// its starting value shortly after f finishes.
func noGoroutineLeak(t *testing.T, f func()) {
	t.Helper()
	before := runtime.NumGoroutine()
	f()
	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Fatalf("goroutine leak: %d before, %d after", before, runtime.NumGoroutine())
		}
		time.Sleep(tick)
	}
}

func TestEventuallyPasses(t *testing.T) {
	noGoroutineLeak(t, func() {
		Eventually(time.Second, tick, func() bool { return true })

		var n atomic.Int32
		start := time.Now()
		Eventually(time.Second, tick, func() bool { return n.Add(1) >= 5 })
		if time.Since(start) > 500*time.Millisecond {
			t.Fatal("Eventually did not return promptly")
		}
	})
}

func TestEventuallyAsync(t *testing.T) {
	var flag atomic.Bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		flag.Store(true)
	}()
	Eventually(time.Second, tick, flag.Load)
}

func TestEventuallyFails(t *testing.T) {
	noGoroutineLeak(t, func() {
		start := time.Now()
		msg := panicText(t, func() {
			Eventually(short, tick, func() bool { return false }, "ready")
		})
		if time.Since(start) < short {
			t.Fatal("Eventually gave up before the timeout")
		}
		if !strings.HasPrefix(msg, "ready: ") || !strings.Contains(msg, "not met") {
			t.Fatalf("message = %q", msg)
		}
	})
}

func TestEventuallyBadArgs(t *testing.T) {
	Panics(func() { Eventually(0, tick, func() bool { return true }) })
	Panics(func() { Eventually(short, 0, func() bool { return true }) })
}

func TestNeverPasses(t *testing.T) {
	noGoroutineLeak(t, func() {
		start := time.Now()
		Never(short, tick, func() bool { return false })
		if time.Since(start) < short {
			t.Fatal("Never returned before d elapsed")
		}
	})
}

func TestNeverFails(t *testing.T) {
	noGoroutineLeak(t, func() {
		Panics(func() { Never(short, tick, func() bool { return true }) })

		var n atomic.Int32
		start := time.Now()
		msg := panicText(t, func() {
			Never(time.Second, tick, func() bool { return n.Add(1) > 3 }, "no retry")
		})
		if time.Since(start) > 500*time.Millisecond {
			t.Fatal("Never did not fail fast")
		}
		if !strings.HasPrefix(msg, "no retry: ") {
			t.Fatalf("message = %q", msg)
		}
	})
}

func TestNeverBadArgs(t *testing.T) {
	Panics(func() { Never(0, tick, func() bool { return false }) })
	Panics(func() { Never(short, -1, func() bool { return false }) })
}
