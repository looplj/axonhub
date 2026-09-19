package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/dumper"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/log"
	appmetrics "github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// ErrStreamIncomplete reports an upstream stream that ended without a terminal
// event and without an aggregated complete response. The same value is persisted
// on the request and delivered to the client, so both sides agree on the reason.
var ErrStreamIncomplete = errors.New("stream ended without terminal event or completed response")

type streamTerminalState string

const (
	streamTerminalNone       streamTerminalState = ""
	streamTerminalCompleted  streamTerminalState = "completed"
	streamTerminalFailed     streamTerminalState = "failed"
	streamTerminalIncomplete streamTerminalState = "incomplete"
	streamTerminalCanceled   streamTerminalState = "canceled"
)

// InboundPersistentStream wraps a stream and tracks all responses for final saving to database.
// It implements the streams.Stream interface and handles persistence in the Close method.
//
//nolint:containedctx // Checked.
type InboundPersistentStream struct {
	ctx                    context.Context
	stream                 streams.Stream[*httpclient.StreamEvent]
	request                *ent.Request
	requestExec            *ent.RequestExecution
	requestService         *biz.RequestService
	transformer            transformer.Inbound
	perf                   *biz.PerformanceRecord
	responseChunks         []*httpclient.StreamEvent
	closed                 bool
	downstreamTTFTRecorded bool
	terminalState          streamTerminalState
	state                  *PersistenceState
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

	return s
}

func (ts *InboundPersistentStream) Next() bool {
	return ts.stream.Next()
}

func (ts *InboundPersistentStream) Current() *httpclient.StreamEvent {
	event := ts.stream.Current()
	if event != nil {
		if !ts.downstreamTTFTRecorded && ts.request != nil && hasClientVisibleOutput(event) {
			attrs := appmetrics.RequestAttributes{
				ProjectID: ts.request.ProjectID, RequestModelID: ts.request.ModelID,
				APIKeyID: ts.request.APIKeyID, Source: string(ts.request.Source),
				Format: ts.request.Format, Stream: ts.request.Stream,
			}
			ttft := time.Since(ts.request.CreatedAt).Seconds()
			appmetrics.Metrics.RecordDownstreamPerformance(ts.ctx, attrs, "streaming", -1, &ttft)
			ts.downstreamTTFTRecorded = true
		}
		// For raw binary audio chunks, persist only a size summary.
		ts.responseChunks = append(ts.responseChunks, httpclient.SummarizeBinaryChunk(event))
		if ts.terminalState == streamTerminalNone {
			ts.terminalState = classifyStreamTerminalEvent(event)
			if ts.terminalState != streamTerminalNone {
				ts.terminalState = ts.finalTerminalState()
				ts.state.StreamCompleted = ts.terminalState == streamTerminalCompleted
			}
		}
	}

	return event
}

// hasClientVisibleOutput recognizes output in the downstream wire protocol.
// Current runs before the HTTP write: this measures output ready for delivery,
// not an acknowledgement from the client. A final chunk can also carry output.
func hasClientVisibleOutput(event *httpclient.StreamEvent) bool {
	if event == nil || len(event.Data) == 0 || bytes.Equal(event.Data, llm.DoneStreamEvent.Data) {
		return false
	}

	if strings.HasPrefix(event.Type, "audio/") || event.Type == "application/octet-stream" {
		return true
	}
	data := gjson.ParseBytes(event.Data)
	eventType := event.Type
	if eventType == "" {
		eventType = data.Get("type").String()
	}
	switch eventType {
	case "content_block_delta":
		return hasOutputString(data.Get("delta"), "text", "thinking", "partial_json")
	case "content_block_start":
		block := data.Get("content_block")
		return hasOutputString(block, "text", "thinking") ||
			(block.Get("type").String() == "tool_use" && hasOutputString(block, "name"))
	case "response.output_text.delta", "response.reasoning_text.delta", "response.reasoning_summary_text.delta",
		"response.function_call_arguments.delta", "response.refusal.delta", "response.audio.delta",
		"response.output_audio.delta", "transcript.text.delta":
		return hasOutputString(data, "delta")
	case "speech.audio.delta":
		return hasOutputString(data, "audio")
	case "response.output_item.added":
		item := data.Get("item")
		return item.Get("type").String() == "function_call" && hasOutputString(item, "name")
	case "text-delta", "reasoning-delta": // AI SDK data stream
		return hasOutputString(data, "delta")
	}

	for _, choice := range data.Get("choices").Array() {
		delta := choice.Get("delta")
		if hasOutputString(choice, "text") || hasOutputString(delta, "content", "reasoning_content", "reasoning", "refusal", "audio.data") {
			return true
		}
		for _, call := range delta.Get("tool_calls").Array() {
			if hasOutputString(call, "function.name", "function.arguments") {
				return true
			}
		}
	}
	for _, candidate := range data.Get("candidates").Array() {
		for _, part := range candidate.Get("content.parts").Array() {
			if hasOutputString(part, "text", "functionCall.name", "inlineData.data") {
				return true
			}
		}
	}
	return false
}

