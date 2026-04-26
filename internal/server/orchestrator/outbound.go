package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// OutboundPersistentStream wraps a stream and tracks all responses for final saving to database.
// It implements the streams.Stream interface and handles persistence in the Close method.
//
//nolint:containedctx // Checked.
type OutboundPersistentStream struct {
	ctx context.Context

	RequestService  *biz.RequestService
	UsageLogService *biz.UsageLogService

	stream      streams.Stream[*httpclient.StreamEvent]
	request     *ent.Request
	requestExec *ent.RequestExecution

	transformer    transformer.Outbound
	perf           *biz.PerformanceRecord
	responseChunks []*httpclient.StreamEvent
	closed         bool
	state          *PersistenceState
}

var _ streams.Stream[*httpclient.StreamEvent] = (*OutboundPersistentStream)(nil)

func NewOutboundPersistentStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *biz.RequestService,
	usageLogService *biz.UsageLogService,
	outboundTransformer transformer.Outbound,
	perf *biz.PerformanceRecord,
	state *PersistenceState,
) *OutboundPersistentStream {
	s := &OutboundPersistentStream{
		ctx:             ctx,
		stream:          stream,
		request:         request,
		requestExec:     requestExec,
		RequestService:  requestService,
		UsageLogService: usageLogService,
		transformer:     outboundTransformer,
		perf:            perf,
		responseChunks:  make([]*httpclient.StreamEvent, 0),
		closed:          false,
		state:           state,
	}

	return s
}

func (ts *OutboundPersistentStream) Next() bool {
	return ts.stream.Next()
}

func (ts *OutboundPersistentStream) Current() *httpclient.StreamEvent {
	event := ts.stream.Current()
	if event != nil {
		ts.responseChunks = append(ts.responseChunks, event)
		// Check if this is a terminal event, which indicates the stream completed successfully.
		// For Chat Completions API this is the raw [DONE] event; for Responses API this is
		// response.completed; for Anthropic Messages API this is message_stop.
		if isTerminalStreamEvent(event) {
			ts.state.StreamCompleted = true
		}
	}

	return event
}

func (ts *OutboundPersistentStream) Err() error {
	return ts.stream.Err()
}

