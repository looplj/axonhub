package orchestrator

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

func isNilConnectionTracker(tracker ConnectionTracker) bool {
	if tracker == nil {
		return true
	}
	v := reflect.ValueOf(tracker)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

func withConnectionTracking(outbound *PersistentOutboundTransformer, tracker ConnectionTracker) pipeline.Middleware {
	if isNilConnectionTracker(tracker) {
		return &noopConnectionTracking{}
	}

	return &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}
}

type connectionTracking struct {
	pipeline.DummyMiddleware

	outbound     *PersistentOutboundTransformer
	tracker      ConnectionTracker
	channelID    int
	hasIncrement bool
	decremented  bool
	decrementMu  sync.Mutex
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
	if m.hasIncrement && !m.decremented {
		m.tracker.DecrementConnection(m.channelID)
	}
	m.channelID = channel.ID
	m.hasIncrement = true
	m.decremented = false
	m.tracker.IncrementConnection(channel.ID)
	m.decrementMu.Unlock()

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
	m.decrementMu.Lock()
	captureChannelID := m.channelID
	captureHasIncrement := m.hasIncrement
	captureDecremented := m.decremented
	m.decremented = true
	m.decrementMu.Unlock()

	captureChannelName := ""
	if ch := m.outbound.GetCurrentChannel(); ch != nil {
		captureChannelName = ch.Name
	}

	wrapped := newOnCloseStream(stream, func() {
		if captureDecremented || !captureHasIncrement {
			return
		}
		m.tracker.DecrementConnection(captureChannelID)

		log.Debug(context.WithoutCancel(ctx), "Decremented connection count (stream close)",
			log.Int("channel_id", captureChannelID),
			log.String("channel_name", captureChannelName),
			log.Int("active_connections", m.tracker.GetActiveConnections(captureChannelID)),
		)
	})

	if captureHasIncrement {
		go func() {
			select {
			case <-ctx.Done():
				wrapped.Close()
			case <-wrapped.Done():
			}
		}()
	}

	return wrapped, nil
}

func (m *connectionTracking) OnOutboundRawError(ctx context.Context, err error) {
	m.decrementConnection(ctx)
}

func (m *connectionTracking) decrementConnection(ctx context.Context) {
	m.decrementMu.Lock()
	defer m.decrementMu.Unlock()

	if m.decremented || !m.hasIncrement {
		return
	}

	m.decremented = true

	m.tracker.DecrementConnection(m.channelID)

	channelName := ""
	if ch := m.outbound.GetCurrentChannel(); ch != nil {
		channelName = ch.Name
	}

	log.Debug(ctx, "Decremented connection count",
		log.Int("channel_id", m.channelID),
		log.String("channel_name", channelName),
		log.Int("active_connections", m.tracker.GetActiveConnections(m.channelID)),
	)
}

type noopConnectionTracking struct {
	pipeline.DummyMiddleware
}

func (m *noopConnectionTracking) Name() string {
	return "track-connections-noop"
}

func withChannelLimiter(
	outbound *PersistentOutboundTransformer,
	manager *ChannelLimiterManager,
	limiterMetrics *ChannelLimiterMetrics,
) pipeline.Middleware {
	return &channelLimiterMiddleware{
		outbound: outbound,
		manager:  manager,
		metrics:  limiterMetrics,
	}
}

type channelLimiterMiddleware struct {
	pipeline.DummyMiddleware

	outbound *PersistentOutboundTransformer
	manager  *ChannelLimiterManager
	metrics  *ChannelLimiterMetrics

	current atomic.Pointer[limiterSlot]
}

type limiterSlot struct {
	lim  *ChannelLimiter
	once sync.Once
}

func (m *channelLimiterMiddleware) Name() string { return "channel-limiter" }

func (m *channelLimiterMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return request, nil
	}

	lim := m.manager.GetOrCreate(channel)
	if lim == nil {
		return request, nil
	}

	hardMode := lim.queueSize > 0

	var acquireStart time.Time
	if hardMode {
		acquireStart = time.Now()
	}

	if err := lim.Acquire(ctx); err != nil {
		if queueErr := asChannelQueueError(channel, err); queueErr != nil {
			switch queueErr.Reason {
			case channelQueueReasonFull:
				m.metrics.IncQueueFull(ctx, channel)
			case channelQueueReasonTimeout:
				m.metrics.IncQueueTimeout(ctx, channel)
			}

			log.Debug(ctx, "channel queue admission rejected",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.String("reason", queueErr.Reason),
			)

			return nil, queueErr
		}

		return nil, err
	}

	if hardMode {
		m.metrics.ObserveQueueWait(ctx, channel, time.Since(acquireStart))
	}

	if old := m.current.Swap(&limiterSlot{lim: lim}); old != nil {
		old.once.Do(func() { old.lim.Release() })
	}

	if log.DebugEnabled(ctx) {
		inFlight, waiting := lim.Stats()
		log.Debug(ctx, "channel limiter slot acquired",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int("in_flight", inFlight),
			log.Int("waiting", waiting),
		)
	}

	return request, nil
}

func (m *channelLimiterMiddleware) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	m.releaseCurrent(ctx)
	return response, nil
}

func (m *channelLimiterMiddleware) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	slot := m.current.Load()
	if slot == nil {
		return stream, nil
	}

	return &channelLimiterStream{
		Stream:  stream,
		release: func() { m.releaseSlot(ctx, slot) },
	}, nil
}

func (m *channelLimiterMiddleware) OnOutboundRawError(ctx context.Context, err error) {
	m.releaseCurrent(ctx)
}

func (m *channelLimiterMiddleware) releaseCurrent(ctx context.Context) {
	if slot := m.current.Load(); slot != nil {
		m.releaseSlot(ctx, slot)
	}
}

func (m *channelLimiterMiddleware) releaseSlot(ctx context.Context, slot *limiterSlot) {
	slot.once.Do(func() {
		slot.lim.Release()

		if log.DebugEnabled(ctx) {
			channel := m.outbound.GetCurrentChannel()
			inFlight, waiting := slot.lim.Stats()
			fields := []log.Field{
				log.Int("in_flight", inFlight),
				log.Int("waiting", waiting),
			}
			if channel != nil {
				fields = append(fields,
					log.Int("channel_id", channel.ID),
					log.String("channel_name", channel.Name),
				)
			}
			log.Debug(ctx, "channel limiter slot released", fields...)
		}
	})
}

type channelLimiterStream struct {
	streams.Stream[*llm.Response]
	release func()
}

func (s *channelLimiterStream) Close() error {
	s.release()
	return s.Stream.Close()
}
