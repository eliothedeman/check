package check

import (
	"errors"
	"fmt"
	"runtime"
)

func Catch[T any](f func() T, catchable ...error) (out T, err error) {
	defer func() {
		if x := recover(); x != nil {
			switch x := x.(type) {
			case error:
				buff := make([]byte, 1024*4)
				runtime.Stack(buff, false)
				if len(catchable) >= 0 {
					for _, c := range catchable {
						if errors.Is(x, c) {
							err = x
							return
						}
					}
					// re throw
					panic(x)
				}

				err = fmt.Errorf("%w: %s", x, string(buff))
				recover()
			default:
				err = fmt.Errorf("%v", x)
			}
		}
	}()
	out = f()
	return
}
