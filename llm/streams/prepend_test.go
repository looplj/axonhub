package streams

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrependStream_PrependsBeforeSource(t *testing.T) {
	base := SliceStream([]int{3, 4, 5})
	prepended := PrependStream[int](base, 1, 2)

	var result []int
	for prepended.Next() {
		result = append(result, prepended.Current())
	}

	require.Equal(t, []int{1, 2, 3, 4, 5}, result)
	require.NoError(t, prepended.Err())
	require.NoError(t, prepended.Close())
}

func TestPrependStream_EmptyBase(t *testing.T) {
	base := SliceStream([]int{})
	prepended := PrependStream[int](base, 1, 2)

	var result []int
	for prepended.Next() {
		result = append(result, prepended.Current())
	}

	require.Equal(t, []int{1, 2}, result)
	require.NoError(t, prepended.Err())
	require.NoError(t, prepended.Close())
}

func TestPrependStream_NoPrepends(t *testing.T) {
	base := SliceStream([]int{1, 2})
	prepended := PrependStream[int](base)

	var result []int
	for prepended.Next() {
		result = append(result, prepended.Current())
	}

	require.Equal(t, []int{1, 2}, result)
	require.NoError(t, prepended.Err())
	require.NoError(t, prepended.Close())
}

func TestPrependStream_ErrorInSource(t *testing.T) {
	testErr := errors.New("test error")
	base := &errorStream[int]{
		items: []int{3, 4},
		err:   testErr,
	}
	prepended := PrependStream[int](base, 1, 2)

	var result []int
	for prepended.Next() {
		result = append(result, prepended.Current())
	}

	require.Equal(t, []int{1, 2, 3, 4}, result)
	require.Error(t, prepended.Err())
	require.Equal(t, testErr, prepended.Err())
}

func TestPrependStream_CloseDelegatesToSource(t *testing.T) {
	testErr := errors.New("close error")
	base := &closeTrackingStream[int]{
		items:    []int{3},
		closeErr: testErr,
	}
	prepended := PrependStream[int](base, 1, 2)

	require.NoError(t, prepended.Err())
	require.Equal(t, testErr, prepended.Close())
	require.True(t, base.closed)
}

type closeTrackingStream[T any] struct {
	items    []T
	index    int
	closed   bool
	closeErr error
}

func (s *closeTrackingStream[T]) Next() bool {
	if s.index < len(s.items) {
		s.index++
		return true
	}

	return false
}

func (s *closeTrackingStream[T]) Current() T {
	if s.index > 0 && s.index <= len(s.items) {
		return s.items[s.index-1]
	}

	var zero T

	return zero
}

func (s *closeTrackingStream[T]) Err() error {
	return nil
}

func (s *closeTrackingStream[T]) Close() error {
	s.closed = true
	return s.closeErr
}
