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

// withModelConnectionTracking creates a middleware that tracks active connections per channel+model.
func withModelConnectionTracking(outbound *PersistentOutboundTransformer, tracker *ModelConnectionTracker) pipeline.Middleware {
	if tracker == nil {
		// If no tracker provided, return a no-op middleware
		return &noopModelConnectionTracking{}
	}

	return &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}
}

// modelConnectionTracking is a middleware that increments/decrements model connection count.
type modelConnectionTracking struct {
	pipeline.DummyMiddleware

	outbound *PersistentOutboundTransformer
	tracker  *ModelConnectionTracker
}

func (m *modelConnectionTracking) Name() string {
	return "track-model-connections"
}

func (m *modelConnectionTracking) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	// Increment model connection count when starting a request
	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return request, nil
	}

	model := m.outbound.GetRequestedModel()
	if model == "" {
		return request, nil
	}

	m.tracker.IncrementModelConnection(channel.ID, model)

	log.Debug(ctx, "Incremented model connection count",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.String("model", model),
		log.Int("active_connections", m.tracker.GetModelConnectionCount(channel.ID, model)),
	)

	return request, nil
}

func (m *modelConnectionTracking) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	// Decrement model connection count after response completes
	m.decrementConnection(ctx)
	return response, nil
}

func (m *modelConnectionTracking) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	// Wrap stream to decrement connection when stream closes
	return &modelConnectionTrackingStream{
		ctx:      ctx,
		stream:   stream,
		tracker:  m.tracker,
		outbound: m.outbound,
	}, nil
}

func (m *modelConnectionTracking) OnOutboundRawError(ctx context.Context, err error) {
	// Decrement model connection count on error
	m.decrementConnection(ctx)
}

func (m *modelConnectionTracking) decrementConnection(ctx context.Context) {
	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return
	}

	model := m.outbound.GetRequestedModel()
	if model == "" {
		return
	}

	m.tracker.DecrementModelConnection(channel.ID, model)

	log.Debug(ctx, "Decremented model connection count",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.String("model", model),
		log.Int("active_connections", m.tracker.GetModelConnectionCount(channel.ID, model)),
	)
}

// modelConnectionTrackingStream wraps a stream to decrement model connection count when closed.
//
//nolint:containedctx // ctx is used for logging.
type modelConnectionTrackingStream struct {
	ctx        context.Context
	stream     streams.Stream[*llm.Response]
	tracker    *ModelConnectionTracker
	outbound   *PersistentOutboundTransformer
	closeOnce  sync.Once
}

func (s *modelConnectionTrackingStream) Current() *llm.Response {
	return s.stream.Current()
}

func (s *modelConnectionTrackingStream) Next() bool {
	return s.stream.Next()
}

func (s *modelConnectionTrackingStream) Close() error {
	s.closeOnce.Do(func() {
		s.decrementConnection()
	})

	return s.stream.Close()
}

func (s *modelConnectionTrackingStream) Err() error {
	return s.stream.Err()
}

func (s *modelConnectionTrackingStream) decrementConnection() {
	channel := s.outbound.GetCurrentChannel()
	if channel == nil {
		return
	}

	model := s.outbound.GetRequestedModel()
	if model == "" {
		return
	}

	s.tracker.DecrementModelConnection(channel.ID, model)

	log.Debug(s.ctx, "Decremented model connection count (stream closed)",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.String("model", model),
		log.Int("active_connections", s.tracker.GetModelConnectionCount(channel.ID, model)),
	)
}

// noopModelConnectionTracking is a no-op middleware when model connection tracking is disabled.
type noopModelConnectionTracking struct {
	pipeline.DummyMiddleware
}

func (m *noopModelConnectionTracking) Name() string {
	return "track-model-connections-noop"
}

func (m *noopModelConnectionTracking) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return stream, nil
}
