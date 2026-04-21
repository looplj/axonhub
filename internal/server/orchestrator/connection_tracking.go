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

// withConnectionTracking creates a middleware that tracks active connections per channel.
func withConnectionTracking(outbound *PersistentOutboundTransformer, tracker ConnectionTracker) pipeline.Middleware {
	if tracker == nil {
		// If no tracker provided, return a no-op middleware
		return &noopConnectionTracking{}
	}

	return &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}
}

// connectionTracking is a middleware that increments/decrements connection count.
type connectionTracking struct {
	pipeline.DummyMiddleware

	outbound    *PersistentOutboundTransformer
	tracker     ConnectionTracker
	channelID   int
	decremented bool
	decrementMu sync.Mutex
}

func (m *connectionTracking) Name() string {
	return "track-connections"
}

func (m *connectionTracking) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return request, nil
	}

	m.decrementMu.Lock()
	m.channelID = channel.ID
	m.decremented = false
	m.decrementMu.Unlock()

	m.tracker.IncrementConnection(channel.ID)

	log.Debug(ctx, "Incremented connection count",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Int("active_connections", m.tracker.GetActiveConnections(channel.ID)),
	)

	return request, nil
}

func (m *connectionTracking) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	m.decrementConnection(ctx)
	return response, nil
}

func (m *connectionTracking) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return &onCloseStream{
		stream: stream,
		onClose: func() {
			m.decrementConnection(context.WithoutCancel(ctx))
		},
	}, nil
}

func (m *connectionTracking) OnOutboundRawError(ctx context.Context, err error) {
	m.decrementConnection(ctx)
}

func (m *connectionTracking) decrementConnection(ctx context.Context) {
	m.decrementMu.Lock()
	defer m.decrementMu.Unlock()

	if m.decremented || m.channelID == 0 {
		return
	}

	m.decremented = true

	m.tracker.DecrementConnection(m.channelID)

	log.Debug(ctx, "Decremented connection count",
		log.Int("channel_id", m.channelID),
		log.Int("active_connections", m.tracker.GetActiveConnections(m.channelID)),
	)
}

// noopConnectionTracking is a no-op middleware when connection tracking is disabled.
type noopConnectionTracking struct {
	pipeline.DummyMiddleware
}

func (m *noopConnectionTracking) Name() string {
	return "track-connections-noop"
}
