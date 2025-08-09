package streams

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap_IntToString(t *testing.T) {
	stream := SliceStream([]int{1, 2, 3})
	mapped := Map[int, string](stream, func(i int) string { return "n=" + string(rune('0'+i)) })

	var result []string
	for mapped.Next() {
		result = append(result, mapped.Current())
	}

	assert.Equal(t, []string{"n=1", "n=2", "n=3"}, result)
	assert.NoError(t, mapped.Err())
	assert.NoError(t, mapped.Close())
}

func TestMapErr_Success(t *testing.T) {
	stream := SliceStream([]int{1, 2, 3})
	mapped := MapErr[int, int](stream, func(i int) (int, error) { return i * 2, nil })

	var result []int
	for mapped.Next() {
		result = append(result, mapped.Current())
	}

	assert.Equal(t, []int{2, 4, 6}, result)
	assert.NoError(t, mapped.Err())
	assert.NoError(t, mapped.Close())
}

func TestMapErr_WithErrorStopsIteration(t *testing.T) {
	stream := SliceStream([]int{1, 2, 3})
	errTest := errors.New("boom")
	mapped := MapErr[int, int](stream, func(i int) (int, error) {
		if i == 2 {
			return 0, errTest
		}

		return i * 10, nil
	})

	var result []int
	for mapped.Next() {
		result = append(result, mapped.Current())
	}

	assert.Equal(t, []int{10}, result)
	assert.Equal(t, errTest, mapped.Err())
	assert.NoError(t, mapped.Close())
}

func TestMap_PropagatesSourceErrorAndClose(t *testing.T) {
	// Create a custom stream that simulates an error after the first item
	s := &faultyStream[int]{items: []int{1, 2}, err: errors.New("source error")}
	mapped := Map[int, int](s, func(i int) int { return i * 2 })

	var result []int
	for mapped.Next() {
		result = append(result, mapped.Current())
	}
	// Once source stops early due to error, Next will be false and Err should be the source error
	assert.Equal(t, []int{2}, result)
	assert.Equal(t, s.err, mapped.Err())
	assert.NoError(t, mapped.Close())
}

// faultyStream is a minimal Stream implementation for testing error propagation
// It emits the first item, then on the next call to Next() it returns false and sets an error.
// Close() returns nil to simplify tests

type faultyStream[T any] struct {
	items   []T
	idx     int
	err     error
	errored bool
}

func (f *faultyStream[T]) Next() bool {
	if f.errored {
		return false
	}
	// Emit first item only
	if f.idx == 0 && len(f.items) > 0 {
		f.idx = 1
		return true
	}
	// Simulate error on attempting to advance further
	f.errored = true

	return false
}

func (f *faultyStream[T]) Current() T {
	return f.items[f.idx-1]
}

func (f *faultyStream[T]) Err() error {
	if f.errored {
		return f.err
	}

	return nil
}

func (f *faultyStream[T]) Close() error { return nil }
