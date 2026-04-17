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

// modelConnectionKey captures the channel+model at increment time.
type modelConnectionKey struct {
	channelID int
	model     string
}

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
	incrKey  *modelConnectionKey // captured at increment time
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

	m.incrKey = &modelConnectionKey{channelID: channel.ID, model: model}
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
	return &modelConnectionTrackingStream{
		ctx:     ctx,
		stream:  stream,
		tracker: m.tracker,
		decrKey: m.incrKey,
	}, nil
}

func (m *modelConnectionTracking) OnOutboundRawError(ctx context.Context, err error) {
	// Decrement model connection count on error
	m.decrementConnection(ctx)
}

func (m *modelConnectionTracking) decrementConnection(ctx context.Context) {
	if m.incrKey == nil {
		return
	}

	m.tracker.DecrementModelConnection(m.incrKey.channelID, m.incrKey.model)

	log.Debug(ctx, "Decremented model connection count",
		log.Int("channel_id", m.incrKey.channelID),
		log.String("model", m.incrKey.model),
		log.Int("active_connections", m.tracker.GetModelConnectionCount(m.incrKey.channelID, m.incrKey.model)),
	)
}

// modelConnectionTrackingStream wraps a stream to decrement model connection count when closed.
//
//nolint:containedctx // ctx is used for logging.
type modelConnectionTrackingStream struct {
	ctx       context.Context
	stream    streams.Stream[*llm.Response]
	tracker   *ModelConnectionTracker
	decrKey   *modelConnectionKey
	closeOnce sync.Once
}

func (s *modelConnectionTrackingStream) Current() *llm.Response {
	return s.stream.Current()
}

func (s *modelConnectionTrackingStream) Next() bool {
	if !s.stream.Next() {
		s.closeOnce.Do(func() {
			if s.decrKey != nil {
				s.tracker.DecrementModelConnection(s.decrKey.channelID, s.decrKey.model)
				log.Debug(s.ctx, "Decremented model connection count (stream exhausted)",
					log.Int("channel_id", s.decrKey.channelID),
					log.String("model", s.decrKey.model),
					log.Int("active_connections", s.tracker.GetModelConnectionCount(s.decrKey.channelID, s.decrKey.model)),
				)
			}
		})
		return false
	}
	return true
}

func (s *modelConnectionTrackingStream) Close() error {
	s.closeOnce.Do(func() {
		if s.decrKey != nil {
			s.tracker.DecrementModelConnection(s.decrKey.channelID, s.decrKey.model)
			log.Debug(s.ctx, "Decremented model connection count (stream closed)",
				log.Int("channel_id", s.decrKey.channelID),
				log.String("model", s.decrKey.model),
				log.Int("active_connections", s.tracker.GetModelConnectionCount(s.decrKey.channelID, s.decrKey.model)),
			)
		}
	})

	return s.stream.Close()
}

func (s *modelConnectionTrackingStream) Err() error {
	return s.stream.Err()
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
