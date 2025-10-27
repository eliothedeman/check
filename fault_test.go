package check

import "testing"

func TestErrPointNoConfig(t *testing.T) {
	if err := ErrPoint("one"); err != nil {
		t.Error(err)
	}
}

func TestErrPointSingleConfig(t *testing.T) {
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
