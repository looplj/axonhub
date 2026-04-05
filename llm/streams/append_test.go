package streams

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendStream_AppendsAfterSource(t *testing.T) {
	base := SliceStream([]int{1, 2, 3})
	appended := AppendStream[int](base, 4, 5)

	var result []int
	for appended.Next() {
		result = append(result, appended.Current())
	}

	require.Equal(t, []int{1, 2, 3, 4, 5}, result)
	require.NoError(t, appended.Err())
	require.NoError(t, appended.Close())
}

func TestAppendStream_EmptyBase(t *testing.T) {
	base := SliceStream([]int{})
	appended := AppendStream[int](base, 1, 2)

	var result []int
	for appended.Next() {
		result = append(result, appended.Current())
	}

	require.Equal(t, []int{1, 2}, result)
	require.NoError(t, appended.Err())
	require.NoError(t, appended.Close())
}

func TestAppendStream_NoAppends(t *testing.T) {
	base := SliceStream([]int{1, 2})
	appended := AppendStream[int](base)

	var result []int
	for appended.Next() {
		result = append(result, appended.Current())
	}

	require.Equal(t, []int{1, 2}, result)
	require.NoError(t, appended.Err())
	require.NoError(t, appended.Close())
}

func TestAppendStream_ErrorInSource(t *testing.T) {
	testErr := errors.New("test error")
	// Create a stream that will error
	base := &errorStream[int]{
		items: []int{1, 2},
		err:   testErr,
	}
	appended := AppendStream[int](base, 3, 4)

	var result []int
	for appended.Next() {
		result = append(result, appended.Current())
	}

	// Should only get items from base, not appended items
	require.Equal(t, []int{1, 2}, result)
	require.Error(t, appended.Err())
	require.Equal(t, testErr, appended.Err())
}

// errorStream is a test helper that returns an error after yielding all items.
type errorStream[T any] struct {
	items []T
	index int
	err   error
}

func (s *errorStream[T]) Next() bool {
	if s.index < len(s.items) {
		s.index++
		return true
	}

	return false
}

func (s *errorStream[T]) Current() T {
	if s.index > 0 && s.index <= len(s.items) {
		return s.items[s.index-1]
	}

	var zero T

	return zero
}

func (s *errorStream[T]) Err() error {
	if s.index >= len(s.items) {
		return s.err
	}

	return nil
}

func (s *errorStream[T]) Close() error {
	return nil
}
