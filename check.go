package check

import (
	"cmp"
	"errors"
	"fmt"
)

func formatCmp[T any](cmp string, a, b T) string {
	return fmt.Sprintf("%v %s %v", a, cmp, b)
}

func Eq[T comparable](a, b T) {
	if a != b {
		panic(formatCmp("==", a, b))
	}
}

func NotEq[T comparable](a, b T) {
	if a == b {
		panic(formatCmp("!=", a, b))
	}
}

func GT[T cmp.Ordered](a, b T) {
	if a <= b {
		panic(formatCmp(">", a, b))
	}
}

func LT[T cmp.Ordered](a, b T) {
	if a >= b {
		panic(formatCmp("<", a, b))
	}
}

func GTE[T cmp.Ordered](a, b T) {
	if a < b {
		panic(formatCmp(">=", a, b))
	}
}

func LTE[T cmp.Ordered](a, b T) {
	if a > b {
		panic(formatCmp(">=", a, b))
	}
}

func Nil[T any](x *T) {
	if x != nil {
		panic(formatCmp("!=", x, nil))
	}
}

func NotNil[T any](x *T) {
	if x == nil {
		panic(formatCmp("==", x, nil))
	}
}

func Is[T any](a any) {
	if _, ok := a.(T); !ok {
		var want T
		panic(fmt.Sprintf("%v != %T", a, want))
	}
}

func ErrIs(e error, target error) {
	if !errors.Is(e, target) {
		panic(fmt.Sprintf("%v is not %v", e, target))
	}
}

func Panics(f func()) {
	defer func() {
		r := recover()
		if r == nil {
			panic(fmt.Sprintf("Expected %T to panic but it did not", f))
		}
	}()
	f()
}
