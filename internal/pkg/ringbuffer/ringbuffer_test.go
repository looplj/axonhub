package ringbuffer

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		expected int
	}{
		{
			name:     "valid capacity",
			capacity: 10,
			expected: 10,
		},
		{
			name:     "zero capacity should default to 1",
			capacity: 0,
			expected: 1,
		},
		{
			name:     "negative capacity should default to 1",
			capacity: -5,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := New[int](tt.capacity)
			require.NotNil(t, rb)
			require.Equal(t, tt.expected, rb.Capacity())
			require.Equal(t, 0, rb.Len())
		})
	}
}

func TestRingBuffer_Push(t *testing.T) {
	t.Run("push to empty buffer", func(t *testing.T) {
		rb := New[string](5)
		rb.Push(100, "first")
		require.Equal(t, 1, rb.Len())

		val, ok := rb.Get(100)
		require.True(t, ok)
		require.Equal(t, "first", val)
	})

	t.Run("push multiple items", func(t *testing.T) {
		rb := New[string](5)
		rb.Push(100, "first")
		rb.Push(200, "second")
		rb.Push(300, "third")
		require.Equal(t, 3, rb.Len())

		val, ok := rb.Get(100)
		require.True(t, ok)
		require.Equal(t, "first", val)

		val, ok = rb.Get(200)
		require.True(t, ok)
		require.Equal(t, "second", val)

		val, ok = rb.Get(300)
		require.True(t, ok)
		require.Equal(t, "third", val)
	})

	t.Run("push to full buffer removes oldest", func(t *testing.T) {
		rb := New[string](3)
		rb.Push(100, "first")
		rb.Push(200, "second")
		rb.Push(300, "third")
		require.Equal(t, 3, rb.Len())

		// This should remove "first"
		rb.Push(400, "fourth")
		require.Equal(t, 3, rb.Len())

		_, ok := rb.Get(100)
		require.False(t, ok)

		val, ok := rb.Get(400)
		require.True(t, ok)
		require.Equal(t, "fourth", val)
	})

	t.Run("push with same timestamp updates value", func(t *testing.T) {
		rb := New[string](5)
		rb.Push(100, "first")
		rb.Push(100, "updated")
		require.Equal(t, 1, rb.Len())

		val, ok := rb.Get(100)
		require.True(t, ok)
		require.Equal(t, "updated", val)
	})
}

func TestRingBuffer_Get(t *testing.T) {
	rb := New[int](5)
	rb.Push(100, 10)
	rb.Push(200, 20)
	rb.Push(300, 30)

	tests := []struct {
		name      string
		timestamp int64
		wantValue int
		wantOk    bool
	}{
		{
			name:      "get existing item",
			timestamp: 100,
			wantValue: 10,
			wantOk:    true,
		},
		{
			name:      "get middle item",
			timestamp: 200,
			wantValue: 20,
			wantOk:    true,
		},
		{
			name:      "get last item",
			timestamp: 300,
			wantValue: 30,
			wantOk:    true,
		},
		{
			name:      "get non-existent item",
			timestamp: 400,
			wantValue: 0,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := rb.Get(tt.timestamp)
			require.Equal(t, tt.wantOk, ok)
			require.Equal(t, tt.wantValue, val)
		})
	}
}

func TestRingBuffer_CleanupBefore(t *testing.T) {
	t.Run("cleanup oldest items", func(t *testing.T) {
		rb := New[string](10)
		for i := int64(0); i < 10; i++ {
			rb.Push(i*100, "value")
		}

		require.Equal(t, 10, rb.Len())

		// Remove items with timestamp < 500
		removed := rb.CleanupBefore(500)
		require.Equal(t, 5, removed)
		require.Equal(t, 5, rb.Len())

		// Verify the correct items remain
		_, ok := rb.Get(400)
		require.False(t, ok)

		_, ok = rb.Get(500)
		require.True(t, ok)
	})

	t.Run("cleanup all items", func(t *testing.T) {
		rb := New[string](5)
		rb.Push(100, "first")
		rb.Push(200, "second")
		rb.Push(300, "third")

		removed := rb.CleanupBefore(1000)
		require.Equal(t, 3, removed)
		require.Equal(t, 0, rb.Len())
	})

	t.Run("cleanup with no matching items", func(t *testing.T) {
		rb := New[string](5)
		rb.Push(100, "first")
		rb.Push(200, "second")

		removed := rb.CleanupBefore(50)
		require.Equal(t, 0, removed)
		require.Equal(t, 2, rb.Len())
	})

	t.Run("cleanup empty buffer", func(t *testing.T) {
		rb := New[string](5)
		removed := rb.CleanupBefore(100)
		require.Equal(t, 0, removed)
		require.Equal(t, 0, rb.Len())
	})
}

