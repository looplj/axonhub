package orchestrator

import (
	"bytes"
	"context"
	"errors"

	"github.com/looplj/axonhub/internal/dumper"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// InboundPersistentStream wraps a stream and tracks all responses for final saving to database.
// It implements the streams.Stream interface and handles persistence in the Close method.
//
//nolint:containedctx // Checked.
type InboundPersistentStream struct {
	ctx            context.Context
	stream         streams.Stream[*httpclient.StreamEvent]
	request        *ent.Request
	requestExec    *ent.RequestExecution
	requestService *biz.RequestService
	transformer    transformer.Inbound
	perf           *biz.PerformanceRecord
	responseChunks []*httpclient.StreamEvent
	chunkBuffer    *biz.ChunkBuffer
	previewKey     string
	closed         bool
	state          *PersistenceState
}

var _ streams.Stream[*httpclient.StreamEvent] = (*InboundPersistentStream)(nil)

func NewInboundPersistentStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *biz.RequestService,
	transformer transformer.Inbound,
	perf *biz.PerformanceRecord,
	state *PersistenceState,
) *InboundPersistentStream {
	s := &InboundPersistentStream{
		ctx:            ctx,
		stream:         stream,
		request:        request,
		requestExec:    requestExec,
		requestService: requestService,
		transformer:    transformer,
		perf:           perf,
		responseChunks: make([]*httpclient.StreamEvent, 0),
		closed:         false,
		state:          state,
	}

	if state.LivePreview && request != nil {
		s.previewKey = biz.RequestKey(request.ID)
		s.chunkBuffer = biz.DefaultStreamPreviewRegistry.GetBuffer(s.previewKey)
		if s.chunkBuffer == nil {
			s.chunkBuffer = biz.NewChunkBuffer()
		}
		biz.DefaultStreamPreviewRegistry.RegisterBuffer(s.previewKey, s.chunkBuffer)
	}

	return s
}

func (ts *InboundPersistentStream) Next() bool {
	return ts.stream.Next()
}

func (ts *InboundPersistentStream) Current() *httpclient.StreamEvent {
	event := ts.stream.Current()
	if event != nil {
		ts.responseChunks = append(ts.responseChunks, event)
		if ts.chunkBuffer != nil {
			ts.chunkBuffer.Append(event)
		}
		if isTerminalStreamEvent(event) {
			ts.state.StreamCompleted = true
		}
	}

	return event
}

// isTerminalStreamEvent checks if the event represents the end of a successfully completed stream.
// For Chat Completions API this is the raw [DONE] event; for Responses API this is response.completed.
func isTerminalStreamEvent(event *httpclient.StreamEvent) bool {
	// For chat completions, check for [DONE] event
	return bytes.Equal(event.Data, llm.DoneStreamEvent.Data) ||
		// For Responses API, check for response.completed event
		event.Type == "response.completed" ||
		// For Anthropic Messages API, check for message_stop event
		event.Type == "message_stop"
}

func (ts *InboundPersistentStream) Err() error {
	return ts.stream.Err()
}

