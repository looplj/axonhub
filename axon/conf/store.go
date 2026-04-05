package conf

import (
	"sync/atomic"
)

type Store[T any] struct {
	version atomic.Uint64
	value   atomic.Pointer[T]
}

func NewStore[T any](initial T) *Store[T] {
	s := &Store[T]{}
	s.value.Store(&initial)
	s.version.Store(1)
	return s
}

func (s *Store[T]) Get() (T, uint64) {
	v := s.value.Load()
	return *v, s.version.Load()
}

func (s *Store[T]) Set(next T) uint64 {
	s.value.Store(&next)
	return s.version.Add(1)
}
