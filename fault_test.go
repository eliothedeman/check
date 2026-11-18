package check

import (
	"context"
	"errors"
	"io"
	"math"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestErrPointNoConfig(t *testing.T) {
	if err := ErrPoint("one"); err != nil {
		t.Error(err)
	}
}

func TestErrPointSingleConfig(t *testing.T) {
	WithFaults(t)
	ErrCfg("one")
	NotNil(ErrPoint("one"))
}

func TestProbabilityConfig(t *testing.T) {
	cfg := defaultCfg()
	Eq(cfg.check(), true)
	cfg.probability = 0
	Eq(cfg.check(), false)
	Prob(0.5)(cfg)
	for range 1000 {
		cfg.check()
	}
	GT(cfg.passes.Load(), 450)
	GT(cfg.fails.Load(), 450)
}

// TestErrPointConcurrent hammers one config from many goroutines. It is only
// meaningful under `go test -race`.
func TestErrPointConcurrent(t *testing.T) {
	WithFaults(t)
	ErrCfg("race-point", Prob[float64](0.5))
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 500 {
				_ = ErrPoint("race-point")
			}
		}()
	}
	wg.Wait()
}

// constSource is a rand.Source that always returns the same value.
type constSource int64

func (c constSource) Int63() int64   { return int64(c) }
func (c constSource) Seed(int64)     {}
func (c constSource) Uint64() uint64 { return uint64(c) }

func TestProbZeroNeverFires(t *testing.T) {
	// Worst case source: always draws 0.
	cfg := defaultCfg()
	cfg.dice = constSource(0)
	Prob(0.0)(cfg)
	for range 10000 {
		if cfg.check() {
			t.Fatal("Prob(0.0) fired")
		}
	}
	// Worst case in the other direction: always draws the maximum.
	cfg = defaultCfg()
	cfg.dice = constSource(math.MaxInt64)
	Prob(1.0)(cfg)
	for range 10000 {
		if !cfg.check() {
			t.Fatal("Prob(1.0) did not fire")
		}
	}
}

func TestProbZeroNeverFiresSeeded(t *testing.T) {
	WithFaults(t)
	ErrCfg("zero", Prob(0.0), WithSeed(1))
	for range 100000 {
		if err := ErrPoint("zero"); err != nil {
			t.Fatal("Prob(0.0) fired:", err)
		}
	}
	ErrCfg("one", Prob(1.0), WithSeed(1))
	for range 100000 {
		if err := ErrPoint("one"); err == nil {
			t.Fatal("Prob(1.0) did not fire")
		}
	}
}

func TestErrReset(t *testing.T) {
	WithFaults(t)
	ErrCfg("a")
	ErrCfg("b")
	if ErrPoint("a") == nil || ErrPoint("b") == nil {
		t.Fatal("configured faults did not fire")
	}
	ErrReset()
	if err := ErrPoint("a"); err != nil {
		t.Error("a fired after ErrReset:", err)
	}
	if err := ErrPoint("b"); err != nil {
		t.Error("b fired after ErrReset:", err)
	}
}

func TestErrRemove(t *testing.T) {
	WithFaults(t)
	ErrCfg("a")
	ErrCfg("b")
	ErrRemove("a")
	ErrRemove("never-configured") // must not panic
	if err := ErrPoint("a"); err != nil {
		t.Error("a fired after ErrRemove:", err)
	}
	if ErrPoint("b") == nil {
		t.Error("b should be untouched by ErrRemove(a)")
	}
}

func TestWithFaultsIsolatesAndRestores(t *testing.T) {
	WithFaults(t)
	ErrCfg("outer")
	t.Run("inner", func(t *testing.T) {
		WithFaults(t)
		if err := ErrPoint("outer"); err != nil {
			t.Error("outer fault leaked into the inner test:", err)
		}
		ErrCfg("inner")
		if ErrPoint("inner") == nil {
			t.Error("inner fault did not fire")
		}
	})
	if ErrPoint("outer") == nil {
		t.Error("outer fault was not restored after the inner test")
	}
	if err := ErrPoint("inner"); err != nil {
		t.Error("inner fault leaked out of the inner test:", err)
	}
}

func fireSequence(name string, n int) []bool {
	out := make([]bool, n)
	for i := range out {
		out[i] = ErrPoint(name) != nil
	}
	return out
}

func TestSeedReproducible(t *testing.T) {
	WithFaults(t)
	orig := FaultSeed()
	t.Cleanup(func() { Seed(orig) })

	Seed(42)
	if got := FaultSeed(); got != 42 {
		t.Fatalf("FaultSeed() = %d, want 42", got)
	}
	ErrCfg("seeded", Prob(0.5))
	first := fireSequence("seeded", 200)

	Seed(42)
	ErrCfg("seeded", Prob(0.5))
	second := fireSequence("seeded", 200)

	if !slices.Equal(first, second) {
		t.Fatal("same seed produced different fired sequences")
	}
	if !slices.Contains(first, true) || !slices.Contains(first, false) {
		t.Fatal("sequence is degenerate; Prob(0.5) should both fire and not fire")
	}

	// Creation order must not matter: the source derives from (seed, name).
	Seed(42)
	ErrCfg("other", Prob(0.5))
	ErrCfg("seeded", Prob(0.5))
	third := fireSequence("seeded", 200)
	if !slices.Equal(first, third) {
		t.Fatal("sequence depends on config creation order")
	}

	Seed(43)
	ErrCfg("seeded", Prob(0.5))
	if slices.Equal(first, fireSequence("seeded", 200)) {
		t.Fatal("different seeds produced the same sequence")
	}
}

