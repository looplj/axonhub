package streams

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendStream_AppendsAfterSource(t *testing.T) {
	base := SliceStream([]int{1, 2, 3})
	appended := AppendStream[int](base, 4, 5)

	var result []int
	for appended.Next() {
		result = append(result, appended.Current())
	}

	assert.Equal(t, []int{1, 2, 3, 4, 5}, result)
	assert.NoError(t, appended.Err())
	assert.NoError(t, appended.Close())
}

func TestAppendStream_EmptyBase(t *testing.T) {
	base := SliceStream([]int{})
	appended := AppendStream[int](base, 1, 2)

	var result []int
	for appended.Next() {
		result = append(result, appended.Current())
	}

	assert.Equal(t, []int{1, 2}, result)
	assert.NoError(t, appended.Err())
	assert.NoError(t, appended.Close())
}

func TestAppendStream_NoAppends(t *testing.T) {
	base := SliceStream([]int{1, 2})
	appended := AppendStream[int](base)

	var result []int
	for appended.Next() {
		result = append(result, appended.Current())
	}

	assert.Equal(t, []int{1, 2}, result)
	assert.NoError(t, appended.Err())
	assert.NoError(t, appended.Close())
}
