package ringbuffer

import (
	"sync"
)

// Item represents a single item in the ring buffer with a timestamp.
type Item[T any] struct {
	Timestamp int64
	Value     T
}

// RingBuffer is a fixed-size circular buffer that stores items ordered by timestamp.
// It automatically removes old items when the buffer is full or when cleanup is triggered.
// Uses an internal map index for O(1) Get operations.
type RingBuffer[T any] struct {
	mu       sync.RWMutex
	items    []Item[T]
	index    map[int64]int // timestamp -> array index mapping for O(1) lookup
	capacity int
	size     int
	head     int // points to the oldest item
	tail     int // points to the next insertion position
}

// New creates a new RingBuffer with the specified capacity.
func New[T any](capacity int) *RingBuffer[T] {
	if capacity <= 0 {
		capacity = 1
	}

	return &RingBuffer[T]{
		items:    make([]Item[T], capacity),
		index:    make(map[int64]int, capacity),
		capacity: capacity,
		size:     0,
		head:     0,
		tail:     0,
	}
}

// Push adds a new item to the ring buffer.
// If an item with the same timestamp exists, it updates the value.
// If the buffer is full, the oldest item is removed.
func (rb *RingBuffer[T]) Push(timestamp int64, value T) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Check if we should update an existing item with the same timestamp
	if idx, exists := rb.index[timestamp]; exists {
		rb.items[idx].Value = value
		return
	}

	// If buffer is full, remove the oldest item from index
	if rb.size >= rb.capacity {
		oldTimestamp := rb.items[rb.head].Timestamp
		delete(rb.index, oldTimestamp)
	}

	// Add new item
	rb.items[rb.tail] = Item[T]{
		Timestamp: timestamp,
		Value:     value,
	}
	rb.index[timestamp] = rb.tail

	rb.tail = (rb.tail + 1) % rb.capacity

	if rb.size < rb.capacity {
		rb.size++
	} else {
		// Buffer is full, move head forward
		rb.head = (rb.head + 1) % rb.capacity
	}
}

// Get retrieves an item by timestamp in O(1) time.
// Returns the value and true if found, zero value and false otherwise.
func (rb *RingBuffer[T]) Get(timestamp int64) (T, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if idx, exists := rb.index[timestamp]; exists {
		return rb.items[idx].Value, true
	}

	var zero T

	return zero, false
}

// CleanupBefore removes all items with timestamps before the cutoff.
// This is an O(k) operation where k is the number of items to remove.
func (rb *RingBuffer[T]) CleanupBefore(cutoff int64) int {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	removed := 0

	for rb.size > 0 {
		if rb.items[rb.head].Timestamp >= cutoff {
			break
		}

		// Remove from index
		oldTimestamp := rb.items[rb.head].Timestamp
		delete(rb.index, oldTimestamp)

		// Move head forward
		rb.head = (rb.head + 1) % rb.capacity
		rb.size--
		removed++
	}

	return removed
}

// Range iterates over all items in the buffer, calling fn for each item.
// If fn returns false, iteration stops.
// Items are visited in order from oldest to newest.
func (rb *RingBuffer[T]) Range(fn func(timestamp int64, value T) bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	for i := range rb.size {
		idx := (rb.head + i) % rb.capacity

		item := rb.items[idx]
		if !fn(item.Timestamp, item.Value) {
			break
		}
	}
}

// Len returns the current number of items in the buffer.
func (rb *RingBuffer[T]) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	return rb.size
}

// Capacity returns the maximum capacity of the buffer.
func (rb *RingBuffer[T]) Capacity() int {
	return rb.capacity
}

// Clear removes all items from the buffer.
func (rb *RingBuffer[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.size = 0
	rb.head = 0
	rb.tail = 0
	rb.index = make(map[int64]int, rb.capacity)
}

// GetAll returns a slice of all items currently in the buffer.
// Items are returned in order from oldest to newest.
func (rb *RingBuffer[T]) GetAll() []Item[T] {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size == 0 {
		return nil
	}

	result := make([]Item[T], 0, rb.size)
	for i := range rb.size {
		idx := (rb.head + i) % rb.capacity
		result = append(result, rb.items[idx])
	}

	return result
}