func TestWithSeedReproducible(t *testing.T) {
	WithFaults(t)
	ErrCfg("x", Prob(0.5), WithSeed(7))
	ErrCfg("y", Prob(0.5), WithSeed(7))
	ErrCfg("z", Prob(0.5), WithSeed(8))
	x, y, z := fireSequence("x", 200), fireSequence("y", 200), fireSequence("z", 200)
	if !slices.Equal(x, y) {
		t.Error("same WithSeed on two configs gave different sequences")
	}
	if slices.Equal(x, z) {
		t.Error("different WithSeed gave the same sequence")
	}
}

func TestErrStats(t *testing.T) {
	WithFaults(t)
	if got := ErrStats("unknown"); got != (FaultStats{}) {
		t.Errorf("unknown fault stats = %+v, want zero", got)
	}
	ErrCfg("always", Prob(1.0))
	ErrCfg("never", Prob(0.0))
	for range 5 {
		_ = ErrPoint("always")
	}
	for range 3 {
		_ = ErrPoint("never")
	}
	if got, want := ErrStats("always"), (FaultStats{Checks: 5, Fired: 5}); got != want {
		t.Errorf("always stats = %+v, want %+v", got, want)
	}
	if got, want := ErrStats("never"), (FaultStats{Checks: 3, Fired: 0}); got != want {
		t.Errorf("never stats = %+v, want %+v", got, want)
	}
	ErrResetStats("always")
	if got := ErrStats("always"); got != (FaultStats{}) {
		t.Errorf("stats after ErrResetStats = %+v, want zero", got)
	}
	if got := ErrStats("never"); got.Checks != 3 {
		t.Errorf("ErrResetStats(always) touched never: %+v", got)
	}
	ErrResetStats("unknown") // must not panic
	_ = ErrPoint("always")
	if got := ErrStats("always"); got != (FaultStats{Checks: 1, Fired: 1}) {
		t.Errorf("stats did not count after reset: %+v", got)
	}
}

func TestDelay(t *testing.T) {
	WithFaults(t)
	ErrCfg("slow", Delay(50*time.Millisecond))
	start := time.Now()
	err := ErrPoint("slow")
	if el := time.Since(start); el < 50*time.Millisecond {
		t.Errorf("ErrPoint returned after %v, want >= 50ms", el)
	}
	if err != nil {
		t.Errorf("Delay alone should return nil, got %v", err)
	}
	if got := ErrStats("slow").Fired; got != 1 {
		t.Errorf("fired = %d, want 1", got)
	}
}

func TestDelayWithError(t *testing.T) {
	WithFaults(t)
	ErrCfg("slow-err", Delay(20*time.Millisecond), WithError(io.ErrUnexpectedEOF))
	start := time.Now()
	err := ErrPoint("slow-err")
	if el := time.Since(start); el < 20*time.Millisecond {
		t.Errorf("returned after %v, want >= 20ms", el)
	}
	if err != io.ErrUnexpectedEOF {
		t.Errorf("err = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestDelayComposesWithProb(t *testing.T) {
	WithFaults(t)
	// Never fires, so the hour-long delay must never be taken.
	ErrCfg("prob-zero", Delay(time.Hour), Prob(0.0))
	start := time.Now()
	for range 100 {
		if err := ErrPoint("prob-zero"); err != nil {
			t.Fatal(err)
		}
	}
	if el := time.Since(start); el > time.Second {
		t.Fatalf("Delay with Prob(0.0) slept: %v", el)
	}
	// Fires about half the time.
	ErrCfg("prob-half", Delay(time.Millisecond), Prob(0.5), WithSeed(3))
	for range 100 {
		_ = ErrPoint("prob-half")
	}
	got := ErrStats("prob-half")
	if got.Checks != 100 || got.Fired < 20 || got.Fired > 80 {
		t.Errorf("stats = %+v, want 100 checks and roughly half fired", got)
	}
}

func TestErrPointCtxHonoursContext(t *testing.T) {
	WithFaults(t)
	ErrCfg("stall", Delay(time.Hour))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := ErrPointCtx(ctx, "stall")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
	if el := time.Since(start); el > time.Second {
		t.Errorf("ErrPointCtx did not return promptly: %v", el)
	}
	if got := ErrPointCtx(context.Background(), "no-such-fault"); got != nil {
		t.Errorf("unconfigured ErrPointCtx = %v, want nil", got)
	}
}

func TestWithError(t *testing.T) {
	WithFaults(t)
	sentinel := errors.New("backend 503")
	ErrCfg("specific", WithError(sentinel))
	if err := ErrPoint("specific"); err != sentinel {
		t.Errorf("err = %v, want exactly the supplied error", err)
	}
	ErrCfg("default")
	err := ErrPoint("default")
	if !errors.Is(err, ErrFault) {
		t.Errorf("default err = %v, want it to wrap ErrFault", err)
	}
	if errors.Is(err, sentinel) {
		t.Error("default err matched an unrelated sentinel")
	}
}

func TestDelayZeroReturnsNil(t *testing.T) {
	WithFaults(t)
	ErrCfg("zero-delay", Delay(0))
	if err := ErrPoint("zero-delay"); err != nil {
		t.Errorf("Delay(0) alone should return nil, got %v", err)
	}
}

func TestProbNaNPanics(t *testing.T) {
	Panics(func() { Prob(math.NaN()) })
	Panics(func() { Prob(float32(math.NaN())) })
}
