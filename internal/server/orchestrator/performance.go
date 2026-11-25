package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

const errorMatchBodyLimit = 8 * 1024

// withPerformanceRecording creates a unified middleware that handles all performance tracking.
// It initializes metrics, tracks first token in streams, and records final metrics.
func withPerformanceRecording(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return &performanceRecording{
		outbound: outbound,
	}
}

// performanceRecording is a unified middleware that handles all performance tracking.
type performanceRecording struct {
	pipeline.DummyMiddleware

	outbound *PersistentOutboundTransformer
}

func (m *performanceRecording) Name() string {
	return "record-performance"
}

func (m *performanceRecording) OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	if m.outbound.state.Perf == nil {
		m.outbound.state.Perf = &biz.PerformanceRecord{}
	}

	if request.Stream != nil {
		m.outbound.state.Perf.Stream = *request.Stream
	} else {
		m.outbound.state.Perf.Stream = false
	}

	return request, nil
}

func (m *performanceRecording) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	// Initialize performance metrics at the start of request
	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return request, nil
	}

	// Preserve Stream flag from existing PerformanceRecord (set in OnInboundLlmRequest)
	var streamFlag bool
	if m.outbound.state.Perf != nil {
		streamFlag = m.outbound.state.Perf.Stream
	}

	// Create a new PerformanceRecord instance for each request.
	perf := biz.PerformanceRecord{}
	perf.StartTime = time.Now()
	perf.ChannelID = channel.ID
	perf.Success = false
	perf.RequestCompleted = false
	perf.Stream = streamFlag

	// Get the API key used for this request from context (set by TraceStickyKeyProvider).
	// OAuth channels authenticate from Credentials.OAuth and never pass through the
	// key provider, so they carry no context key. Identify them by the fixed OAuth
	// credential ref instead, which lets auto-disable and scheduled recovery treat an
	// OAuth channel as an ordinary one-credential channel.
	if apiKey, ok := contexts.GetChannelAPIKey(ctx); ok {
		perf.APIKey = apiKey
	} else if channel.Credentials.IsOAuth() {
		perf.APIKey = objects.OAuthCredentialRef
	}

	m.outbound.state.Perf = &perf
	m.outbound.state.performanceAttempt = &performanceAttempt{
		record:  &perf,
		service: m.outbound.state.ChannelService,
	}

	log.Debug(ctx, "Started performance tracking",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
	)

	return request, nil
}

func (m *performanceRecording) OnOutboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	return response, nil
}

func (m *performanceRecording) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	if m.outbound.state.Perf == nil {
		return response, nil
	}

	if response != nil && response.Usage != nil {
		if tokenCount := response.Usage.GetCompletionTokens(); tokenCount != nil && *tokenCount > 0 {
			m.outbound.state.Perf.CompletionTokens = *tokenCount
		}
	}

	// Preserve the response latency for persistence. The final attempt result
	// can still reject this response (for example, during empty-response detection).
	m.outbound.state.Perf.MarkSuccess()

	return response, nil
}

func (m *performanceRecording) OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	return stream, nil
}

func (m *performanceRecording) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return &recordPerformanceStream{
		stream:  stream,
		perf:    m.outbound.state.Perf,
		attempt: m.outbound.state.performanceAttempt,
	}, nil
}

func (m *performanceRecording) OnOutboundRawError(_ context.Context, err error) {
	if attempt := m.outbound.state.performanceAttempt; attempt != nil {
		attempt.rawError = err
	}
}

func (m *performanceRecording) OnOutboundAttemptResult(ctx context.Context, result *pipeline.Result, err error) {
	attempt := m.outbound.state.performanceAttempt
	m.outbound.state.performanceAttempt = nil
	if attempt == nil {
		return
	}
	if err == nil && result != nil && result.Stream {
		attempt.acceptStream(ctx)
		return
	}
	attempt.finish(ctx, err)
}

// A stream can be closed before the pipeline decides whether it will be
// returned, rejected or aggregated. Only its final attempt result may commit
// the record to channel health.
type performanceAttempt struct {
	record      *biz.PerformanceRecord
	service     *biz.ChannelService
	rawError    error
	streamError error

	mu             sync.Mutex
	streamAccepted bool
	streamClosed   bool
	finished       bool
}

func (a *performanceAttempt) acceptStream(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.streamAccepted = true
	if a.streamClosed {
		a.submit(ctx)
	}
}

func (a *performanceAttempt) closeStream(ctx context.Context, decision streamOutcomeDecision, providerTerminal bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.finished {
		return
	}
	a.streamClosed = true
	switch {
	case decision.state == streamTerminalCompleted || (providerTerminal && decision.state == streamTerminalIncomplete):
		if !a.record.RequestCompleted || !a.record.Success {
			a.record.MarkSuccess()
		}
	case decision.state == streamTerminalCanceled:
		a.record.MarkCanceled()
	default:
		if decision.cause == nil {
			decision.cause = ErrStreamIncomplete
		}
		detail := decision.cause
		if ExtractStatusCodeFromError(a.streamError) > 0 {
			detail = a.streamError
		}
		a.record.MarkFailedWithMessage(ExtractErrorCode(detail), extractErrorMessageForMatching(detail))
	}
	if a.streamAccepted {
		a.submit(ctx)
	}
}

