package check

import (
	"cmp"
	"fmt"
)

// SliceEq panics if slices a and b are not element-wise equal
func SliceEq[T comparable](a, b []T) {
	if len(a) != len(b) {
		panic(fmt.Sprintf("slice lengths differ: %d != %d\n  a: %v\n  b: %v", len(a), len(b), a, b))
	}
	for i := range a {
		if a[i] != b[i] {
			panic(fmt.Sprintf("slices differ at index %d: %v != %v", i, a[i], b[i]))
		}
	}
}

// SliceContains panics if v is not found in s
func SliceContains[T comparable](s []T, v T) {
	for _, x := range s {
		if x == v {
			return
		}
	}
	panic(fmt.Sprintf("%v not found in %v", v, s))
}

// SliceSorted panics if s is not sorted in ascending order
func SliceSorted[T cmp.Ordered](s []T) {
	for i := 1; i < len(s); i++ {
		if s[i] < s[i-1] {
			panic(fmt.Sprintf("slice not sorted at index %d, %d: %v > %v", i-1, i, s[i-1], s[i]))
		}
	}
}
