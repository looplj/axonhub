package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/log"

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
func (r *StreamPreviewRegistry) RegisterBuffer(key string, buffer *ChunkBuffer) {
	entry := &previewEntry{buffer: buffer}
	r.entries.Store(key, entry)
}

// Register registers the stream's chunk slice for preview access.
// Deprecated: Use RegisterBuffer instead. This method is kept for backward compatibility.
// Called when the persistent stream is created.
func (r *StreamPreviewRegistry) Register(key string, chunks *[]*httpclient.StreamEvent) {
	r.entries.Store(key, &previewEntry{
		buffer: &ChunkBuffer{chunks: *chunks},
	})
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

// StartSweeper starts a background worker that periodically cleans up stale chunk buffers.
func (r *StreamPreviewRegistry) StartSweeper(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.sweepStaleEntries(ctx)
			}
		}
	}()
}

// sweepStaleEntries iterates over the registry and removes closed or long-idle buffers.
func (r *StreamPreviewRegistry) sweepStaleEntries(ctx context.Context) {
	threshold := 10 * time.Minute
	now := time.Now()
	evictedCount := 0

	r.entries.Range(func(key, value any) bool {
		entry, ok := value.(*previewEntry)
		if !ok || entry.buffer == nil {
			r.entries.Delete(key)
			return true
		}

		buffer := entry.buffer

		if buffer.IsClosed() {
			r.entries.Delete(key)
			evictedCount++
			return true
		}

		if now.Sub(buffer.LastAppendedAt()) > threshold {
			log.Warn(ctx, "Preview registry sweeper force-closing an idle zombie stream buffer", log.Any("key", key), log.Duration("idle_time", now.Sub(buffer.LastAppendedAt())))
			buffer.Close()        // Detach any dangling subscribers
			r.entries.Delete(key) // Evict from registry
			evictedCount++
		}
		return true
	})

	if evictedCount > 0 {
		log.Debug(ctx, "Preview registry swept stale entries", log.Int("evicted", evictedCount))
	}
}
