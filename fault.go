package check

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var errLock sync.RWMutex

type errFactory = func(string) error

var ErrFault = errors.New("fault")

func defaultErrFactory(txt string) error {
	return fmt.Errorf("%w: %s", ErrFault, txt)
}

var errDice = rand.NewSource(time.Now().Unix())

type errCfg struct {
	probability int64
	errFactory
	dice   rand.Source
	checks atomic.Int64
	passes atomic.Int64
	fails  atomic.Int64
}

func (e *errCfg) check() bool {
	NotNil(e)
	e.checks.Add(1)
	if e.dice.Int63() <= e.probability {
		e.passes.Add(1)
		return true
	}
	e.fails.Add(1)
	return false
}

func (e *errCfg) err(name string) error {
	if e.probability <= e.dice.Int63() {
		return nil
	}
	return e.errFactory(name)
}

var errCfgMap = map[string]*errCfg{}

type ErrCondition = func(string) error

func ErrPoint(name string) error {
	errLock.RLock()
	defer errLock.RUnlock()

	cfg, ok := errCfgMap[name]
	if !ok {
		return nil
	}
	if !cfg.check() {
		return nil
	}
	return cfg.errFactory(name)
}

type ErrOpt = func(*errCfg)

// Prob takes a probability between 0.0 and 1.0 and applies it to the given error
func Prob[T float32 | float64](p T) ErrOpt {
	BetweenInclusive(p, 0, 1)
	prob := int64(float64(math.MaxInt64) * float64(p))
	return func(e *errCfg) {
		e.probability = prob
	}
}

func ErrCfg(name string, opts ...ErrOpt) {
	cfg := defaultCfg()
	for _, o := range opts {
		o(cfg)
	}
	errLock.Lock()
	defer errLock.Unlock()
	errCfgMap[name] = cfg
}

func defaultCfg() *errCfg {
	cfg := new(errCfg)
	cfg.probability = math.MaxInt64
	cfg.dice = errDice
	cfg.errFactory = defaultErrFactory
	return cfg
}
