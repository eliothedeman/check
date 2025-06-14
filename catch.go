package check

import (
	"errors"
	"fmt"
)

func Catch[T any](f func() T, catchable ...error) (out T, err error) {
	defer func() {
		if x := recover(); x != nil {
			switch x := x.(type) {
			case error:
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
				err = x
				recover()
			default:
				err = fmt.Errorf("%v", x)
			}
		}
	}()
	f()
	return
}
