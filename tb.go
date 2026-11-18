package check

import (
	"fmt"
	"runtime"
	"strings"
)

// TB is the part of testing.TB that Recover and Run need. *testing.T,
// *testing.B and *testing.F satisfy it. It is declared here so this package
// does not import "testing".
type TB interface {
	Helper()
	Errorf(format string, args ...any)
	FailNow()
}

// Recover turns an assertion panic into a test failure. Defer it at the top of
// a test:
//
//	func TestX(t *testing.T) {
//		defer check.Recover(t)
//		check.Eq(got, want)
//	}
//
// On a panic it calls t.Errorf with the panic value and the file:line of the
// caller of the failing assertion, then t.FailNow. It reports panics that did
// not come from this package too. Recover does nothing if there is no panic.
//
// Recover must be deferred directly (defer check.Recover(t)) and must run on
// the test goroutine, because t.FailNow must. In any other goroutine use Run.
func Recover(t TB) {
	r := recover()
	if r == nil {
		return
	}
	t.Helper()
	frame, found := callerOutsideCheck()
	t.Errorf("%s", describePanic(r, frame, found))
	t.FailNow()
}

// Run calls f and reports whether it finished without a panic. If f panics, Run
// calls t.Errorf with the panic value and the file:line of the caller of the
// failing assertion, and returns false. It never calls t.FailNow, so it is
// safe to call from any goroutine, and a failed assertion in a goroutine fails
// one test instead of killing the test binary.
func Run(t TB, f func()) (ok bool) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		t.Helper()
		frame, found := callerOutsideCheck()
		t.Errorf("%s", describePanic(r, frame, found))
		ok = false
	}()
	f()
	return true
}

const checkPkgPrefix = "github.com/eliothedeman/check."

// callerOutsideCheck returns the first stack frame that is neither in the
// runtime nor in this package. When called from a deferred function while a
// panic is unwinding, that frame is the caller of the failing assertion.
func callerOutsideCheck() (frame runtime.Frame, found bool) {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if f.Function != "" &&
			!strings.HasPrefix(f.Function, checkPkgPrefix) &&
			!strings.HasPrefix(f.Function, "runtime.") {
			return f, true
		}
		if !more {
			return runtime.Frame{}, false
		}
	}
}

func describePanic(r any, frame runtime.Frame, found bool) string {
	if !found {
		return fmt.Sprint(r)
	}
	return fmt.Sprintf("%s\n\tat %s:%d", fmt.Sprint(r), frame.File, frame.Line)
}
