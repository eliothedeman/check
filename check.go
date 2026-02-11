package check

import (
	"cmp"
	"errors"
	"fmt"
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
		panic(formatCmp(">=", a, b, msg))
	}
}

func Between[T cmp.Ordered](a, low, high T, msg ...string) {
	GT(a, low, msg...)
	LT(a, high, msg...)
}

func BetweenInclusive[T cmp.Ordered](a, low, high T, msg ...string) {
	GTE(a, low, msg...)
	LTE(a, high, msg...)
}

// Nil panics if x is not nil
func Nil(x any, msg ...string) {
	if x != nil {
		panic(formatCmp("!=", x, nil, msg))
	}
}

// NotNil panics if x is nil
func NotNil(x any, msg ...string) {
	if x == nil {
		panic(formatCmp("==", x, nil, msg))
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
