package streams

// Stream represents a generic stream interface
type Stream[T any] interface {
	Next() bool
	Current() T
	Err() error
	Close() error
}

// StreamResponse wraps streaming response with error handling
type StreamResponse[T any] struct {
	ch     <-chan T
	errCh  <-chan error
	closed bool
}

// NewStreamResponse creates a new stream response
func NewStreamResponse[T any](ch <-chan T, errCh <-chan error) Stream[T] {
	return &StreamResponse[T]{
		ch:    ch,
		errCh: errCh,
	}
}

// Next checks if there's a next item
func (s *StreamResponse[T]) Next() bool {
	if s.closed {
		return false
	}
	select {
	case _, ok := <-s.ch:
		if !ok {
			s.closed = true
			return false
		}
		return true
	case <-s.errCh:
		s.closed = true
		return false
	default:
		return true
	}
}

// Current returns the current item
func (s *StreamResponse[T]) Current() T {
	select {
	case item := <-s.ch:
		return item
	default:
		var zero T
		return zero
	}
}

// Err returns any error that occurred
func (s *StreamResponse[T]) Err() error {
	select {
	case err := <-s.errCh:
		return err
	default:
		return nil
	}
}

// Close closes the stream
func (s *StreamResponse[T]) Close() error {
	s.closed = true
	return nil
}
