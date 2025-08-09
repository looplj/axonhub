package streams

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSliceStream(t *testing.T) {
	tests := []struct {
		name     string
		items    []int
		expected []int
	}{
		{
			name:     "empty slice",
			items:    []int{},
			expected: []int{},
		},
		{
			name:     "single item",
			items:    []int{1},
			expected: []int{1},
		},
		{
			name:     "multiple items",
			items:    []int{1, 2, 3, 4, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := SliceStream(tt.items)
			result := make([]int, 0)

			for stream.Next() {
				result = append(result, stream.Current())
			}

			assert.Equal(t, tt.expected, result)
			assert.NoError(t, stream.Err())
			assert.NoError(t, stream.Close())
		})
	}
}

func TestSliceStream_EmptyAfterCompletion(t *testing.T) {
	stream := SliceStream([]int{1, 2, 3})

	// Consume all items
	for stream.Next() {
		stream.Current()
	}

	// Should return false for Next() after completion
	assert.False(t, stream.Next())

	// Current() should return zero value after completion
	assert.Equal(t, 0, stream.Current())
}

func TestSliceStream_StringType(t *testing.T) {
	items := []string{"hello", "world", "test"}
	stream := SliceStream(items)

	var result []string

	for stream.Next() {
		result = append(result, stream.Current())
	}

	assert.Equal(t, items, result)
	assert.NoError(t, stream.Err())
	assert.NoError(t, stream.Close())
}

func TestSliceStream_CustomStruct(t *testing.T) {
	type testStruct struct {
		ID   int
		Name string
	}

	items := []testStruct{
		{ID: 1, Name: "first"},
		{ID: 2, Name: "second"},
	}

	stream := SliceStream(items)

	var result []testStruct

	for stream.Next() {
		result = append(result, stream.Current())
	}

	assert.Equal(t, items, result)
	assert.NoError(t, stream.Err())
	assert.NoError(t, stream.Close())
}
