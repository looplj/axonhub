package orchestrator

import (
	"context"
	"errors"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

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

	if m.outbound.state.Perf == nil {
		m.outbound.state.Perf = &biz.PerformanceRecord{}
	}

	perf := m.outbound.state.Perf
	perf.StartTime = time.Now()
	perf.ChannelID = channel.ID
	perf.Success = false
	perf.RequestCompleted = false

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

	m.outbound.state.Perf.MarkSuccess(lo.FromPtr(response.Usage.GetCompletionTokens()))
	m.outbound.state.ChannelService.AsyncRecordPerformance(ctx, m.outbound.state.Perf)

	// Record success to model circuit breaker if available
	if m.outbound.state.ModelCircuitBreaker != nil {
		channel := m.outbound.GetCurrentChannel()
		modelID := m.outbound.GetRequestedModel()
		if channel != nil && modelID != "" {
			m.outbound.state.ModelCircuitBreaker.RecordSuccess(ctx, channel.ID, modelID)
		}
	}

	return response, nil
}

func (m *performanceRecording) OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	return stream, nil
}

func (m *performanceRecording) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return &recordPerformanceStream{
		ctx:    ctx,
		stream: stream,
		state:  m.outbound.state,
	}, nil
}

func (m *performanceRecording) OnOutboundRawError(ctx context.Context, err error) {
	// Record performance metrics for failed requests
	if m.outbound.state.Perf == nil {
		return
	}

	perf := m.outbound.state.Perf
	if errors.Is(err, context.Canceled) {
		perf.MarkCanceled()
	} else {
		errorCode := ExtractErrorCode(err)
		perf.MarkFailed(errorCode)

		// Record error to model circuit breaker if available
		if m.outbound.state.ModelCircuitBreaker != nil {
			channel := m.outbound.GetCurrentChannel()
			modelID := m.outbound.GetRequestedModel()
			if channel != nil && modelID != "" {
				m.outbound.state.ModelCircuitBreaker.RecordError(ctx, channel.ID, modelID)
			}
		}
	}

	m.outbound.state.ChannelService.AsyncRecordPerformance(ctx, perf)
}

// recordPerformanceStream records performance metrics for a stream of responses.
//
//nolint:containedctx // ctx is used for logging.
type recordPerformanceStream struct {
	ctx    context.Context
	stream streams.Stream[*llm.Response]
	state  *PersistenceState

	firstTokenSet bool
}

func (s *recordPerformanceStream) Current() *llm.Response {
	event := s.stream.Current()
	if event == nil {
		return event
	}

	if !s.firstTokenSet && s.state.Perf != nil {
		s.state.Perf.MarkFirstToken()
		s.firstTokenSet = true
	}

	if tokenCount := event.Usage.GetCompletionTokens(); tokenCount != nil && *tokenCount > 0 {
		s.state.Perf.MarkSuccess(*tokenCount)
		s.state.ChannelService.AsyncRecordPerformance(s.ctx, s.state.Perf)

		// Record success to model circuit breaker if available (only once per stream)
		if s.firstTokenSet && s.state.ModelCircuitBreaker != nil {
			if s.state.Perf != nil && s.state.Perf.ChannelID > 0 && s.state.OriginalModel != "" {
				s.state.ModelCircuitBreaker.RecordSuccess(s.ctx, s.state.Perf.ChannelID, s.state.OriginalModel)
			}
		}
	}

	return event
}

func (s *recordPerformanceStream) Next() bool {
	return s.stream.Next()
}

func (s *recordPerformanceStream) Close() error {
	return s.stream.Close()
}

func (s *recordPerformanceStream) Err() error {
	return s.stream.Err()
}

// ExtractErrorCode extracts HTTP error code from error.
func ExtractErrorCode(err error) int {
	// Check if error is an HTTP error
	httpErr := &httpclient.Error{}
	if errors.As(err, &httpErr) {
		code := httpErr.StatusCode
		return code
	}

	// Default to 500
	return 500
}

type NoopPerformanceRecording struct {
	pipeline.DummyMiddleware
}

func (m *NoopPerformanceRecording) Name() string {
	return "noop-performance"
}
