package check

import (
	"fmt"
	"testing"
)

func TestSliceEq(t *testing.T) {
	table := []struct {
		name   string
		a, b   []int
		passes bool
	}{
		{"equal slices", []int{1, 2, 3}, []int{1, 2, 3}, true},
		{"empty slices", []int{}, []int{}, true},
		{"nil vs nil", nil, nil, true},
		{"nil vs empty", nil, []int{}, true},
		{"different lengths", []int{1, 2}, []int{1, 2, 3}, false},
		{"same length different elements", []int{1, 2, 3}, []int{1, 4, 3}, false},
		{"single element equal", []int{5}, []int{5}, true},
		{"single element different", []int{5}, []int{6}, false},
	}

	for _, x := range table {
		t.Run(fmt.Sprintf("%+v", x), func(t *testing.T) {
			if x.passes {
				SliceEq(x.a, x.b)
			} else {
				Panics(func() {
					SliceEq(x.a, x.b)
				})
			}
		})
	}

	// String slice cases
	t.Run("string slices equal", func(t *testing.T) {
		SliceEq([]string{"a", "b", "c"}, []string{"a", "b", "c"})
	})
	t.Run("string slices different", func(t *testing.T) {
		Panics(func() {
			SliceEq([]string{"a", "b"}, []string{"a", "x"})
		})
	})
}

func TestSliceContains(t *testing.T) {
	table := []struct {
		name   string
		s      []int
		v      int
		passes bool
	}{
		{"element present", []int{1, 2, 3}, 2, true},
		{"element not present", []int{1, 2, 3}, 4, false},
		{"empty slice", []int{}, 1, false},
		{"nil slice", nil, 1, false},
		{"first element", []int{10, 20, 30}, 10, true},
		{"last element", []int{10, 20, 30}, 30, true},
	}

	for _, x := range table {
		t.Run(fmt.Sprintf("%+v", x), func(t *testing.T) {
			if x.passes {
				SliceContains(x.s, x.v)
			} else {
				Panics(func() {
					SliceContains(x.s, x.v)
				})
			}
		})
	}

	// String slice cases
	t.Run("string slice contains", func(t *testing.T) {
		SliceContains([]string{"foo", "bar", "baz"}, "bar")
	})
	t.Run("string slice missing", func(t *testing.T) {
		Panics(func() {
			SliceContains([]string{"foo", "bar", "baz"}, "qux")
		})
	})
}

func TestSliceSorted(t *testing.T) {
	table := []struct {
		name   string
		s      []int
		passes bool
	}{
		{"already sorted", []int{1, 2, 3, 4, 5}, true},
		{"empty slice", []int{}, true},
		{"single element", []int{42}, true},
		{"nil slice", nil, true},
		{"reverse sorted", []int{5, 4, 3, 2, 1}, false},
		{"unsorted in middle", []int{1, 2, 5, 3, 4}, false},
		{"all equal elements", []int{7, 7, 7, 7}, true},
		{"two elements sorted", []int{1, 2}, true},
		{"two elements unsorted", []int{2, 1}, false},
	}

	for _, x := range table {
		t.Run(fmt.Sprintf("%+v", x), func(t *testing.T) {
			if x.passes {
				SliceSorted(x.s)
			} else {
				Panics(func() {
					SliceSorted(x.s)
				})
			}
		})
	}

	// String slice cases
	t.Run("string slice sorted", func(t *testing.T) {
		SliceSorted([]string{"alpha", "beta", "gamma"})
	})
	t.Run("string slice unsorted", func(t *testing.T) {
		Panics(func() {
			SliceSorted([]string{"gamma", "alpha", "beta"})
		})
	})
}
