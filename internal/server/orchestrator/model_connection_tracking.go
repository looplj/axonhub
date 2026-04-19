package orchestrator

import (
	"context"
	"sync"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

type modelConnectionKey struct {
	channelID int
	model     string
}

func isNilModelConnectionTracker(tracker ModelConnectionTrackerInterface) bool {
	if tracker == nil {
		return true
	}

	concrete, ok := tracker.(*ModelConnectionTracker)

	return ok && concrete == nil
}

func withModelConnectionTracking(outbound *PersistentOutboundTransformer, tracker ModelConnectionTrackerInterface) pipeline.Middleware {
	if isNilModelConnectionTracker(tracker) {
		return &noopModelConnectionTracking{}
	}

	return &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}
}

type modelConnectionTracking struct {
	pipeline.DummyMiddleware

	outbound    *PersistentOutboundTransformer
	tracker     ModelConnectionTrackerInterface
	incrKey     *modelConnectionKey
	decremented bool
	decrementMu sync.Mutex
}

func (m *modelConnectionTracking) Name() string {
	return "track-model-connections"
}

func (m *modelConnectionTracking) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return request, nil
	}

	model := m.outbound.GetRequestedModel()
	if model == "" {
		return request, nil
	}

	if m.incrKey != nil && !m.decremented {
		m.tracker.DecrementModelConnection(m.incrKey.channelID, m.incrKey.model)
	}

	m.incrKey = &modelConnectionKey{channelID: channel.ID, model: model}
	m.tracker.IncrementModelConnection(channel.ID, model)

	m.decremented = false

	log.Debug(ctx, "Incremented model connection count",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.String("model", model),
		log.Int("active_connections", m.tracker.GetModelConnectionCount(channel.ID, model)),
	)

	return request, nil
}

func (m *modelConnectionTracking) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	m.decrementConnection(ctx)
	return response, nil
}

func (m *modelConnectionTracking) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return &onCloseStream{
		stream: stream,
		onClose: func() {
			m.decrementConnection(ctx)
		},
	}, nil
}

func (m *modelConnectionTracking) OnOutboundRawError(ctx context.Context, err error) {
	m.decrementConnection(ctx)
}

func (m *modelConnectionTracking) decrementConnection(ctx context.Context) {
	m.decrementMu.Lock()
	defer m.decrementMu.Unlock()

	if m.decremented || m.incrKey == nil {
		return
	}

	m.decremented = true
	m.tracker.DecrementModelConnection(m.incrKey.channelID, m.incrKey.model)

	log.Debug(ctx, "Decremented model connection count",
		log.Int("channel_id", m.incrKey.channelID),
		log.String("model", m.incrKey.model),
		log.Int("active_connections", m.tracker.GetModelConnectionCount(m.incrKey.channelID, m.incrKey.model)),
	)
}

type noopModelConnectionTracking struct {
	pipeline.DummyMiddleware
}

func (m *noopModelConnectionTracking) Name() string {
	return "track-model-connections-noop"
}

func (m *noopModelConnectionTracking) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return stream, nil
}
