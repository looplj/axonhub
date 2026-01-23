package orchestrator

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

func withModelCircuitBreaker(outbound *PersistentOutboundTransformer, modelCircuitBreaker *biz.ModelCircuitBreaker) pipeline.Middleware {
	return &modelCircuitBreakerTracker{
		outbound:            outbound,
		modelCircuitBreaker: modelCircuitBreaker,
	}
}

type modelCircuitBreakerTracker struct {
	pipeline.DummyMiddleware

	outbound            *PersistentOutboundTransformer
	modelCircuitBreaker *biz.ModelCircuitBreaker

	probeActive    bool
	probeChannelID int
	probeModelID   string
}

func (m *modelCircuitBreakerTracker) Name() string {
	return "model-circuit-breaker-tracker"
}

func (m *modelCircuitBreakerTracker) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	if m.outbound == nil || m.outbound.state == nil {
		return request, nil
	}

	if m.modelCircuitBreaker == nil {
		return request, nil
	}

	channel := m.outbound.GetCurrentChannel()

	modelID := m.outbound.GetRequestedModel()
	if channel == nil || modelID == "" {
		return request, nil
	}

	stats := m.modelCircuitBreaker.GetModelCircuitBreakerStats(ctx, channel.ID, modelID)
	if stats == nil || stats.State != biz.StateOpen {
		return request, nil
	}

	if !m.modelCircuitBreaker.TryBeginProbe(ctx, channel.ID, modelID) {
		log.Debug(ctx, "skipping candidate by circuit breaker: probe conditions not met or another probe in progress",
			log.Int("channel_id", channel.ID),
			log.String("model_id", modelID),
		)

		return nil, errSkipCandidateByCircuitBreaker
	}

	m.probeActive = true
	m.probeChannelID = channel.ID
	m.probeModelID = modelID

	return request, nil
}

func (m *modelCircuitBreakerTracker) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	if m.outbound != nil {
		m.releaseProbeLease()

		channel := m.outbound.GetCurrentChannel()
		modelID := m.outbound.GetRequestedModel()
		m.modelCircuitBreaker.RecordSuccess(ctx, channel.ID, modelID)
	}

	return response, nil
}

func (m *modelCircuitBreakerTracker) OnOutboundRawError(ctx context.Context, err error) {
	if m.outbound != nil {
		m.releaseProbeLease()

		if !errors.Is(err, context.Canceled) {
			channel := m.outbound.GetCurrentChannel()
			modelID := m.outbound.GetRequestedModel()
			m.modelCircuitBreaker.RecordError(ctx, channel.ID, modelID)
		}
	}
}

func (m *modelCircuitBreakerTracker) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return &probeReleasingStream{
		ctx:            ctx,
		stream:         stream,
		state:          m.outbound.state,
		probeChannelID: m.probeChannelID,
		probeModelID:   m.probeModelID,
		release: func() {
			if m.outbound != nil {
				m.releaseProbeLease()
			}
		},
		released:            false,
		recorded:            false,
		modelCircuitBreaker: m.modelCircuitBreaker,
	}, nil
}

func (m *modelCircuitBreakerTracker) releaseProbeLease() {
	if m.outbound == nil || m.outbound.state == nil || !m.probeActive {
		return
	}

	m.modelCircuitBreaker.EndProbe(m.probeChannelID, m.probeModelID)
	m.probeActive = false
	m.probeChannelID = 0
	m.probeModelID = ""
}

//nolint:containedctx // Checked.
type probeReleasingStream struct {
	ctx      context.Context
	stream   streams.Stream[*llm.Response]
	state    *PersistenceState
	release  func()
	released bool
	recorded bool

	modelCircuitBreaker *biz.ModelCircuitBreaker
	probeChannelID      int
	probeModelID        string
}

func (s *probeReleasingStream) Next() bool {
	return s.stream.Next()
}

func (s *probeReleasingStream) Current() *llm.Response {
	event := s.stream.Current()
	if event == nil {
		return nil
	}

	if !s.recorded {
		if tokenCount := event.Usage.GetCompletionTokens(); tokenCount != nil && *tokenCount > 0 {
			channelID := s.probeChannelID
			modelID := s.probeModelID

			s.modelCircuitBreaker.RecordSuccess(s.ctx, channelID, modelID)
			s.recorded = true
		}
	}

	return event
}

func (s *probeReleasingStream) Err() error {
	return s.stream.Err()
}

func (s *probeReleasingStream) Close() error {
	if !s.released && s.release != nil {
		s.released = true
		s.release()
	}

	return s.stream.Close()
}
