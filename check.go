package check

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func panicMsg(base string, msg []string) string {
	if len(msg) > 0 {
		return strings.Join(msg, " ") + ": " + base
	}
	return base
}

func formatCmp[T any](cmp string, a, b T, msg []string) string {
	return panicMsg(fmt.Sprintf("%v %s %v", a, cmp, b), msg)
}

// Eq compares the two input values and panics if they are not equal
func Eq[T comparable](a, b T, msg ...string) {
	if a != b {
		panic(formatCmp("==", a, b, msg))
	}
}

// NotEq compares the two input values and panics if they are equal
func NotEq[T comparable](a, b T, msg ...string) {
	if a == b {
		panic(formatCmp("!=", a, b, msg))
	}
}

// GT panics if a is not greater than b
func GT[T cmp.Ordered](a, b T, msg ...string) {
	if a <= b {
		panic(formatCmp(">", a, b, msg))
	}
}

// LT panics if a is not less than b
func LT[T cmp.Ordered](a, b T, msg ...string) {
	if a >= b {
		panic(formatCmp("<", a, b, msg))
	}
}

// GTE panics if a is not greater than or equal to b
func GTE[T cmp.Ordered](a, b T, msg ...string) {
	if a < b {
		panic(formatCmp(">=", a, b, msg))
	}
}

// LTE panics if a is not less than or equal to b
func LTE[T cmp.Ordered](a, b T, msg ...string) {
	if a > b {
		panic(formatCmp("<=", a, b, msg))
	}
}

// Between panics unless low < a < high.
func Between[T cmp.Ordered](a, low, high T, msg ...string) {
	GT(a, low, msg...)
	LT(a, high, msg...)
}

// BetweenInclusive panics unless low <= a <= high.
func BetweenInclusive[T cmp.Ordered](a, low, high T, msg ...string) {
	GTE(a, low, msg...)
	LTE(a, high, msg...)
}

// isNil reports whether x is nil, including typed nils: a nil pointer, map,
// slice, channel, func or interface boxed into an any.
func isNil(x any) bool {
	if x == nil {
		return true
	}
	switch v := reflect.ValueOf(x); v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	}
	return false
}

// Nil panics if x is not nil. A typed nil (for example a nil *T, map, slice,
// channel or func stored in x) counts as nil.
func Nil(x any, msg ...string) {
	if !isNil(x) {
		panic(panicMsg(fmt.Sprintf("expected nil, got %v", x), msg))
	}
}

// NotNil panics if x is nil. A typed nil (for example a nil *T, map, slice,
// channel or func stored in x) counts as nil.
func NotNil(x any, msg ...string) {
	if isNil(x) {
		panic(panicMsg("expected non-nil", msg))
	}
}

// Is panics if a is not of type T
func Is[T any](a any, msg ...string) {
	if _, ok := a.(T); !ok {
		var want T
		panic(panicMsg(fmt.Sprintf("%v != %T", a, want), msg))
	}
}

// ErrIs panics if e does not match target using errors.Is
func ErrIs(e error, target error, msg ...string) {
	if !errors.Is(e, target) {
		panic(panicMsg(fmt.Sprintf("%v is not %v", e, target), msg))
	}
}

// Must returns t, and panics if err is not nil.
func Must[T any](t T, err error, msg ...string) T {
	Nil(err, msg...)
	return t
}

// Panics executes f and panics if f does not panic
func Panics(f func(), msg ...string) {
	defer func() {
		r := recover()
		if r == nil {
			panic(panicMsg(fmt.Sprintf("Expected %T to panic but it did not", f), msg))
		}
	}()
	f()
}
