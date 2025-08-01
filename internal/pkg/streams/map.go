package streams

func Map[T, R any](stream Stream[T], mapper func(T) R) Stream[R] {
	return &mapStream[T, R]{
		stream: stream,
		mapper: mapper,
	}
}

type mapStream[T, R any] struct {
	stream Stream[T]
	mapper func(T) R
}

func (s *mapStream[T, R]) Next() bool {
	return s.stream.Next()
}

func (s *mapStream[T, R]) Current() R {
	return s.mapper(s.stream.Current())
}

func (s *mapStream[T, R]) Err() error {
	return s.stream.Err()
}

func (s *mapStream[T, R]) Close() error {
	return s.stream.Close()
}

func MapErr[T, R any](stream Stream[T], mapper func(T) (R, error)) Stream[R] {
	return &mapErrStream[T, R]{
		stream: stream,
		mapper: mapper,
	}
}

type mapErrStream[T, R any] struct {
	stream  Stream[T]
	mapper  func(T) (R, error)
	current R
	err     error
}

func (s *mapErrStream[T, R]) Next() bool {
	if s.stream.Next() {
		cur, err := s.mapper(s.stream.Current())
		if err != nil {
			s.err = err
			return false
		}
		s.current = cur
		return true
	}
	return false
}

func (s *mapErrStream[T, R]) Current() R {
	return s.current
}

func (s *mapErrStream[T, R]) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.stream.Err()
}

func (s *mapErrStream[T, R]) Close() error {
	return s.stream.Close()
}