func (ts *InboundPersistentStream) Close() error {
	if ts.closed {
		return nil
	}

	ts.closed = true
	ctx := ts.ctx
	if ts.previewKey != "" {
		defer biz.DefaultStreamPreviewRegistry.Unregister(ts.previewKey)
	}
	if ts.chunkBuffer != nil {
		defer ts.chunkBuffer.Close()
	}

	log.Debug(ctx, "Closing persistent stream", log.Int("chunk_count", len(ts.responseChunks)), log.Bool("received_done", ts.state.StreamCompleted))

	streamErr := ts.stream.Err()
	ctxErr := ctx.Err()

	// If we received the [DONE] event, treat the stream as successfully completed
	// even if there's a context cancellation error. This handles the case where
	// the client disconnects immediately after receiving the last chunk.
	if ts.state.StreamCompleted {
		// Stream completed successfully - perform final persistence
		log.Debug(ctx, "Stream completed successfully (received terminal event), performing final persistence")
		ts.persistResponseChunks(ctx)

		return ts.stream.Close()
	}

	// If there's an explicit stream error (not just context cancellation), treat as failure
	// regardless of what chunks we have. Stream errors indicate the upstream response
	// was incomplete or corrupted.
	if streamErr != nil && !errors.Is(streamErr, context.Canceled) && !errors.Is(streamErr, context.DeadlineExceeded) {
		if ts.request != nil {
			persistCtx := context.WithoutCancel(ctx)

			if err := ts.requestService.UpdateRequestStatusFromError(persistCtx, ts.request.ID, streamErr); err != nil {
				log.Warn(persistCtx, "Failed to update request status from error", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	// If we haven't received a terminal event, check if the chunks we DO have form a complete response.
	// This handles models that aggregate internally (like Codex) or upstream proxy hung connections
	// where the provider sent the full JSON payload but failed to send [DONE] before dropping.
	var responseBody []byte
	var meta llm.ResponseMeta
	var aggErr error

	if len(ts.responseChunks) > 0 && !ts.state.StreamCompleted {
		responseBody, meta, aggErr = ts.transformer.AggregateStreamChunks(context.WithoutCancel(ctx), ts.responseChunks)
		if aggErr == nil && meta.ID != "" && len(responseBody) > 0 {
			log.Debug(ctx, "Stream has valid complete response without terminal event, treating as completed")
			ts.state.StreamCompleted = true
		}
	}

	// Check if context was canceled (client disconnected before [DONE]).
	// Skip the error path if we determined the stream actually completed successfully above.
	if (ctxErr != nil || streamErr != nil) && !ts.state.StreamCompleted {
		if ts.request != nil {
			persistCtx := context.WithoutCancel(ctx)

			errToReport := ctxErr
			if errToReport == nil {
				errToReport = streamErr
			}

			if err := ts.requestService.UpdateRequestStatusFromError(persistCtx, ts.request.ID, errToReport); err != nil {
				log.Warn(persistCtx, "Failed to update request status from error", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	// Stream completed successfully - perform final persistence
	log.Debug(ctx, "Stream completed successfully, performing final persistence")

	// We already aggregated the chunks above, so pass them directly to avoid double-aggregation
	if len(responseBody) > 0 {
		ts._persistResponse(context.WithoutCancel(ctx), responseBody, meta)
	} else {
		ts.persistResponseChunks(ctx)
	}

	return ts.stream.Close()
}

func (ts *InboundPersistentStream) persistResponseChunks(ctx context.Context) {
	defer func() {
		if cause := recover(); cause != nil {
			log.Warn(ctx, "Failed to persist inbound response chunks", log.Any("cause", cause))
		}
	}()

	// Use context without cancellation to ensure persistence even if client canceled
	persistCtx := context.WithoutCancel(ctx)

	// Aggregate stream chunks first, then delegate to _persistResponse
	responseBody, meta, err := ts.transformer.AggregateStreamChunks(persistCtx, ts.responseChunks)
	if err != nil {
		log.Warn(persistCtx, "Failed to aggregate chunks for main request", log.Cause(err))
		dumper.DumpStreamEvents(persistCtx, ts.responseChunks, "response_chunks.json")
	}

	ts._persistResponse(persistCtx, responseBody, meta)
}

// _persistResponse performs the actual persistence with pre-aggregated data.
// This avoids redundant aggregation when the data is already available.
func (ts *InboundPersistentStream) _persistResponse(ctx context.Context, responseBody []byte, meta llm.ResponseMeta) {
	if ts.request == nil {
		return
	}

	// Build latency metrics from performance record
	var metrics *biz.LatencyMetrics

	if ts.perf != nil {
		firstTokenLatencyMs, requestLatencyMs, _ := ts.perf.Calculate()

		metrics = &biz.LatencyMetrics{
			LatencyMs: &requestLatencyMs,
		}
		if ts.perf.Stream && ts.perf.FirstTokenTime != nil {
			metrics.FirstTokenLatencyMs = &firstTokenLatencyMs
		}
	}

	err := ts.requestService.UpdateRequestCompleted(ctx, ts.request.ID, meta.ID, responseBody, metrics)
	if err != nil {
		log.Warn(ctx, "Failed to update request status to completed", log.Cause(err))
	}

	// Save all response chunks at once
	if err := ts.requestService.SaveRequestChunks(ctx, ts.request.ID, ts.responseChunks); err != nil {
		log.Warn(ctx, "Failed to save request chunks", log.Cause(err))
	}
}

// PersistentInboundTransformer wraps an inbound transformer with enhanced capabilities.
type PersistentInboundTransformer struct {
	wrapped transformer.Inbound
	state   *PersistenceState
}

func (p *PersistentInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	return p.wrapped.TransformError(ctx, rawErr)
}

func (p *PersistentInboundTransformer) TransformRequest(ctx context.Context, request *httpclient.Request) (*llm.Request, error) {
	llmRequest, err := p.wrapped.TransformRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	llmRequest.RawRequest = request
	p.state.RawRequest = request
	p.state.LlmRequest = llmRequest

	return llmRequest, nil
}

func (p *PersistentInboundTransformer) TransformResponse(ctx context.Context, response *llm.Response) (*httpclient.Response, error) {
	return p.wrapped.TransformResponse(ctx, response)
}

func (p *PersistentInboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	channelStream, err := p.wrapped.TransformStream(ctx, stream)
	if err != nil {
		return nil, err
	}

	persistentStream := NewInboundPersistentStream(
		ctx,
		channelStream,
		p.state.Request,
		p.state.RequestExec,
		p.state.RequestService,
		p, // Use the PersistentInboundTransformer as the transformer
		p.state.Perf,
		p.state,
	)

	return persistentStream, nil
}

func (p *PersistentInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return p.wrapped.AggregateStreamChunks(ctx, chunks)
}
