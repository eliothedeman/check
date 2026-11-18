package check

import (
	"fmt"
	"strings"
)

// Number is the set of types InDelta accepts: every integer and float type,
// including named types such as time.Duration.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// NoErr panics if err is not nil. The panic message includes the error text.
func NoErr(err error, msg ...string) {
	Nil(err, msg...)
}

// ErrContains panics if err is nil or if its message does not contain substr.
func ErrContains(err error, substr string, msg ...string) {
	if err == nil {
		panic(panicMsg(fmt.Sprintf("expected an error containing %q, got nil", substr), msg))
	}
	if !strings.Contains(err.Error(), substr) {
		panic(panicMsg(fmt.Sprintf("error %q does not contain %q", err.Error(), substr), msg))
	}
}

// InDelta panics unless a and b differ by at most delta. It works for any
// integer or float type, so it also compares time.Duration values. Two floats
// where either is NaN are never within delta. delta must not be negative.
func InDelta[T Number](a, b, delta T, msg ...string) {
	validDelta := delta >= 0 // false for NaN
	if !validDelta {
		panic(panicMsg(fmt.Sprintf("InDelta: delta must not be negative, got %v", delta), msg))
	}
	if a == b {
		return
	}
	d := b - a
	if a > b {
		d = a - b
	}
	// d >= 0 is false for NaN and for a signed overflow; both must fail.
	within := d >= 0 && d <= delta
	if !within {
		panic(panicMsg(fmt.Sprintf("%v and %v differ by more than %v", a, b, delta), msg))
	}
}

// Contains panics if s does not contain substr.
func Contains(s, substr string, msg ...string) {
	if !strings.Contains(s, substr) {
		panic(panicMsg(fmt.Sprintf("%q does not contain %q", s, substr), msg))
	}
}

// NotContains panics if s contains substr.
func NotContains(s, substr string, msg ...string) {
	if strings.Contains(s, substr) {
		panic(panicMsg(fmt.Sprintf("%q unexpectedly contains %q", s, substr), msg))
	}
}