func hasOutputString(data gjson.Result, paths ...string) bool {
	for _, path := range paths {
		value := data.Get(path)
		if value.Type == gjson.String && value.String() != "" {
			return true
		}
	}
	return false
}

// IsTerminalStreamEvent checks both SSE metadata and JSON data for a
// protocol-level terminal marker. The SSE writers use it to decide whether the
// client already received the stream outcome, so this must stay the single source
// of truth for "the stream ended properly".
func IsTerminalStreamEvent(event *httpclient.StreamEvent) bool {
	if event == nil {
		return false
	}

	// For chat completions, check for [DONE] event
	if bytes.Equal(event.Data, llm.DoneStreamEvent.Data) ||
		// For Responses API, check for all terminal response events
		isResponsesTerminalEvent(event.Type) ||
		// For Anthropic Messages API, check for message_stop event
		event.Type == "message_stop" ||
		// For OpenAI audio APIs (TTS sse / STT stream) which have no [DONE] sentinel:
		// rely on the terminal *.done event surfaced as StreamEvent.Type.
		event.Type == "speech.audio.done" ||
		event.Type == "transcript.text.done" ||
		event.Type == httpclient.BinaryStreamDoneEventType {
		return true
	}

	// Compatible SSE providers do not always populate the SSE `event` field and
	// instead carry the event type only in the JSON data. Also recognize a chat
	// completion's finish_reason as semantic completion: clients commonly close
	// the connection immediately after consuming that final useful chunk, before
	// the trailing [DONE] marker is read by the server.
	eventType := gjson.GetBytes(event.Data, "type").String()
	switch eventType {
	case "message_stop", "speech.audio.done", "transcript.text.done":
		return true
	}
	if isResponsesTerminalEvent(eventType) {
		return true
	}

	// OpenAI chat completions: choices[].finish_reason
	if hasNonEmptyJSONStringField(event.Data, "choices", "finish_reason") {
		return true
	}

	// Gemini generateContent streams have no [DONE] sentinel. Completion is
	// signaled by candidates[].finishReason (e.g. STOP, MAX_TOKENS, SAFETY).
	return hasNonEmptyJSONStringField(event.Data, "candidates", "finishReason")
}

func isResponsesTerminalEvent(eventType string) bool {
	switch eventType {
	case "response.completed", "response.failed", "response.incomplete", "response.cancelled", "error":
		return true
	default:
		return false
	}
}

func classifyStreamTerminalEvent(event *httpclient.StreamEvent) streamTerminalState {
	if !IsTerminalStreamEvent(event) {
		return streamTerminalNone
	}
	const responseStatusCanceledBritish = "cancelled" //nolint:misspell // OpenAI protocol spelling.

	eventType := event.Type
	jsonEventType := gjson.GetBytes(event.Data, "type").String()
	responseStatus := gjson.GetBytes(event.Data, "response.status").String()

	// Standalone errors can arrive before response.created and have no response status.
	if eventType == "error" || jsonEventType == "error" {
		return streamTerminalFailed
	}

	if eventType == "response.cancelled" || jsonEventType == "response.cancelled" ||
		responseStatus == responseStatusCanceledBritish || responseStatus == "canceled" {
		return streamTerminalCanceled
	}
	if eventType == "response.failed" || jsonEventType == "response.failed" || responseStatus == "failed" {
		return streamTerminalFailed
	}
	if eventType == "response.incomplete" || jsonEventType == "response.incomplete" || responseStatus == "incomplete" {
		return streamTerminalIncomplete
	}

	return streamTerminalCompleted
}

// streamTerminalErrorMessage is called for a non-successful terminal event.
func streamTerminalErrorMessage(event *httpclient.StreamEvent, state streamTerminalState) string {
	for _, path := range []string{
		"response.error.message",
		"error.message",
		"response.incomplete_details.reason",
	} {
		if message := gjson.GetBytes(event.Data, path).String(); message != "" {
			return message
		}
	}

	// Compatible providers may use response.completed for abnormal outcomes.
	// Report the classified outcome rather than a misleading event type.
	return string(state)
}

func hasNonEmptyJSONStringField(data []byte, arrayPath, field string) bool {
	arr := gjson.GetBytes(data, arrayPath)
	if !arr.IsArray() {
		return false
	}

	completed := false
	arr.ForEach(func(_, item gjson.Result) bool {
		value := item.Get(field)
		completed = value.Type == gjson.String && value.String() != ""

		return !completed
	})

	return completed
}

