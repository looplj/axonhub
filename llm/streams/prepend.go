package streams

import "runtime"

func PrependStream[T any](stream Stream[T], items ...T) Stream[T] {
	ps := &prependStream[T]{
		stream:       stream,
		prependItems: items,
		prependIndex: 0,
	}
	runtime.SetFinalizer(ps, func(s *prependStream[T]) {
		if !s.closed {
			_ = s.stream.Close()
		}
	})
	return ps
}

type prependStream[T any] struct {
	stream       Stream[T]
	prependItems []T
	prependIndex int
	current      T
	closed       bool
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

func (s *prependStream[T]) Current() T { return s.current }
func (s *prependStream[T]) Err() error { return s.stream.Err() }
func (s *prependStream[T]) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.stream.Close()
}