func (a *performanceAttempt) finish(ctx context.Context, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.finished {
		return
	}
	if isLocalAdmissionError(err) || shared.IsResponsesRecoveryError(err) {
		a.finished = true
		return
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			a.record.MarkCanceled()
		} else {
			detail := err
			if ExtractStatusCodeFromError(a.rawError) > 0 {
				detail = a.rawError
			}
			a.record.MarkFailedWithMessage(ExtractErrorCode(detail), extractErrorMessageForMatching(detail))
		}
	} else if !a.record.RequestCompleted {
		a.record.MarkSuccess()
	}
	a.submit(ctx)
}

// submit is called with mu held. AsyncRecordPerformance takes a copy of the
// finished record so that the metrics worker never reads live request state.
func (a *performanceAttempt) submit(ctx context.Context) {
	if a.finished {
		return
	}
	a.finished = true
	if a.service != nil {
		a.service.AsyncRecordPerformance(ctx, a.record)
	}
}

// recordPerformanceStream records performance metrics for a stream of responses.
type recordPerformanceStream struct {
	stream  streams.Stream[*llm.Response]
	perf    *biz.PerformanceRecord
	attempt *performanceAttempt

	firstTokenSet     bool
	reasoningStartSet bool
	reasoningEndSet   bool
}

func (s *recordPerformanceStream) Current() *llm.Response {
	event := s.stream.Current()
	if event == nil {
		return event
	}

	if !s.firstTokenSet && s.perf != nil && hasMeaningfulStreamOutput(event) {
		s.perf.MarkFirstToken()
		s.firstTokenSet = true
	}

	if s.perf != nil && len(event.Choices) > 0 {
		delta := event.Choices[0].Delta
		if delta != nil {
			if delta.ReasoningContent != nil && *delta.ReasoningContent != "" {
				if !s.reasoningStartSet {
					s.perf.MarkReasoningStart()
					s.reasoningStartSet = true
				}
			} else if (delta.Content.Content != nil && *delta.Content.Content != "") || len(delta.Content.MultipleContent) > 0 || len(delta.ToolCalls) > 0 {
				if s.reasoningStartSet && !s.reasoningEndSet {
					s.perf.MarkReasoningEnd()
					s.reasoningEndSet = true
				}
			}
		}
	}

	if tokenCount := event.Usage.GetCompletionTokens(); s.perf != nil && tokenCount != nil && *tokenCount > 0 {
		s.perf.CompletionTokens = *tokenCount
	}

	return event
}

// hasMeaningfulStreamOutput reports whether a normalized stream event contains
// model output. Provider lifecycle events (for example Anthropic's
// message_start) may carry usage snapshots but do not count as the first token.
func hasMeaningfulStreamOutput(event *llm.Response) bool {
	if event == nil {
		return false
	}

	for _, choice := range event.Choices {
		delta := choice.Delta
		if delta == nil {
			continue
		}

		if delta.Content.Content != nil && *delta.Content.Content != "" ||
			len(delta.Content.MultipleContent) > 0 ||
			len(delta.ToolCalls) > 0 ||
			delta.ReasoningContent != nil && *delta.ReasoningContent != "" ||
			delta.Reasoning != nil && *delta.Reasoning != "" ||
			len(delta.ReasoningItems) > 0 ||
			len(delta.InlineToolResults) > 0 ||
			delta.Audio != nil ||
			delta.Refusal != "" {
			return true
		}
	}

	return false
}

func (s *recordPerformanceStream) Next() bool {
	if s.stream.Next() {
		return true
	}
	if s.attempt != nil {
		s.attempt.streamError = s.stream.Err()
	}
	return false
}

func (s *recordPerformanceStream) Close() error {
	return s.stream.Close()
}

func (s *recordPerformanceStream) Err() error {
	return s.stream.Err()
}

// ExtractErrorCode extracts HTTP error code from error.
func ExtractErrorCode(err error) int {
	if code := ExtractStatusCodeFromError(err); code > 0 {
		return code
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, pipeline.ErrStreamFirstEventTimeout) ||
		errors.Is(err, pipeline.ErrNonStreamResponseTimeout) {
		return http.StatusGatewayTimeout
	}

	// Default to 500
	return 500
}

func extractErrorMessageForMatching(err error) string {
	message := ExtractErrorMessage(err)
	httpErr := &httpclient.Error{}
	if !errors.As(err, &httpErr) || len(httpErr.Body) == 0 {
		return message
	}

	body := httpErr.Body
	if len(body) > errorMatchBodyLimit {
		body = body[:errorMatchBodyLimit]
	}

	return message + "\n" + string(body)
}

type NoopPerformanceRecording struct {
	pipeline.DummyMiddleware
}

func (m *NoopPerformanceRecording) Name() string {
	return "noop-performance"
}
