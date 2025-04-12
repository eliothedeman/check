package check

import (
	"cmp"
	"fmt"
)

func formatCmp[T any](cmp string, a, b T) string {
	return fmt.Sprintf("%v %s %v", a, cmp, b)
}

func Eq[T comparable](a, b T) {
	if !(a == b) {
		panic(formatCmp("==", a, b))
	}
}

func NotEq[T comparable](a, b T) {
	if !(a != b) {
		panic(formatCmp("!=", a, b))
	}
}

func GT[T cmp.Ordered](a, b T) {
	if !(a > b) {
		panic(formatCmp(">", a, b))
	}
}

func LT[T cmp.Ordered](a, b T) {
	if !(a < b) {
		panic(formatCmp("<", a, b))
	}
}

func GTE[T cmp.Ordered](a, b T) {
	if !(a >= b) {
		panic(formatCmp(">=", a, b))
	}
}

func LTE[T cmp.Ordered](a, b T) {
	if !(a <= b) {
		panic(formatCmp(">=", a, b))
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
