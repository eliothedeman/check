package check

import (
	"cmp"
	"errors"
	"fmt"
)

func formatCmp[T any](cmp string, a, b T) string {
	return fmt.Sprintf("%v %s %v", a, cmp, b)
}

// Eq compares the two input values and panics if they are not equal
func Eq[T comparable](a, b T) {
	if a != b {
		panic(formatCmp("==", a, b))
	}
}

// NotEq compares the two input values and panics if they are equal
func NotEq[T comparable](a, b T) {
	if a == b {
		panic(formatCmp("!=", a, b))
	}
}

// GT panics if a is not greater than b
func GT[T cmp.Ordered](a, b T) {
	if a <= b {
		panic(formatCmp(">", a, b))
	}
}

// LT panics if a is not less than b
func LT[T cmp.Ordered](a, b T) {
	if a >= b {
		panic(formatCmp("<", a, b))
	}
}

// GTE panics if a is not greater than or equal to b
func GTE[T cmp.Ordered](a, b T) {
	if a < b {
		panic(formatCmp(">=", a, b))
	}
}

// LTE panics if a is not less than or equal to b
func LTE[T cmp.Ordered](a, b T) {
	if a > b {
		panic(formatCmp(">=", a, b))
	}
}

func Between[T cmp.Ordered](a, low, high T) {
	GT(a, low)
	LT(a, high)
}

func BetweenInclusive[T cmp.Ordered](a, low, high T) {
	GTE(a, low)
	LTE(a, high)
}

// Nil panics if x is not nil
func Nil(x any) {
	if x != nil {
		panic(formatCmp("!=", x, nil))
	}
}

// NotNil panics if x is nil
func NotNil(x any) {
	if x == nil {
		panic(formatCmp("==", x, nil))
	}
}

// Is panics if a is not of type T
func Is[T any](a any) {
	if _, ok := a.(T); !ok {
		var want T
		panic(fmt.Sprintf("%v != %T", a, want))
	}
}

// ErrIs panics if e does not match target using errors.Is
func ErrIs(e error, target error) {
	if !errors.Is(e, target) {
		panic(fmt.Sprintf("%v is not %v", e, target))
	}
}

func Must[T any](t T, err error) T {
	Nil(err)
	return t
}

// Panics executes f and panics if f does not panic
func Panics(f func()) {
	defer func() {
		r := recover()
		if r == nil {
			panic(fmt.Sprintf("Expected %T to panic but it did not", f))
		}
	}()
	f()
}
