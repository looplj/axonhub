package streams

// AppendStream creates a new stream that appends additional items to the end of the source stream.
func AppendStream[T any](stream Stream[T], items ...T) Stream[T] {
	return &appendStream[T]{
		stream:      stream,
		appendItems: items,
		appendIndex: 0,
		streamDone:  false,
	}
}

type appendStream[T any] struct {
	stream      Stream[T]
	appendItems []T
	appendIndex int
	streamDone  bool
	current     T
}

func (s *appendStream[T]) Next() bool {
	// First, consume the original stream
	if !s.streamDone {
		if s.stream.Next() {
			s.current = s.stream.Current()
			return true
		}

		s.streamDone = true
	}

	// Then, consume the appended items
	if s.appendIndex < len(s.appendItems) {
		s.current = s.appendItems[s.appendIndex]
		s.appendIndex++

		return true
	}

	return false
}

func (s *appendStream[T]) Current() T {
	return s.current
}

func (s *appendStream[T]) Err() error {
	return s.stream.Err()
}

func (s *appendStream[T]) Close() error {
	return s.stream.Close()
}
