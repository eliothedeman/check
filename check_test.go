package check

import (
	"fmt"
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
