package check

import (
	"strings"
	"testing"
)

// panicText runs f and returns the panic value as a string. It fails the
// test if f does not panic.
func panicText(t *testing.T, f func()) (msg string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
		s, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		msg = s
	}()
	f()
	return ""
}

type nilThing struct{}

type nilErr struct{}

func (*nilErr) Error() string { return "nilErr" }

func TestNilTyped(t *testing.T) {
	var (
		p  *nilThing
		m  map[string]int
		s  []int
		c  chan int
		fn func()
		e  *nilErr
		ei error = e // non-nil interface holding a nil pointer
	)
	for name, v := range map[string]any{"ptr": p, "map": m, "slice": s, "chan": c, "func": fn, "iface": ei, "untyped": nil} {
		t.Run(name, func(t *testing.T) {
			Nil(v)
			Panics(func() { NotNil(v) })
		})
	}
}

func TestNotNilTyped(t *testing.T) {
	vals := map[string]any{
		"ptr":   &nilThing{},
		"map":   map[string]int{},
		"slice": []int{},
		"chan":  make(chan int),
		"func":  func() {},
		"int":   0,
		"str":   "",
		"err":   &nilErr{},
	}
	for name, v := range vals {
		t.Run(name, func(t *testing.T) {
			NotNil(v)
			Panics(func() { Nil(v) })
		})
	}
}

func TestMustTypedNilError(t *testing.T) {
	var e *nilErr
	var err error = e
	if got := Must(7, err); got != 7 {
		t.Fatalf("got %d", got)
	}
}

func TestLTEMessage(t *testing.T) {
	msg := panicText(t, func() { LTE(5, 3) })
	if !strings.Contains(msg, "5 <= 3") {
		t.Fatalf("LTE message %q does not mention <=", msg)
	}
}

func TestNilMessages(t *testing.T) {
	msg := panicText(t, func() { Nil(42) })
	if !strings.Contains(msg, "expected nil, got 42") {
		t.Fatalf("Nil message = %q", msg)
	}
	msg = panicText(t, func() { NotNil(nil) })
	if !strings.Contains(msg, "expected non-nil") {
		t.Fatalf("NotNil message = %q", msg)
	}
	msg = panicText(t, func() { NotNil((*nilThing)(nil), "ctx") })
	if !strings.HasPrefix(msg, "ctx: ") || !strings.Contains(msg, "expected non-nil") {
		t.Fatalf("NotNil typed message = %q", msg)
	}
}
