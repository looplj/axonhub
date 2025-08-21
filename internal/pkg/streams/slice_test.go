package streams

import (
	"testing"

	"github.com/stretchr/testify/require"
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

			require.Equal(t, tt.expected, result)
			require.NoError(t, stream.Err())
			require.NoError(t, stream.Close())
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
	require.False(t, stream.Next())

	// Current() should return zero value after completion
	require.Equal(t, 0, stream.Current())
}

func TestSliceStream_StringType(t *testing.T) {
	items := []string{"hello", "world", "test"}
	stream := SliceStream(items)

	var result []string

	for stream.Next() {
		result = append(result, stream.Current())
	}

	require.Equal(t, items, result)
	require.NoError(t, stream.Err())
	require.NoError(t, stream.Close())
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

	require.Equal(t, items, result)
	require.NoError(t, stream.Err())
	require.NoError(t, stream.Close())
}
