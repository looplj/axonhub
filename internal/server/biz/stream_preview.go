package biz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// StreamPreviewRegistry provides read access to in-flight stream chunks
// without duplicating data. It holds references to ChunkBuffer instances
// owned by InboundPersistentStream / OutboundPersistentStream.
//
// Key format: "request:{id}" for request-level chunks,
// "execution:{id}" for execution-level chunks.
type StreamPreviewRegistry struct {
	entries sync.Map // map[string]*previewEntry
}

type previewEntry struct {
	buffer *ChunkBuffer // reference to the stream's chunk buffer
}

// NewStreamPreviewRegistry creates a new StreamPreviewRegistry.
func NewStreamPreviewRegistry() *StreamPreviewRegistry {
	return &StreamPreviewRegistry{}
}

// DefaultStreamPreviewRegistry is the package-level global registry.
var DefaultStreamPreviewRegistry = NewStreamPreviewRegistry()

// RequestKey returns the registry key for a request.
func RequestKey(requestID int) string {
	return fmt.Sprintf("request:%d", requestID)
}

// ExecutionKey returns the registry key for a request execution.
func ExecutionKey(executionID int) string {
	return fmt.Sprintf("execution:%d", executionID)
}

// RegisterBuffer registers a ChunkBuffer for preview access.
// Called when the persistent stream is created.
// Returns a notification function that should be called after each append.
func (r *StreamPreviewRegistry) RegisterBuffer(key string, buffer *ChunkBuffer) func() {
	entry := &previewEntry{buffer: buffer}
	r.entries.Store(key, entry)

	// Return a notification function that updates the buffer's length snapshot
	return func() {
		r.NotifyAppend(key)
	}
}

// Register registers the stream's chunk slice for preview access.
// Deprecated: Use RegisterBuffer instead. This method is kept for backward compatibility.
// Called when the persistent stream is created.
func (r *StreamPreviewRegistry) Register(key string, chunks *[]*httpclient.StreamEvent) {
	r.entries.Store(key, &previewEntry{
		buffer: &ChunkBuffer{chunks: *chunks},
	})
}

// NotifyAppend updates the length snapshot after a new chunk is appended.
// This is O(1) — just reads the current buffer length.
// Called from Current() in the streaming goroutine.
func (r *StreamPreviewRegistry) NotifyAppend(key string) {
	v, ok := r.entries.Load(key)
	if !ok {
		return
	}

	entry, ok := v.(*previewEntry)
	if !ok {
		return
	}

	// The ChunkBuffer handles its own synchronization
	// This method is kept for backward compatibility
	_ = entry
}
// GetChunks returns the current live chunks as JSON in the same format
// as SaveRequestChunks (jsonStreamEvent marshaling). Returns nil if no
// entry is registered for the key.
func (r *StreamPreviewRegistry) GetChunks(key string) []objects.JSONRawMessage {
	v, ok := r.entries.Load(key)
	if !ok {
		return nil
	}

	entry, ok := v.(*previewEntry)
	if !ok {
		return nil
	}

	buffer := entry.buffer
	if buffer == nil {
		return nil
	}

	// Get a snapshot of the current chunks
	chunks := buffer.Slice()
	if len(chunks) == 0 {
		return nil
	}

	// Read up to the snapshot length — safe because the streaming goroutine
	// only appends beyond this index, and existing elements are never moved.
	var result []objects.JSONRawMessage
	for _, chunk := range chunks {
		// Skip terminal DONE events
		if bytes.Equal(chunk.Data, llm.DoneStreamEvent.Data) {
			continue
		}

		b, err := xjson.Marshal(struct {
			LastEventID string          `json:"last_event_id,omitempty"`
			Type        string          `json:"event"`
			Data        json.RawMessage `json:"data"`
		}{
			LastEventID: chunk.LastEventID,
			Type:        chunk.Type,
			Data:        chunk.Data,
		})
		if err != nil {
			continue
		}

		result = append(result, b)
	}

	return result
}

// Unregister removes the entry for the given key.
// Called from Close() in the streaming goroutine after persistence.
func (r *StreamPreviewRegistry) Unregister(key string) {
	r.entries.Delete(key)
}

// GetBuffer returns the ChunkBuffer for the given key, or nil if not found.
func (r *StreamPreviewRegistry) GetBuffer(key string) *ChunkBuffer {
	v, ok := r.entries.Load(key)
	if !ok {
		return nil
	}

	entry, ok := v.(*previewEntry)
	if !ok {
		return nil
	}

	return entry.buffer
}
