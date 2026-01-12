package streams

// SliceStream creates a stream from a slice of items.
func SliceStream[T any](items []T) Stream[T] {
	return &sliceStream[T]{
		items: items,
		index: 0,
	}
}

type sliceStream[T any] struct {
	items []T
	index int
}

func (s *sliceStream[T]) Next() bool {
	return s.index < len(s.items)
}

func (s *sliceStream[T]) Current() T {
	if s.index < len(s.items) {
		item := s.items[s.index]
		s.index++

		return item
	}

	var zero T

	return zero
}

func (s *sliceStream[T]) Err() error {
	return nil
}

func (s *sliceStream[T]) Close() error {
	return nil
}
