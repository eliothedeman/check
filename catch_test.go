package check

import (
	"errors"
	"strings"
	"testing"
)

var errSentinel = errors.New("sentinel")

func TestCatchErrorPanicWithoutCatchable(t *testing.T) {
	out, err := Catch(func() int {
		panic(errSentinel)
	})
	if err == nil {
		t.Fatal("expected the panic to be caught")
	}
	if out != 0 {
		t.Errorf("out = %d, want zero value", out)
	}
	if !errors.Is(err, errSentinel) {
		t.Errorf("err %q does not wrap the original panic", err)
	}
	if !strings.Contains(err.Error(), "goroutine") {
		t.Errorf("err has no stack trace: %q", err)
	}
}

func TestCatchStringPanicHasStack(t *testing.T) {
	_, err := Catch(func() string {
		GT(0, 1) // panics with a string
		return "unreachable"
	})
	if err == nil {
		t.Fatal("expected the panic to be caught")
	}
	if !strings.Contains(err.Error(), "0 > 1") {
		t.Errorf("err lost the panic message: %q", err)
	}
	if !strings.Contains(err.Error(), "goroutine") {
		t.Errorf("err has no stack trace: %q", err)
	}
}

func TestCatchableMatchIsCaught(t *testing.T) {
	wrapped := errors.Join(errors.New("context"), errSentinel)
	out, err := Catch(func() int {
		panic(wrapped)
	}, errSentinel)
	if !errors.Is(err, errSentinel) {
		t.Fatalf("err = %v, want it to match the catchable error", err)
	}
	if out != 0 {
		t.Errorf("out = %d, want zero value", out)
	}
}

func TestCatchNonMatchingRePanics(t *testing.T) {
	other := errors.New("other")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("non-matching panic was swallowed")
		}
		if r != other {
			t.Errorf("re-panicked with %v, want the original %v", r, other)
		}
	}()
	_, _ = Catch(func() int {
		panic(other)
	}, errSentinel)
	t.Fatal("Catch returned instead of re-panicking")
}

func TestCatchNonErrorPanicWithCatchableRePanics(t *testing.T) {
	defer func() {
		if r := recover(); r != "boom" {
			t.Errorf("recovered %v, want the original string panic", r)
		}
	}()
	_, _ = Catch(func() int {
		panic("boom")
	}, errSentinel)
	t.Fatal("Catch returned instead of re-panicking")
}

func TestCatchNoPanic(t *testing.T) {
	out, err := Catch(func() int { return 7 })
	if err != nil || out != 7 {
		t.Errorf("got (%d, %v), want (7, nil)", out, err)
	}
}
