package check

import (
	"cmp"
	"testing"
)

type binOp[T cmp.Ordered] struct {
	name string
	cmp  int
	op   func(T, T)
}

func binOps[T cmp.Ordered]() []binOp[T] {
	return []binOp[T]{
		{
			"eq",
			0, Eq[T],
		},
		{
			"gt",
			1, GT[T],
		},
		{
			"lt",
			-1, LT[T],
		},
	}
}

func BenchmarkOps(b *testing.B) {
	ops := binOps[int]()
	for _, o := range ops {
		b.Run(o.name, func(b *testing.B) {
			var x int
			var y int
			if o.cmp == 0 {
				x = 100
				y = 100
			}
			if o.cmp > 0 {
				x = 200
				y = 100
			}
			if o.cmp < 0 {
				x = 50
				y = 100
			}
			for b.Loop() {
				o.op(x, y)
			}
		})
	}
}
