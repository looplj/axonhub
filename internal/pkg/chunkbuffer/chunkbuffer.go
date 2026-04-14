package chunkbuffer

import (
	"sync"
	"time"

	"github.com/looplj/axonhub/llm/httpclient"
)

// Buffer is a thread-safe buffer for accumulating stream chunks.
// It serves as the single source of truth for both live streaming preview
// and final persistence, eliminating duplicate storage.
//
// The buffer supports:
//   - Append: adding new chunks from the streaming goroutine
//   - Slice: reading all chunks for final persistence
type Buffer struct {
	mu             sync.RWMutex
	chunks         []*httpclient.StreamEvent
	closed         bool
	subscribers    map[chan struct{}]struct{}
	lastAppendedAt time.Time
}

const maxChunkCapacity = 50000

// New creates a new Buffer.
func New() *Buffer {
	return &Buffer{
		chunks:         make([]*httpclient.StreamEvent, 0),
		subscribers:    make(map[chan struct{}]struct{}),
		lastAppendedAt: time.Now(),
	}
}

// Append adds a chunk to the buffer.
// It is safe to call from the streaming goroutine.
// Returns false if the buffer is closed.
func (b *Buffer) Append(chunk *httpclient.StreamEvent) bool {
	if chunk == nil {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return false
	}

	if len(b.chunks) >= maxChunkCapacity {
		// Reject append to prevent unbounded memory growth and potential OOMs.
		return false
	}

	b.chunks = append(b.chunks, chunk)
	b.lastAppendedAt = time.Now()
	b.broadcastLocked()

	return true
}

// Slice returns a copy of all chunks in the buffer.
func (b *Buffer) Slice() []*httpclient.StreamEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*httpclient.StreamEvent, len(b.chunks))
	copy(result, b.chunks)
	return result
}

// Len returns the current number of chunks in the buffer.
func (b *Buffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.chunks)
}

// At returns the chunk at index when present.
func (b *Buffer) At(index int) (*httpclient.StreamEvent, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if index < 0 || index >= len(b.chunks) {
		return nil, false
	}

	return b.chunks[index], true
}

// Read returns the chunk at index when present, along with the next index and
// whether the buffer was closed at the same instant the read was performed.
func (b *Buffer) Read(index int) (*httpclient.StreamEvent, int, bool, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if index < 0 || index >= len(b.chunks) {
		return nil, index, b.closed, false
	}

	return b.chunks[index], index + 1, b.closed, true
}

// LastAppendedAt returns the timestamp of the last successfully appended chunk.
func (b *Buffer) LastAppendedAt() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastAppendedAt
}

// Close marks the buffer as closed, preventing further appends.
func (b *Buffer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.broadcastLocked()
}

// IsClosed returns true if the buffer is closed.
func (b *Buffer) IsClosed() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.closed
}

// SubscribeFromCurrent registers a subscriber and atomically returns the
// current buffer length as the replay cutoff for that subscriber.
func (b *Buffer) SubscribeFromCurrent() (<-chan struct{}, int, func()) {
	ch := make(chan struct{}, 1)

	b.mu.Lock()

	replayUntil := len(b.chunks)
	if b.closed {
		select {
		case ch <- struct{}{}:
		default:
		}
		b.mu.Unlock()

		return ch, replayUntil, func() {}
	}
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	return ch, replayUntil, func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}
}

func (b *Buffer) broadcastLocked() {
	for ch := range b.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