func TestRingBuffer_Range(t *testing.T) {
	t.Run("range over all items", func(t *testing.T) {
		rb := New[int](5)
		rb.Push(100, 10)
		rb.Push(200, 20)
		rb.Push(300, 30)

		var (
			timestamps []int64
			values     []int
		)

		rb.Range(func(timestamp int64, value int) bool {
			timestamps = append(timestamps, timestamp)
			values = append(values, value)

			return true
		})

		require.Equal(t, []int64{100, 200, 300}, timestamps)
		require.Equal(t, []int{10, 20, 30}, values)
	})

	t.Run("range with early termination", func(t *testing.T) {
		rb := New[int](5)
		rb.Push(100, 10)
		rb.Push(200, 20)
		rb.Push(300, 30)

		var count int
		rb.Range(func(timestamp int64, value int) bool {
			count++
			return count < 2 // Stop after 2 items
		})

		require.Equal(t, 2, count)
	})

	t.Run("range over empty buffer", func(t *testing.T) {
		rb := New[int](5)
		called := false

		rb.Range(func(timestamp int64, value int) bool {
			called = true
			return true
		})
		require.False(t, called)
	})

	t.Run("range after wraparound", func(t *testing.T) {
		rb := New[int](3)
		rb.Push(100, 10)
		rb.Push(200, 20)
		rb.Push(300, 30)
		rb.Push(400, 40) // This causes wraparound

		var timestamps []int64
		rb.Range(func(timestamp int64, value int) bool {
			timestamps = append(timestamps, timestamp)
			return true
		})

		require.Equal(t, []int64{200, 300, 400}, timestamps)
	})
}

func TestRingBuffer_Len(t *testing.T) {
	rb := New[string](5)
	require.Equal(t, 0, rb.Len())

	rb.Push(100, "first")
	require.Equal(t, 1, rb.Len())

	rb.Push(200, "second")
	require.Equal(t, 2, rb.Len())

	rb.CleanupBefore(150)
	require.Equal(t, 1, rb.Len())

	rb.Clear()
	require.Equal(t, 0, rb.Len())
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := New[string](5)
	rb.Push(100, "first")
	rb.Push(200, "second")
	rb.Push(300, "third")
	require.Equal(t, 3, rb.Len())

	rb.Clear()
	require.Equal(t, 0, rb.Len())

	_, ok := rb.Get(100)
	require.False(t, ok)
}

func TestRingBuffer_GetAll(t *testing.T) {
	t.Run("get all from populated buffer", func(t *testing.T) {
		rb := New[int](5)
		rb.Push(100, 10)
		rb.Push(200, 20)
		rb.Push(300, 30)

		items := rb.GetAll()
		require.Len(t, items, 3)
		require.Equal(t, int64(100), items[0].Timestamp)
		require.Equal(t, 10, items[0].Value)
		require.Equal(t, int64(200), items[1].Timestamp)
		require.Equal(t, 20, items[1].Value)
		require.Equal(t, int64(300), items[2].Timestamp)
		require.Equal(t, 30, items[2].Value)
	})

	t.Run("get all from empty buffer", func(t *testing.T) {
		rb := New[int](5)
		items := rb.GetAll()
		require.Nil(t, items)
	})

	t.Run("get all after wraparound", func(t *testing.T) {
		rb := New[int](3)
		rb.Push(100, 10)
		rb.Push(200, 20)
		rb.Push(300, 30)
		rb.Push(400, 40)

		items := rb.GetAll()
		require.Len(t, items, 3)
		require.Equal(t, int64(200), items[0].Timestamp)
		require.Equal(t, int64(300), items[1].Timestamp)
		require.Equal(t, int64(400), items[2].Timestamp)
	})
}

