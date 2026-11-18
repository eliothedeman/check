package check_test

import (
	"fmt"
	"strings"

	"github.com/eliothedeman/check"
)

// printTB is a stand-in for *testing.T that prints the first line of each
// failure. A real test passes its *testing.T.
type printTB struct{}

func (printTB) Helper() {}

func (printTB) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(strings.SplitN(msg, "\n", 2)[0])
}

func (printTB) FailNow() {}

// Recover is deferred at the top of a test. It turns an assertion panic into
// t.Errorf plus t.FailNow.
func ExampleRecover() {
	t := printTB{}
	func() {
		defer check.Recover(t)
		check.Eq(1, 2, "count")
	}()

	// Output:
	// count: 1 == 2
}

// Run reports a failure with t.Errorf only, so it is safe in any goroutine.
func ExampleRun() {
	t := printTB{}
	ok := check.Run(t, func() {
		check.GT(1, 2)
	})
	fmt.Println(ok)

	// Output:
	// 1 > 2
	// false
}
