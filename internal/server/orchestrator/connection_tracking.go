package orchestrator

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

// withChannelLimiter constructs the per-request middleware that enforces
// per-channel admission control via ChannelLimiterManager.
//
// Manager must be non-nil. Channels without a configured concurrency limit
// (manager returns nil from GetOrCreate) bypass admission and run unmodified.
// metrics may be nil — the middleware skips emissions in that case.
func withChannelLimiter(
	outbound *PersistentOutboundTransformer,
	manager *ChannelLimiterManager,
	metrics *ChannelLimiterMetrics,
) pipeline.Middleware {
	return &channelLimiterMiddleware{
		outbound: outbound,
		manager:  manager,
		metrics:  metrics,
	}
}

// channelLimiterMiddleware acquires a slot before forwarding the request to the
// upstream provider and releases it once the response is fully drained (or the
// request fails).
//
// One instance per Process call, but Process re-enters OnOutboundRawRequest on
// same-channel retry and channel switch. Each Acquire mints its own limiterSlot
// so the release Once cannot bleed across attempts.
type channelLimiterMiddleware struct {
	pipeline.DummyMiddleware

	outbound *PersistentOutboundTransformer
	manager  *ChannelLimiterManager
	metrics  *ChannelLimiterMetrics

	// Single-writer: pipeline.Process invokes middleware hooks serially, so
	// Acquire/Store of current is never racing itself. The only concurrent
	// reader is a wrapped stream's Close, which captures the slot pointer at
	// wrap time and never reads current.
	current atomic.Pointer[limiterSlot]
}

// limiterSlot owns one Acquire/Release pair so each pipeline attempt has its
// own once guard.
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

	// Only time the acquire when it can actually wait — soft mode (queueSize == 0)
	// always returns immediately so the histogram observation would be pure noise.
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

	m.current.Store(&limiterSlot{lim: lim})

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

// releaseSlot must be passed the slot the caller intends to release — stream
// wrappers release the slot they were minted with, not whatever's current at
// Close time.
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

// channelLimiterStream wraps an outbound stream and routes Close to the
// middleware's release path. Because release is sync.Once-guarded, double calls
// from Close + OnOutboundRawError are safe.
type channelLimiterStream struct {
	streams.Stream[*llm.Response]
	release func()
}

func (s *channelLimiterStream) Close() error {
	s.release()
	return s.Stream.Close()
}
