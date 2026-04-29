package streams

func PrependStream[T any](stream Stream[T], items ...T) Stream[T] {
	return &prependStream[T]{
		stream:       stream,
		prependItems: items,
		prependIndex: 0,
	}
}

type prependStream[T any] struct {
	stream       Stream[T]
	prependItems []T
	prependIndex int
	current      T
}

func (s *prependStream[T]) Next() bool {
	if s.prependIndex < len(s.prependItems) {
		s.current = s.prependItems[s.prependIndex]
		s.prependIndex++

		return true
	}

	if s.stream.Next() {
		s.current = s.stream.Current()
		return true
	}

	return false
}

func (s *prependStream[T]) Current() T   { return s.current }
func (s *prependStream[T]) Err() error   { return s.stream.Err() }
func (s *prependStream[T]) Close() error { return s.stream.Close() }
