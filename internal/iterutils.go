package internal

import "iter"

func first[T any](seq iter.Seq[T]) (T, bool) {
	for x := range seq {
		return x, true
	}
	return *new(T), false
}

func flatmap[T any, U any](
	seq iter.Seq[T],
	f func(T) iter.Seq[U],
) iter.Seq[U] {
	return func(yield func(U) bool) {
		for x := range seq {
			for y := range f(x) {
				if !yield(y) {
					return
				}
			}
		}
	}
}

func filtermap[T, U any](
	seq iter.Seq[T],
	predicate func(T) (U, bool),
) iter.Seq[U] {
	return func(yield func(U) bool) {
		for x := range seq {
			if y, ok := predicate(x); ok && !yield(y) {
				return
			}
		}
	}
}