func (ts *InboundPersistentStream) Err() error {
	return ts.stream.Err()
}

func (ts *InboundPersistentStream) finalTerminalState() streamTerminalState {
	// Generic finish_reason/message_stop events can lose the provider's failure
	// or cancellation. Preserve that outcome even if the client disconnects
	// before the converted terminal event is consumed. A successful provider
	// outcome must not hide a downstream transformation failure.
	switch ts.state.OutboundStreamTerminal {
	case streamTerminalFailed, streamTerminalIncomplete, streamTerminalCanceled:
		return ts.state.OutboundStreamTerminal
	default:
		return ts.terminalState
	}
}

func (ts *InboundPersistentStream) Close() error {
	if ts.closed {
		return nil
	}

	ts.closed = true
	ctx := ts.ctx

	log.Debug(ctx, "Closing persistent stream", log.Int("chunk_count", len(ts.responseChunks)), log.Bool("received_done", ts.state.StreamCompleted))

	streamErr := ts.stream.Err()
	ctxErr := ctx.Err()
	ts.terminalState = ts.finalTerminalState()

	// A terminal event carries the final stream outcome. Persist its structured
	// response even if a transport or context error arrives afterward.
	if ts.terminalState != streamTerminalNone {
		log.Debug(ctx, "Stream terminal event received, performing final persistence",
			log.String("terminal_state", string(ts.terminalState)))
		ts.persistResponseChunks(ctx)

		return ts.stream.Close()
	}

	// If there's an explicit stream error (not just context cancellation), treat as failure
	// regardless of what chunks we have. Stream errors indicate the upstream response
	// was incomplete or corrupted.
	if streamErr != nil && !errors.Is(streamErr, context.Canceled) && !errors.Is(streamErr, context.DeadlineExceeded) {
		persistCtx := context.WithoutCancel(ctx)
		ts.persistFailureChunks(persistCtx)

		if ts.request != nil {
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
		if aggErr == nil && meta.ID != "" && len(responseBody) > 0 && isCompletedAggregated(meta) {
			log.Debug(ctx, "Stream has valid complete response without terminal event, treating as completed")
			ts.state.StreamCompleted = true
		}
	}

	// Check if context was canceled (client disconnected before [DONE]).
	// Skip the error path if we determined the stream actually completed successfully above.
	if (ctxErr != nil || streamErr != nil) && !ts.state.StreamCompleted {
		persistCtx := context.WithoutCancel(ctx)
		ts.persistFailureChunks(persistCtx)

		if ts.request != nil {
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

	// If the stream ended without a terminal event and we couldn't determine it was
	// completed through aggregation, mark it as incomplete/failed. This handles the case
	// where the upstream connection drops silently (EOF) without sending a terminal event,
	// which would otherwise fall through and incorrectly mark the request as "completed".
	if !ts.state.StreamCompleted {
		log.Debug(ctx, "Stream ended without terminal event or completed response, treating as incomplete")

		persistCtx := context.WithoutCancel(ctx)
		// Persist partial chunks for debugging; do not mark the request completed.
		ts.persistFailureChunks(persistCtx)

		if ts.request != nil {
			errToReport := ErrStreamIncomplete

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

// persistFailureChunks stores buffered SSE chunks for a failed/incomplete stream
// without marking the request completed. Used so truncated upstream responses remain
// inspectable when store_chunks is enabled.
func (ts *InboundPersistentStream) persistFailureChunks(ctx context.Context) {
	if ts.request == nil || len(ts.responseChunks) == 0 {
		return
	}

	if err := ts.requestService.SaveRequestChunks(ctx, ts.request.ID, ts.responseChunks); err != nil {
		log.Warn(ctx, "Failed to save request chunks after stream failure", log.Cause(err))
	}
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

	status := ts.terminalState.requestStatus()
	err := ts.requestService.UpdateRequestFinalized(ctx, ts.request.ID, status, meta.ID, responseBody, metrics)
	if err != nil {
		log.Warn(ctx, "Failed to update finalized request", log.Cause(err), log.Any("status", status))
	}

	// Save all response chunks at once
	if err := ts.requestService.SaveRequestChunks(ctx, ts.request.ID, ts.responseChunks); err != nil {
		log.Warn(ctx, "Failed to save request chunks", log.Cause(err))
	}
}

func (s streamTerminalState) requestStatus() request.Status {
	switch s {
	case streamTerminalFailed, streamTerminalIncomplete:
		return request.StatusFailed
	case streamTerminalCanceled:
		return request.StatusCanceled
	default:
		return request.StatusCompleted
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
	p.state.OriginalRequestStream = llmRequest.Stream

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
