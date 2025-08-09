package streams

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilter_Basic(t *testing.T) {
	stream := SliceStream([]int{1, 2, 3, 4, 5, 6})
	filtered := Filter[int](stream, func(i int) bool { return i%2 == 0 })

	var result []int
	for filtered.Next() {
		result = append(result, filtered.Current())
	}

	assert.Equal(t, []int{2, 4, 6}, result)
	assert.NoError(t, filtered.Err())
	assert.NoError(t, filtered.Close())
}

func TestFilter_AllFilteredOut(t *testing.T) {
	stream := SliceStream([]int{1, 3, 5})
	filtered := Filter[int](stream, func(i int) bool { return i%2 == 0 })

	var result []int
	for filtered.Next() {
		result = append(result, filtered.Current())
	}

	assert.Empty(t, result)
	assert.NoError(t, filtered.Err())
	assert.NoError(t, filtered.Close())
}

func TestFilter_NoneFilteredOut(t *testing.T) {
	stream := SliceStream([]int{2, 4, 6})
	filtered := Filter[int](stream, func(i int) bool { return i%2 == 0 })

	var result []int
	for filtered.Next() {
		result = append(result, filtered.Current())
	}

	assert.Equal(t, []int{2, 4, 6}, result)
	assert.NoError(t, filtered.Err())
	assert.NoError(t, filtered.Close())
}
