package biz

import (
	"sync"
	"time"

	"github.com/looplj/axonhub/llm/httpclient"
)

// ChunkBuffer is a thread-safe buffer for accumulating stream chunks.
// It serves as the single source of truth for both live streaming preview
// and final persistence, eliminating duplicate storage.
//
// The buffer supports:
//   - Append: adding new chunks from the streaming goroutine
//   - Slice: reading all chunks for final persistence
type ChunkBuffer struct {
	mu             sync.RWMutex
	chunks         []*httpclient.StreamEvent
	closed         bool // marks buffer as closed (no more appends)
	subscribers    map[chan struct{}]struct{}
	lastAppendedAt time.Time
}

const maxChunkCapacity = 50000

// NewChunkBuffer creates a new ChunkBuffer.
func NewChunkBuffer() *ChunkBuffer {
	return &ChunkBuffer{
		chunks:         make([]*httpclient.StreamEvent, 0),
		subscribers:    make(map[chan struct{}]struct{}),
		lastAppendedAt: time.Now(),
	}
}

// Append adds a chunk to the buffer and invokes the notification callback if set.
// It is safe to call from the streaming goroutine.
// Returns false if the buffer is closed.
func (b *ChunkBuffer) Append(chunk *httpclient.StreamEvent) bool {
	if chunk == nil {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return false
	}

	if len(b.chunks) >= maxChunkCapacity {
		// Reject append to prevent unbounded memory growth and potential OOMs
		return false
	}

	b.chunks = append(b.chunks, chunk)
	b.lastAppendedAt = time.Now()

	b.broadcastLocked()

	return true
}

// Slice returns a copy of all chunks in the buffer.
// This is used for final persistence when the stream closes.
func (b *ChunkBuffer) Slice() []*httpclient.StreamEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return a copy to prevent modification after the fact
	result := make([]*httpclient.StreamEvent, len(b.chunks))
	copy(result, b.chunks)
	return result
}

// Len returns the current number of chunks in the buffer.
func (b *ChunkBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.chunks)
}

// LastAppendedAt returns the timestamp of the last successfully appended chunk.
func (b *ChunkBuffer) LastAppendedAt() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastAppendedAt
}

// Close marks the buffer as closed, preventing further appends.
// This should be called when the stream completes.
func (b *ChunkBuffer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.broadcastLocked()
}

// IsClosed returns true if the buffer is closed.
func (b *ChunkBuffer) IsClosed() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.closed
}

// ChunksPointer returns a pointer to the internal slice.
// WARNING: This is only safe for read-only access by the preview registry
// which uses a length snapshot to safely read existing elements.
// The caller must not modify the returned slice.
func (b *ChunkBuffer) ChunksPointer() *[]*httpclient.StreamEvent {
	return &b.chunks
}

// SnapshotLen returns the current length of the buffer.
// This is used by the preview registry to get a consistent length
// for safe reading of existing elements.
func (b *ChunkBuffer) SnapshotLen() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.chunks)
}

// Subscribe returns a notification channel that receives a signal whenever the
// buffer changes or closes, plus an unsubscribe function.
func (b *ChunkBuffer) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	b.mu.Lock()
	if b.closed {
		select {
		case ch <- struct{}{}:
		default:
		}
		b.mu.Unlock()
		return ch, func() {}
	}
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}
}

func (b *ChunkBuffer) broadcastLocked() {
	for ch := range b.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
