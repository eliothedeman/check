package check

import (
	"errors"
	"fmt"
	"runtime/debug"
)

// Catch executes f and converts panics into errors. It returns the result of f
// and any caught error.
//
// With no catchable errors, every panic is caught, whether its value is an
// error or not (check.GT and friends panic with a string). The returned error
// carries the stack trace of the panic, and wraps the panic value with %w when
// that value is an error.
//
// With catchable errors, only panics whose value is an error matching one of
// them via errors.Is are caught, and that error is returned as is. Every other
// panic is re-panicked with its original value.
func Catch[T any](f func() T, catchable ...error) (out T, err error) {
	defer func() {
		x := recover()
		if x == nil {
			return
		}
		e, isErr := x.(error)
		if len(catchable) > 0 {
			if isErr {
				for _, c := range catchable {
					if errors.Is(e, c) {
						err = e
						return
					}
				}
			}
			// re throw
			panic(x)
		}
		stack := debug.Stack()
		if isErr {
			err = fmt.Errorf("%w\n%s", e, stack)
			return
		}
		err = fmt.Errorf("%v\n%s", x, stack)
	}()
	out = f()
	return
}
