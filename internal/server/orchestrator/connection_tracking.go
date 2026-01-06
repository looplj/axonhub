package orchestrator

import (
	"context"

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

	outbound *PersistentOutboundTransformer
	tracker  ConnectionTracker
}

func (m *connectionTracking) Name() string {
	return "track-connections"
}

func (m *connectionTracking) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	// Increment connection count when starting a request
	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return request, nil
	}

	m.tracker.IncrementConnection(channel.ID)

	log.Debug(ctx, "Incremented connection count",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Int("active_connections", m.tracker.GetActiveConnections(channel.ID)),
	)

	return request, nil
}

func (m *connectionTracking) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	// Decrement connection count after response completes
	m.decrementConnection(ctx)
	return response, nil
}

func (m *connectionTracking) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	// Wrap stream to decrement connection when stream closes
	return &connectionTrackingStream{
		ctx:      ctx,
		stream:   stream,
		tracker:  m.tracker,
		outbound: m.outbound,
	}, nil
}

func (m *connectionTracking) OnOutboundRawError(ctx context.Context, err error) {
	// Decrement connection count on error
	m.decrementConnection(ctx)
}

func (m *connectionTracking) decrementConnection(ctx context.Context) {
	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return
	}

	m.tracker.DecrementConnection(channel.ID)

	log.Debug(ctx, "Decremented connection count",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Int("active_connections", m.tracker.GetActiveConnections(channel.ID)),
	)
}

// connectionTrackingStream wraps a stream to decrement connection count when closed.
//
//nolint:containedctx // ctx is used for logging.
type connectionTrackingStream struct {
	ctx      context.Context
	stream   streams.Stream[*llm.Response]
	tracker  ConnectionTracker
	outbound *PersistentOutboundTransformer
	closed   bool
}

func (s *connectionTrackingStream) Current() *llm.Response {
	return s.stream.Current()
}

func (s *connectionTrackingStream) Next() bool {
	return s.stream.Next()
}

func (s *connectionTrackingStream) Close() error {
	if !s.closed {
		s.closed = true
		s.decrementConnection()
	}

	return s.stream.Close()
}

func (s *connectionTrackingStream) Err() error {
	return s.stream.Err()
}

func (s *connectionTrackingStream) decrementConnection() {
	channel := s.outbound.GetCurrentChannel()
	if channel == nil {
		return
	}

	s.tracker.DecrementConnection(channel.ID)

	log.Debug(s.ctx, "Decremented connection count (stream closed)",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Int("active_connections", s.tracker.GetActiveConnections(channel.ID)),
	)
}

// noopConnectionTracking is a no-op middleware when connection tracking is disabled.
type noopConnectionTracking struct {
	pipeline.DummyMiddleware
}

func (m *noopConnectionTracking) Name() string {
	return "track-connections-noop"
}
