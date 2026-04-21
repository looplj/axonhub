package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/chunkbuffer"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm"
)

// LiveStreamRegistry provides read access to in-flight stream chunks
// without duplicating data. It holds references to chunkbuffer.Buffer instances
// owned by InboundPersistentStream / OutboundPersistentStream.
type LiveStreamRegistry struct {
	requests   sync.Map // map[int]*chunkbuffer.Buffer
	executions sync.Map // map[int]*chunkbuffer.Buffer
}

// NewLiveStreamRegistry creates a new LiveStreamRegistry.
func NewLiveStreamRegistry() *LiveStreamRegistry {
	return &LiveStreamRegistry{}
}

func (r *LiveStreamRegistry) RegisterRequest(requestID int, buffer *chunkbuffer.Buffer) {
	r.requests.Store(requestID, buffer)
}

func (r *LiveStreamRegistry) RegisterExecution(executionID int, buffer *chunkbuffer.Buffer) {
	r.executions.Store(executionID, buffer)
}

func (r *LiveStreamRegistry) UnregisterRequest(requestID int) {
	r.requests.Delete(requestID)
}

func (r *LiveStreamRegistry) UnregisterExecution(executionID int) {
	r.executions.Delete(executionID)
}

func (r *LiveStreamRegistry) GetRequestBuffer(requestID int) *chunkbuffer.Buffer {
	v, ok := r.requests.Load(requestID)
	if !ok {
		return nil
	}

	buffer, ok := v.(*chunkbuffer.Buffer)
	if !ok {
		return nil
	}

	return buffer
}

func (r *LiveStreamRegistry) GetExecutionBuffer(executionID int) *chunkbuffer.Buffer {
	v, ok := r.executions.Load(executionID)
	if !ok {
		return nil
	}

	buffer, ok := v.(*chunkbuffer.Buffer)
	if !ok {
		return nil
	}

	return buffer
}

// GetRequestChunks returns the current live request chunks as JSON in the same format
// as SaveRequestChunks.
func (r *LiveStreamRegistry) GetRequestChunks(requestID int) []objects.JSONRawMessage {
	return marshalPreviewChunks(r.GetRequestBuffer(requestID))
}

// GetExecutionChunks returns the current live request execution chunks as JSON in the same format
// as SaveRequestExecutionChunks.
func (r *LiveStreamRegistry) GetExecutionChunks(executionID int) []objects.JSONRawMessage {
	return marshalPreviewChunks(r.GetExecutionBuffer(executionID))
}

func marshalPreviewChunks(buffer *chunkbuffer.Buffer) []objects.JSONRawMessage {
	if buffer == nil {
		return nil
	}

	// Get a snapshot of the current chunks
	chunks := buffer.Slice()
	if len(chunks) == 0 {
		return nil
	}

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

// StartSweeper starts a background worker that periodically cleans up stale chunk buffers.
func (r *LiveStreamRegistry) StartSweeper(ctx context.Context) {
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
func (r *LiveStreamRegistry) sweepStaleEntries(ctx context.Context) {
	threshold := 10 * time.Minute
	now := time.Now()
	evictedCount := 0

	sweepMap := func(scope string, entries *sync.Map) {
		entries.Range(func(key, value any) bool {
			buffer, ok := value.(*chunkbuffer.Buffer)
			if !ok || buffer == nil {
				entries.Delete(key)
				return true
			}

			if buffer.IsClosed() {
				entries.Delete(key)

				evictedCount++

				return true
			}

			if now.Sub(buffer.LastAppendedAt()) > threshold {
				log.Warn(ctx, "Preview registry sweeper force-closing an idle zombie stream buffer",
					log.String("scope", scope),
					log.Any("id", key),
					log.Duration("idle_time", now.Sub(buffer.LastAppendedAt())),
				)
				buffer.Close()
				entries.Delete(key)

				evictedCount++
			}
			return true
		})
	}

	sweepMap("request", &r.requests)
	sweepMap("execution", &r.executions)

	if evictedCount > 0 {
		log.Debug(ctx, "Preview registry swept stale entries", log.Int("evicted", evictedCount))
	}
}