func (ts *OutboundPersistentStream) Close() error {
	if ts.closed {
		return nil
	}

	ts.closed = true
	ctx := ts.ctx

	log.Debug(ctx, "Closing persistent stream", log.Int("chunk_count", len(ts.responseChunks)), log.Bool("received_done", ts.state.StreamCompleted))

	streamErr := ts.stream.Err()
	ctxErr := ctx.Err()

	// If we received the [DONE] event, treat the stream as successfully completed
	// even if there's a context cancellation error. This handles the case where
	// the client disconnects immediately after receiving the last chunk.
	if ts.state.StreamCompleted {
		ts.logFinalizationDecision(ctx, "terminal_event_completed", streamErr, ctxErr, true, nil)
		// Stream completed successfully - perform final persistence
		log.Debug(ctx, "Stream completed successfully (received [DONE]), performing final persistence")
		ts.persistResponseChunks(ctx)

		return ts.stream.Close()
	}

	// If there's an explicit stream error (not just context cancellation), treat as failure
	// regardless of what chunks we have. Stream errors indicate the upstream response
	// was incomplete or corrupted.
	if streamErr != nil && !errors.Is(streamErr, context.Canceled) && !errors.Is(streamErr, context.DeadlineExceeded) {
		ts.logFinalizationDecision(ctx, "explicit_stream_error", streamErr, ctxErr, false, nil)
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
		defer cancel()

		if ts.requestExec != nil {
			if err := ts.RequestService.UpdateRequestExecutionStatusFromError(persistCtx, ts.requestExec.ID, streamErr); err != nil {
				log.Warn(persistCtx, "Failed to update request execution status from error", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	var responseBody []byte
	var meta llm.ResponseMeta
	var aggErr error
	aggregatedCompleted := false

	if len(ts.responseChunks) > 0 {
		responseBody, meta, aggErr = ts.transformer.AggregateStreamChunks(context.WithoutCancel(ctx), ts.responseChunks)
		aggregatedCompleted = aggErr == nil && isCompletedAggregated(meta)
		ts.logFinalizationDecision(ctx, "aggregated_outbound_chunks", streamErr, ctxErr, aggregatedCompleted, aggErr)
		if aggregatedCompleted {
			log.Debug(ctx, "Stream has valid complete response without terminal event, treating as completed")
			ts.state.StreamCompleted = true
		}
	} else {
		ts.logFinalizationDecision(ctx, "no_outbound_chunks_to_aggregate", streamErr, ctxErr, false, nil)
	}

	// ended without a terminal event / complete aggregated response.
	if (ctxErr != nil || streamErr != nil) && !ts.state.StreamCompleted {
		ts.logFinalizationDecision(ctx, "incomplete_stream_with_error", streamErr, ctxErr, aggregatedCompleted, aggErr)
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
		defer cancel()

		errToReport := streamErr
		if errToReport == nil {
			errToReport = ctxErr
		}
		if errToReport == nil {
			errToReport = errors.New("stream ended without terminal event or completed response")
		}

		if ts.requestExec != nil {
			if err := ts.RequestService.UpdateRequestExecutionStatusFromError(persistCtx, ts.requestExec.ID, errToReport); err != nil {
				log.Warn(persistCtx, "Failed to update request execution status from error", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	if !ts.state.StreamCompleted {
		ts.logFinalizationDecision(ctx, "incomplete_stream_without_terminal_event", streamErr, ctxErr, aggregatedCompleted, aggErr)
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
		defer cancel()

		errToReport := errors.New("stream ended without terminal event or completed response")
		if ts.requestExec != nil {
			if err := ts.RequestService.UpdateRequestExecutionStatusFromError(persistCtx, ts.requestExec.ID, errToReport); err != nil {
				log.Warn(persistCtx, "Failed to update request execution status from error", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	// Stream completed successfully - perform final persistence
	log.Debug(ctx, "Stream completed successfully, performing final persistence")
	decision := "completed_after_aggregation"
	if len(responseBody) == 0 {
		decision = "completed_via_chunk_persistence"
	}
	ts.logFinalizationDecision(ctx, decision, streamErr, ctxErr, aggregatedCompleted, aggErr)

	if len(responseBody) > 0 {
		ts.persistAggregatedResponse(context.WithoutCancel(ctx), responseBody, meta)
	} else {
		ts.persistResponseChunks(ctx)
	}

	return ts.stream.Close()
}

func (ts *OutboundPersistentStream) logFinalizationDecision(ctx context.Context, decision string, streamErr error, ctxErr error, aggregatedCompleted bool, aggregatedErr error) {
	fields := []log.Field{
		log.String("decision", decision),
		log.Bool("terminal_event_seen", ts.state.StreamCompleted),
		log.Int("chunk_count", len(ts.responseChunks)),
		log.String("api_format", string(ts.transformer.APIFormat())),
		log.Bool("aggregated_completed", aggregatedCompleted),
	}

	if streamErr != nil {
		fields = append(fields, log.String("stream_err", streamErr.Error()))
	}
	if ctxErr != nil {
		fields = append(fields, log.String("ctx_err", ctxErr.Error()))
	}
	if aggregatedErr != nil {
		fields = append(fields, log.String("aggregated_err", aggregatedErr.Error()))
	}

	log.Debug(ctx, "Outbound stream finalization decision", fields...)
}

func (ts *OutboundPersistentStream) persistResponseChunks(ctx context.Context) {
	defer func() {
		if cause := recover(); cause != nil {
			log.Warn(ctx, "Failed to persist outbound response chunks", log.Any("cause", cause))
		}
	}()

	// Update request execution with aggregated chunks
	if ts.requestExec != nil {
		// Use context without cancellation to ensure persistence even if client canceled
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
		defer cancel()

		responseBody, meta, err := ts.transformer.AggregateStreamChunks(persistCtx, ts.responseChunks)
		if err != nil {
			log.Warn(persistCtx, "Failed to aggregate chunks using transformer", log.Cause(err))
			return
		}

		ts.persistAggregatedResponse(persistCtx, responseBody, meta)
	}
}

func (ts *OutboundPersistentStream) persistAggregatedResponse(ctx context.Context, responseBody []byte, meta llm.ResponseMeta) {
	if ts.requestExec == nil {
		return
	}

	// Try to create usage log from aggregated response
	if usage := meta.Usage; usage != nil {
		_, err := ts.UsageLogService.CreateUsageLogFromRequest(ctx, ts.request, ts.requestExec, usage)
		if err != nil {
			log.Warn(ctx, "Failed to create usage log from request", log.Cause(err))
		}
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

	err := ts.RequestService.UpdateRequestExecutionCompleted(
		ctx,
		ts.requestExec.ID,
		meta.ID,
		responseBody,
		metrics,
	)
	if err != nil {
		log.Warn(
			ctx,
			"Failed to update request execution with chunks, trying basic completion",
			log.Cause(err),
		)
	}

	// Save all response chunks at once
	if err := ts.RequestService.SaveRequestExecutionChunks(ctx, ts.requestExec.ID, ts.responseChunks); err != nil {
		log.Warn(ctx, "Failed to save request execution chunks", log.Cause(err))
	}
}

func isCompletedAggregated(meta llm.ResponseMeta) bool {
	return meta.Usage != nil && meta.Usage.CompletionTokens > 0
}

var errSkipCandidateByCircuitBreaker = errors.New("skip candidate by circuit breaker")

// PersistentOutboundTransformer wraps an outbound transformer with shared persistence state.
type PersistentOutboundTransformer struct {
	wrapped transformer.Outbound
	state   *PersistenceState
}

// APIFormat returns the API format of the transformer.
func (p *PersistentOutboundTransformer) APIFormat() llm.APIFormat {
	return p.wrapped.APIFormat()
}

func (p *PersistentOutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return p.wrapped.TransformError(ctx, rawErr)
}

func (p *PersistentOutboundTransformer) TransformRequest(ctx context.Context, llmRequest *llm.Request) (*httpclient.Request, error) {
	// Candidates should already be selected by inbound transformer
	if len(p.state.ChannelModelsCandidates) == 0 {
		return nil, errors.New("no candidates available: candidates should be selected by inbound transformer")
	}

	// Select current candidate for this attempt
	if p.state.CurrentCandidateIndex >= len(p.state.ChannelModelsCandidates) {
		return nil, fmt.Errorf("%w: all candidates exhausted", biz.ErrInternal)
	}

	candidate := p.state.ChannelModelsCandidates[p.state.CurrentCandidateIndex]
	entry := candidate.Models[p.state.CurrentModelIndex]

	p.state.CurrentCandidate = candidate
	p.wrapped = candidate.Channel.Outbound

	log.Debug(ctx, "using candidate",
		log.String("channel", candidate.Channel.Name),
		log.String("request_model", p.state.OriginalModel),
		log.String("actual_model", entry.ActualModel),
	)

	llmRequest.Model = entry.ActualModel

	// Apply channel transform options to create a new request
	llmRequest = applyTransformOptions(llmRequest, candidate.Channel.Settings)
	llmRequest = filterResponseCustomToolMessagesForNonResponsesOutbound(llmRequest, p.wrapped.APIFormat())

	return p.wrapped.TransformRequest(ctx, llmRequest)
}

func filterResponseCustomToolMessagesForNonResponsesOutbound(
	llmRequest *llm.Request,
	outboundFormat llm.APIFormat,
) *llm.Request {
	if llmRequest == nil {
		return nil
	}

	if !isResponsesFormat(llmRequest.APIFormat) || isResponsesFormat(outboundFormat) || !containsResponseCustomToolMessages(llmRequest.Messages) {
		return llmRequest
	}

	cloned := *llmRequest
	cloned.Messages = shared.FilterOutResponseCustomToolMessages(llmRequest.Messages)

	return &cloned
}

func isResponsesFormat(format llm.APIFormat) bool {
	return format == llm.APIFormatOpenAIResponse || format == llm.APIFormatOpenAIResponseCompact
}

func containsResponseCustomToolMessages(messages []llm.Message) bool {
	for _, msg := range messages {
		for _, toolCall := range msg.ToolCalls {
			if toolCall.Type == llm.ToolTypeResponsesCustomTool || toolCall.ResponseCustomToolCall != nil {
				return true
			}
		}
	}

	return false
}

func (p *PersistentOutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return p.wrapped.TransformResponse(ctx, response)
}

func (p *PersistentOutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	persistentStream := NewOutboundPersistentStream(
		ctx,
		stream,
		p.state.Request,
		p.state.RequestExec,
		p.state.RequestService,
		p.state.UsageLogService,
		p.wrapped, // Pass the wrapped outbound transformer for chunk aggregation
		p.state.Perf,
		p.state,
	)

	return p.wrapped.TransformStream(ctx, persistentStream)
}

func (p *PersistentOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return p.wrapped.AggregateStreamChunks(ctx, chunks)
}

// GetRequestExecution returns the current request execution.
func (p *PersistentOutboundTransformer) GetRequestExecution() *ent.RequestExecution {
	return p.state.RequestExec
}

// GetRequest returns the current request.
func (p *PersistentOutboundTransformer) GetRequest() *ent.Request {
	return p.state.Request
}

// GetCurrentChannel returns the current channel.
func (p *PersistentOutboundTransformer) GetCurrentChannel() *biz.Channel {
	if p.state == nil || p.state.CurrentCandidate == nil {
		return nil
	}

	return p.state.CurrentCandidate.Channel
}

// GetCurrentModelID returns the current model ID for logging purposes.
func (p *PersistentOutboundTransformer) GetCurrentModelID() string {
	if p.state.CurrentCandidate == nil || len(p.state.CurrentCandidate.Models) == 0 {
		return ""
	}

	return p.state.CurrentCandidate.Models[p.state.CurrentModelIndex].ActualModel
}

// GetRequestedModel returns the originally requested model ID.
func (p *PersistentOutboundTransformer) GetRequestedModel() string {
	return p.state.OriginalModel
}

// HasMoreChannels returns true if there are more candidates available for retry.
// It implements the pipeline.Retryable interface.
func (p *PersistentOutboundTransformer) HasMoreChannels() bool {
	return p.state.CurrentCandidateIndex+1 < len(p.state.ChannelModelsCandidates)
}

// resetPassThroughStreamState cancels the current attempt's fan-out goroutine (if any)
// and clears pass-through stream state so the next attempt starts with a clean slate.
// Must be called before every retry to prevent goroutine leaks and data races on
// state.RawStreamErrRef.
func (p *PersistentOutboundTransformer) resetPassThroughStreamState() {
	if p.state.RawStreamCancel != nil {
		p.state.RawStreamCancel()
		p.state.RawStreamCancel = nil
	}

	p.state.RawStreamCh = nil
	p.state.RawStreamErrRef = nil
}

// NextChannel moves to the next available candidate for retry.
// It implements the pipeline.Retryable interface.
func (p *PersistentOutboundTransformer) NextChannel(ctx context.Context) error {
	// Cancel any in-flight pass-through stream goroutine from the previous attempt
	// so it exits promptly and releases its upstream HTTP connection.
	p.resetPassThroughStreamState()

	p.state.CurrentCandidateIndex++

	p.state.CurrentModelIndex = 0
	if p.state.CurrentCandidateIndex >= len(p.state.ChannelModelsCandidates) {
		return errors.New("no more candidates available for retry")
	}

	// Reset request execution for the new candidate
	p.state.RequestExec = nil

	candidate := p.state.ChannelModelsCandidates[p.state.CurrentCandidateIndex]
	p.state.CurrentCandidate = candidate
	p.wrapped = candidate.Channel.Outbound

	if log.DebugEnabled(ctx) {
		model := candidate.Models[0].ActualModel
		log.Debug(ctx, "switching to next channel for retry",
			log.String("channel", candidate.Channel.Name),
			log.String("model", model),
			log.Int("index", p.state.CurrentCandidateIndex),
		)
	}

	return nil
}

// CanRetry returns true if the current channel can be retried.
// It implements the pipeline.ChannelRetryable interface, it just check the error is retryable, the
// pipeline will ensure the maxSameChannelRetries is not exceeded.
func (p *PersistentOutboundTransformer) CanRetry(err error) bool {
	if p.state.CurrentCandidate == nil {
		return false
	}

	if errors.Is(err, errSkipCandidateByCircuitBreaker) {
		return false
	}

	// Local queue rejection: same channel is full or timed out — bounce immediately
	// to the next channel rather than retrying.
	if isChannelQueueError(err) {
		return false
	}

	// Empty response detection: allow same-channel retry so the pipeline can
	// re-execute the request against the same (or next model in the) channel.
	if errors.Is(err, pipeline.ErrEmptyResponse) {
		log.Debug(context.Background(), "empty response detected",
			log.Int("channel_id", p.state.CurrentCandidate.Channel.ID),
		)

		return true
	}

	// 429 Too Many Requests: check if Retry-After header is present
	if httpclient.HasRetryAfterHeader(err) {
		// If Retry-After header is present, skip same-channel retry
		// (the channel is explicitly rate-limited by upstream)
		log.Debug(context.Background(), "429 with Retry-After, skipping same-channel retry",
			log.Int("channel_id", p.state.CurrentCandidate.Channel.ID),
		)

		return false
	}

	// 429 without Retry-After header, allow same-channel retry (might be transient rate limit)
	if httpclient.IsRateLimitErr(err) {
		log.Debug(context.Background(), "429 without Retry-After, allowing same-channel retry",
			log.Int("channel_id", p.state.CurrentCandidate.Channel.ID),
		)

		return true
	}

	// if there are more models available in the current candidate, try the next model.
	if p.state.CurrentModelIndex+1 < len(p.state.CurrentCandidate.Models) {
		return true
	}

	// otherwise check if the error is retryable.
	return isRetryableError(err)
}

// PrepareForRetry implements the pipeline.ChannelRetryable interface.
// This will reset the request execution for the same channel, so that the same request can be retried.
// It will try the next model in the same channel if available.
func (p *PersistentOutboundTransformer) PrepareForRetry(ctx context.Context) error {
	candidate := p.state.CurrentCandidate

	// Reset request execution for the same channel.
	p.state.RequestExec = nil

	// Cancel any in-flight pass-through stream goroutine from the previous attempt
	// so it exits promptly and releases its upstream HTTP connection.
	p.resetPassThroughStreamState()

	// If there's another model in the list, advance to it.
	if p.state.CurrentModelIndex+1 < len(candidate.Models) {
		// Increase the model index to the next model.
		p.state.CurrentModelIndex++
		p.wrapped = candidate.Channel.Outbound

		if log.DebugEnabled(ctx) {
			model := candidate.Models[p.state.CurrentModelIndex].ActualModel
			log.Debug(ctx, "prepared same channel retry for next model",
				log.Any("channel", candidate.Channel.Name),
				log.Any("model", model),
				log.Int("current_candidate_index", p.state.CurrentCandidateIndex),
				log.Int("current_entry_index", p.state.CurrentModelIndex),
			)
		}

		return nil
	}

	// Otherwise, we're retrying the current (last) model.
	// It handle the models count less than retry policy.
	if log.DebugEnabled(ctx) {
		model := candidate.Models[p.state.CurrentModelIndex].ActualModel
		log.Debug(ctx, "prepared same channel retry for same model",
			log.Any("channel", candidate.Channel.Name),
			log.Any("model", model),
			log.Int("current_candidate_index", p.state.CurrentCandidateIndex),
			log.Int("current_entry_index", p.state.CurrentModelIndex),
		)
	}

	return nil
}

// CustomizeExecutor customizes the executor for the current channel.
// If the current channel has an executor, it will be used.
// Otherwise, the default executor will be used.
//
// The customized executor will be used to execute the request.
// e.g. the aws bedrock process need a custom executor to handle the request.
// It implements the pipeline.ChannelCustomizedExecutor interface.
func (p *PersistentOutboundTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	// Start with the default executor, then layer customizations.
	customizedExecutor := executor

	channel := p.GetCurrentChannel()
	if channel == nil {
		return customizedExecutor
	}

	// 1. Apply proxy settings. Test proxy override takes precedence over channel settings.
	if p.state.Proxy != nil {
		if channel.HTTPClient != nil {
			customizedExecutor = channel.HTTPClient.WithProxy(p.state.Proxy)
		} else {
			customizedExecutor = httpclient.NewHttpClientWithProxy(p.state.Proxy)
		}
	} else if channel.HTTPClient != nil {
		// Use the channel's own HTTP client, which is pre-configured with its proxy settings.
		customizedExecutor = channel.HTTPClient
	}
	// 2. Allow the specific outbound transformer (e.g., for AWS signing) to further customize the client.
	if custom, ok := channel.Outbound.(pipeline.ChannelCustomizedExecutor); ok {
		return custom.CustomizeExecutor(customizedExecutor)
	}

	return customizedExecutor
}