func TestRingBuffer_Concurrent(t *testing.T) {
	t.Run("concurrent push", func(t *testing.T) {
		rb := New[int](100)

		var wg sync.WaitGroup

		// Spawn 10 goroutines, each pushing 10 items
		for i := 0; i < 10; i++ {
			wg.Add(1)

			go func(base int) {
				defer wg.Done()

				for j := 0; j < 10; j++ {
					timestamp := int64(base*10 + j)
					rb.Push(timestamp, base*10+j)
				}
			}(i)
		}

		wg.Wait()
		// We should have 100 items
		require.Equal(t, 100, rb.Len())
	})

	t.Run("concurrent push and get", func(t *testing.T) {
		rb := New[int](50)

		var wg sync.WaitGroup

		// Push items
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := 0; i < 50; i++ {
				rb.Push(int64(i), i*10)
			}
		}()

		// Read items
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := 0; i < 50; i++ {
				rb.Get(int64(i))
			}
		}()

		wg.Wait()
	})

	t.Run("concurrent cleanup and range", func(t *testing.T) {
		rb := New[int](100)

		// Populate buffer
		for i := 0; i < 100; i++ {
			rb.Push(int64(i), i)
		}

		var wg sync.WaitGroup

		// Cleanup old items
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := 0; i < 10; i++ {
				rb.CleanupBefore(int64(i * 10))
			}
		}()

		// Range over items
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := 0; i < 10; i++ {
				rb.Range(func(timestamp int64, value int) bool {
					return true
				})
			}
		}()

		wg.Wait()
	})
}

func TestRingBuffer_ComplexScenario(t *testing.T) {
	t.Run("sliding window simulation", func(t *testing.T) {
		// Simulate a 10-second sliding window with 1-second slots
		rb := New[int](10)

		// Add data for timestamps 0-9
		for i := int64(0); i < 10; i++ {
			rb.Push(i, int(i*100))
		}

		require.Equal(t, 10, rb.Len())

		// Advance time to 15, cleanup old data (< 5)
		rb.CleanupBefore(5)
		require.Equal(t, 5, rb.Len())

		// Add more data
		for i := int64(10); i < 15; i++ {
			rb.Push(i, int(i*100))
		}

		require.Equal(t, 10, rb.Len())

		// Verify only recent data exists
		_, ok := rb.Get(4)
		require.False(t, ok)

		val, ok := rb.Get(5)
		require.True(t, ok)
		require.Equal(t, 500, val)

		val, ok = rb.Get(14)
		require.True(t, ok)
		require.Equal(t, 1400, val)
	})

	t.Run("metrics aggregation pattern", func(t *testing.T) {
		type Metric struct {
			Count int64
			Sum   int64
		}

		rb := New[*Metric](60) // 60-second window

		// Simulate recording metrics
		for i := int64(0); i < 60; i++ {
			rb.Push(i, &Metric{Count: 1, Sum: i})
		}

		// Calculate aggregated metrics
		var totalCount, totalSum int64
		rb.Range(func(timestamp int64, value *Metric) bool {
			totalCount += value.Count
			totalSum += value.Sum

			return true
		})

		require.Equal(t, int64(60), totalCount)
		require.Equal(t, int64(1770), totalSum) // Sum of 0 to 59
	})
}

func BenchmarkRingBuffer_Push(b *testing.B) {
	rb := New[int](1000)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rb.Push(int64(i), i)
	}
}

func BenchmarkRingBuffer_Get(b *testing.B) {
	rb := New[int](1000)
	for i := 0; i < 1000; i++ {
		rb.Push(int64(i), i)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rb.Get(int64(i % 1000))
	}
}

func BenchmarkRingBuffer_CleanupBefore(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		rb := New[int](1000)
		for j := 0; j < 1000; j++ {
			rb.Push(int64(j), j)
		}

		b.StartTimer()

		rb.CleanupBefore(500)
	}
}

func BenchmarkRingBuffer_Range(b *testing.B) {
	rb := New[int](1000)
	for i := 0; i < 1000; i++ {
		rb.Push(int64(i), i)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rb.Range(func(timestamp int64, value int) bool {
			return true
		})
	}
}
