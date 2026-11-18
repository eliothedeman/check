package check

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// errLock guards errCfgMap itself. It is never held while a fault sleeps.
var errLock sync.RWMutex

type errFactory = func(string) error

// ErrFault is the error wrapped by every fault that has no explicit WithError.
var ErrFault = errors.New("fault")

func defaultErrFactory(txt string) error {
	return fmt.Errorf("%w: %s", ErrFault, txt)
}

// globalSeed is the seed that configs created without WithSeed derive from.
// It defaults to the current time and can be replaced with Seed.
var globalSeed atomic.Int64

func init() {
	globalSeed.Store(time.Now().UnixNano())
}

// Seed sets the global seed. Fault points configured afterwards with ErrCfg,
// and without WithSeed, draw from a source derived from this seed and a hash
// of their name, so the same seed replays the same fired/not-fired sequence
// per name regardless of the order in which points were configured.
// Points that are already configured keep the source they were created with.
func Seed(seed int64) {
	globalSeed.Store(seed)
}

// FaultSeed returns the current global seed. Log it when a test fails so the
// run can be replayed with Seed.
func FaultSeed() int64 {
	return globalSeed.Load()
}

// nameSeed derives a per-name seed from the global seed and an FNV hash of name.
func nameSeed(seed int64, name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return seed ^ int64(h.Sum64())
}

// probScale is 2^63: the number of distinct values rand.Source.Int63 returns.
const probScale = float64(1 << 63)

type errCfg struct {
	// probability is the fire threshold: a draw d fires when uint64(d) < probability.
	// Draws lie in [0, 2^63), so 0 never fires and 1<<63 always fires.
	probability uint64
	errFactory
	// errSet reports whether WithError was given.
	errSet bool
	// delay is slept when the fault fires; delaySet reports whether Delay was given.
	delay    time.Duration
	delaySet bool
	// seeded reports whether WithSeed was given.
	seeded bool

	// mu guards dice, which is not safe for concurrent use.
	mu     sync.Mutex
	dice   rand.Source
	checks atomic.Int64
	passes atomic.Int64
	fails  atomic.Int64
}

// draw takes one value from the config's own source.
func (e *errCfg) draw() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dice.Int63()
}

func (e *errCfg) check() bool {
	NotNil(e)
	e.checks.Add(1)
	if uint64(e.draw()) < e.probability {
		e.passes.Add(1)
		return true
	}
	e.fails.Add(1)
	return false
}

var errCfgMap = map[string]*errCfg{}

// ErrCondition is the signature of a function that builds a fault's error.
type ErrCondition = func(string) error

// ErrPoint is a fault injection point. It returns nil unless a fault was
// configured for name with ErrCfg and fired on this call. A fault with Delay
// sleeps before returning. Use ErrPointCtx to bound that sleep with a context.
func ErrPoint(name string) error {
	return ErrPointCtx(context.Background(), name)
}

// ErrPointCtx is ErrPoint with a context. If the fault fires and has a Delay,
// the sleep ends early when ctx is done, and ErrPointCtx returns ctx.Err().
func ErrPointCtx(ctx context.Context, name string) error {
	errLock.RLock()
	cfg, ok := errCfgMap[name]
	errLock.RUnlock()
	if !ok {
		return nil
	}
	if !cfg.check() {
		return nil
	}
	if cfg.delay > 0 {
		t := time.NewTimer(cfg.delay)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		}
	}
	if cfg.delaySet && !cfg.errSet {
		return nil
	}
	return cfg.errFactory(name)
}

// ErrOpt configures a fault point. See ErrCfg.
type ErrOpt = func(*errCfg)

// Prob takes a probability between 0.0 and 1.0 and applies it to the given error.
// Prob(0) never fires and Prob(1) always fires.
func Prob[T float32 | float64](p T) ErrOpt {
	BetweenInclusive(p, 0, 1)
	var prob uint64
	if p >= 1 {
		prob = 1 << 63
	} else {
		prob = uint64(float64(p) * probScale)
	}
	return func(e *errCfg) {
		e.probability = prob
	}
}

// WithSeed seeds this fault point's random source, overriding the global seed.
func WithSeed(seed int64) ErrOpt {
	return func(e *errCfg) {
		e.dice = rand.NewSource(seed)
		e.seeded = true
	}
}

// WithError makes the fault return exactly err (not wrapped) when it fires.
// Without it a fault returns an error wrapping ErrFault. Combined with Delay,
// the fault sleeps and then returns err.
func WithError(err error) ErrOpt {
	NotNil(err)
	return func(e *errCfg) {
		e.errFactory = func(string) error { return err }
		e.errSet = true
	}
}

// Delay makes the fault sleep for d when it fires. Unless WithError is also
// given, ErrPoint then returns nil, so the fault only adds latency. Combine
// with Prob to delay a fraction of calls.
func Delay(d time.Duration) ErrOpt {
	return func(e *errCfg) {
		e.delay = d
		e.delaySet = true
	}
}

// ErrCfg configures the fault point name, replacing any previous config and
// resetting its stats. With no options the fault always fires and returns an
// error wrapping ErrFault.
func ErrCfg(name string, opts ...ErrOpt) {
	cfg := defaultCfg()
	for _, o := range opts {
		o(cfg)
	}
	if !cfg.seeded {
		cfg.dice = rand.NewSource(nameSeed(FaultSeed(), name))
	}
	errLock.Lock()
	defer errLock.Unlock()
	errCfgMap[name] = cfg
}

// ErrReset removes every configured fault point.
func ErrReset() {
	errLock.Lock()
	defer errLock.Unlock()
	errCfgMap = map[string]*errCfg{}
}

// ErrRemove removes the fault point name. Unknown names are ignored.
func ErrRemove(name string) {
	errLock.Lock()
	defer errLock.Unlock()
	delete(errCfgMap, name)
}

// Cleanuper is satisfied by *testing.T, *testing.B and *testing.F.
type Cleanuper interface {
	Cleanup(func())
}

// WithFaults isolates a test's faults. It saves the current fault points,
// removes them all, and registers a t.Cleanup that restores the saved ones, so
// faults configured by the test do not leak into later tests.
func WithFaults(t Cleanuper) {
	errLock.Lock()
	saved := errCfgMap
	errCfgMap = map[string]*errCfg{}
	errLock.Unlock()
	t.Cleanup(func() {
		errLock.Lock()
		defer errLock.Unlock()
		errCfgMap = saved
	})
}

// FaultStats is a snapshot of how often a fault point was reached and fired.
type FaultStats struct {
	// Checks is the number of times the fault point was reached.
	Checks int64
	// Fired is the number of times the fault fired.
	Fired int64
}

// ErrStats returns the stats for the fault point name, or the zero value if it
// is not configured.
func ErrStats(name string) FaultStats {
	errLock.RLock()
	cfg, ok := errCfgMap[name]
	errLock.RUnlock()
	if !ok {
		return FaultStats{}
	}
	return FaultStats{Checks: cfg.checks.Load(), Fired: cfg.passes.Load()}
}

// ErrResetStats zeroes the stats for the fault point name, leaving its config
// in place. Unknown names are ignored.
func ErrResetStats(name string) {
	errLock.RLock()
	cfg, ok := errCfgMap[name]
	errLock.RUnlock()
	if !ok {
		return
	}
	cfg.checks.Store(0)
	cfg.passes.Store(0)
	cfg.fails.Store(0)
}

func defaultCfg() *errCfg {
	cfg := new(errCfg)
	cfg.probability = 1 << 63
	cfg.dice = rand.NewSource(FaultSeed())
	cfg.errFactory = defaultErrFactory
	return cfg
}
