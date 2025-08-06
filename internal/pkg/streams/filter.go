package streams

// Filter creates a new stream that filters items from the source stream
// based on the provided predicate function.
func Filter[T any](stream Stream[T], predicate func(T) bool) Stream[T] {
	return &filterStream[T]{
		stream:    stream,
		predicate: predicate,
	}
}

type filterStream[T any] struct {
	stream      Stream[T]
	predicate   func(T) bool
	current     T
	initialized bool
}

func (s *filterStream[T]) Next() bool {
	for s.stream.Next() {
		item := s.stream.Current()
		if s.predicate(item) {
			s.current = item
			s.initialized = true

			return true
		}
		// Skip items that don't match the predicate
	}

	return false
}

func (s *filterStream[T]) Current() T {
	return s.current
}

func (s *filterStream[T]) Err() error {
	return s.stream.Err()
}

func (s *filterStream[T]) Close() error {
	return s.stream.Close()
}
