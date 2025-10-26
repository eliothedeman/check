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
