package check_test

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/eliothedeman/check"
)

// fakeTB records calls so tests can assert on failure reporting.
type fakeTB struct {
	mu      sync.Mutex
	errs    []string
	failed  int
	helpers int
}

func (f *fakeTB) Helper() {
	f.mu.Lock()
	f.helpers++
	f.mu.Unlock()
}

func (f *fakeTB) Errorf(format string, args ...any) {
	f.mu.Lock()
	f.errs = append(f.errs, fmt.Sprintf(format, args...))
	f.mu.Unlock()
}

func (f *fakeTB) FailNow() {
	f.mu.Lock()
	f.failed++
	f.mu.Unlock()
}

// here returns "file:line" of the line that calls it, plus offset lines.
func here(offset int) string {
	_, file, line, _ := runtime.Caller(1)
	return fmt.Sprintf("%s:%d", file, line+offset)
}

func TestRecoverReportsAssertion(t *testing.T) {
	f := &fakeTB{}
	var want string
	func() {
		defer check.Recover(f)
		want = here(1)
		check.Eq(1, 2, "numbers")
	}()
	if len(f.errs) != 1 || f.failed != 1 {
		t.Fatalf("errs=%v failed=%d", f.errs, f.failed)
	}
	for _, sub := range []string{"numbers: 1 == 2", want} {
		if !strings.Contains(f.errs[0], sub) {
			t.Errorf("message %q lacks %q", f.errs[0], sub)
		}
	}
}

func TestRecoverNoPanic(t *testing.T) {
	f := &fakeTB{}
	func() {
		defer check.Recover(f)
	}()
	if len(f.errs) != 0 || f.failed != 0 {
		t.Fatalf("unexpected report: errs=%v failed=%d", f.errs, f.failed)
	}
}

func TestRecoverReportsForeignPanic(t *testing.T) {
	f := &fakeTB{}
	var want string
	func() {
		defer check.Recover(f)
		want = here(1)
		panic("boom")
	}()
	if len(f.errs) != 1 || f.failed != 1 {
		t.Fatalf("errs=%v failed=%d", f.errs, f.failed)
	}
	for _, sub := range []string{"boom", want} {
		if !strings.Contains(f.errs[0], sub) {
			t.Errorf("message %q lacks %q", f.errs[0], sub)
		}
	}
}

func TestRunPass(t *testing.T) {
	f := &fakeTB{}
	if !check.Run(f, func() { check.Eq(1, 1) }) {
		t.Fatal("Run returned false for a passing function")
	}
	if len(f.errs) != 0 {
		t.Fatalf("unexpected errors: %v", f.errs)
	}
}

func TestRunFail(t *testing.T) {
	f := &fakeTB{}
	var want string
	ok := check.Run(f, func() {
		want = here(1)
		check.GT(1, 2)
	})
	if ok {
		t.Fatal("Run returned true for a failing function")
	}
	if f.failed != 0 {
		t.Fatal("Run must not call FailNow")
	}
	if len(f.errs) != 1 {
		t.Fatalf("errs=%v", f.errs)
	}
	for _, sub := range []string{"1 > 2", want} {
		if !strings.Contains(f.errs[0], sub) {
			t.Errorf("message %q lacks %q", f.errs[0], sub)
		}
	}
}

func TestRunFromGoroutines(t *testing.T) {
	f := &fakeTB{}
	const n = 8
	results := make([]bool, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = check.Run(f, func() { check.Eq(i, 0) })
		}()
	}
	wg.Wait()
	if !results[0] {
		t.Error("goroutine 0 should pass")
	}
	for i := 1; i < n; i++ {
		if results[i] {
			t.Errorf("goroutine %d should fail", i)
		}
	}
	if len(f.errs) != n-1 || f.failed != 0 {
		t.Fatalf("errs=%d failed=%d", len(f.errs), f.failed)
	}
}

// TestRealT shows the intended use with a real *testing.T.
func TestRealT(t *testing.T) {
	defer check.Recover(t)
	check.Eq(2, 1+1)
	if !check.Run(t, func() { check.NoErr(nil) }) {
		t.Fatal("Run failed")
	}
	done := make(chan bool)
	go func() { done <- check.Run(t, func() { check.Contains("abc", "b") }) }()
	if !<-done {
		t.Fatal("goroutine Run failed")
	}
}
