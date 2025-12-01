package check

import (
	"fmt"
	"os"
	"testing"
)

func TestPanics(t *testing.T) {
	called := new(bool)
	Panics(func() {
		*called = true
		panic(called)
	})
	if !*called {
		t.Error("Expdected panic in function")
	}
}

func TestCmp(t *testing.T) {
	table := []struct {
		a, b   int
		cmp    func(int, int)
		passes bool
	}{
		{1, 0, GT[int], true},
		{0, 0, GT[int], false},
		{0, 0, GTE[int], true},
		{100, 0, GTE[int], true},
	}

	for _, x := range table {
		t.Run(fmt.Sprintf("%+v", x), func(t *testing.T) {
			if x.passes {
				x.cmp(x.a, x.b)
			} else {
				Panics(func() {
					x.cmp(x.a, x.b)
				})
			}
		})
	}
}

func TestErr(t *testing.T) {
	table := []struct {
		a, b   error
		passes bool
	}{
		{os.ErrClosed, os.ErrClosed, true},
		{os.ErrClosed, os.ErrDeadlineExceeded, false},
		{fmt.Errorf("%w", os.ErrClosed), os.ErrClosed, true},
		{os.ErrExist, nil, false},
		{nil, os.ErrDeadlineExceeded, false},
		{nil, nil, true},
	}

	for _, x := range table {
		t.Run(fmt.Sprintf("%+v", x), func(t *testing.T) {
			if x.passes {
				ErrIs(x.a, x.b)
			} else {
				Panics(func() {
					ErrIs(x.a, x.b)
				})
			}
		})
	}
}

func TestMust(t *testing.T) {
	// Test successful case: no error
	result := Must(42, nil)
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	// Test successful case: string value
	strResult := Must("hello", nil)
	if strResult != "hello" {
		t.Errorf("Expected 'hello', got '%s'", strResult)
	}

	// Test panic case: with error
	Panics(func() {
		Must(42, os.ErrClosed)
	})

	// Test panic case: with wrapped error
	Panics(func() {
		Must("value", fmt.Errorf("wrapped error"))
	})

	// Test with custom error
	customErr := fmt.Errorf("custom error message")
	Panics(func() {
		Must(0, customErr)
	})
}
