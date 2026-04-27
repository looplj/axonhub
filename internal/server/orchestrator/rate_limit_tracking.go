package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

// withRateLimitTracking creates a middleware that tracks request counts per channel for rate limiting.
func withRateLimitTracking(outbound *PersistentOutboundTransformer, tracker *ChannelRequestTracker) pipeline.Middleware {
	if tracker == nil {
		return &noopRateLimitTracking{}
	}

	return &rateLimitTracking{
		outbound: outbound,
		tracker:  tracker,
	}
}

// rateLimitTracking is a middleware that increments request count for rate limiting.
type rateLimitTracking struct {
	pipeline.DummyMiddleware

	outbound *PersistentOutboundTransformer
	tracker  *ChannelRequestTracker
}

func (m *rateLimitTracking) Name() string {
	return "track-rate-limit"
}

func (m *rateLimitTracking) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return request, nil
	}

	m.tracker.IncrementRequest(channel.ID)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "Incremented rate limit request count",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int64("current_rpm", m.tracker.GetRequestCount(channel.ID)),
		)
	}

	return request, nil
}

func (m *rateLimitTracking) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	channel := m.outbound.GetCurrentChannel()
	if channel == nil || response == nil || response.Usage == nil {
		return response, nil
	}

	totalTokens := response.Usage.TotalTokens
	if totalTokens > 0 {
		m.tracker.AddTokens(channel.ID, totalTokens)

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "Incremented rate limit token count",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.Int64("tokens", totalTokens),
				log.Int64("current_tpm", m.tracker.GetTokenCount(channel.ID)),
			)
		}
	}

	return response, nil
}

func (m *rateLimitTracking) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return &rateLimitTrackingStream{
		ctx:      ctx,
		stream:   stream,
		tracker:  m.tracker,
		outbound: m.outbound,
	}, nil
}

// OnOutboundRawError handles raw HTTP errors, specifically capturing 429 Too Many Requests.
// When a 429 is received, it parses the Retry-After header and sets a cooldown for the channel.
func (m *rateLimitTracking) OnOutboundRawError(ctx context.Context, err error) {
	if m.outbound == nil {
		return
	}

	// Local queue rejections never reached upstream — they must not trigger cooldown.
	if isChannelQueueError(err) {
		return
	}

	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return
	}

	// Only cool down a channel when the upstream explicitly provides a cooldown.
	if !httpclient.HasRetryAfterHeader(err) {
		return
	}

	// Parse Retry-After header from 429 error
	cooldown, ok := httpclient.ParseRetryAfter(err)
	if !ok {
		return
	}

	// Set cooldown for this channel
	m.tracker.SetCooldown(channel.ID, time.Now().Add(cooldown))

	log.Warn(ctx, "channel cooling down due to 429",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Duration("cooldown", cooldown),
	)
}

// rateLimitTrackingStream wraps a stream to track token usage for rate limiting.
//
//nolint:containedctx // ctx is used for logging.
type rateLimitTrackingStream struct {
	ctx      context.Context
	stream   streams.Stream[*llm.Response]
	tracker  *ChannelRequestTracker
	outbound *PersistentOutboundTransformer
}

func (s *rateLimitTrackingStream) Current() *llm.Response {
	event := s.stream.Current()
	if event == nil {
		return event
	}

	// Track tokens if usage information is present (typically in the last chunk)
	if event.Usage != nil && event.Usage.TotalTokens > 0 {
		channel := s.outbound.GetCurrentChannel()
		if channel != nil {
			s.tracker.AddTokens(channel.ID, event.Usage.TotalTokens)

			if log.DebugEnabled(s.ctx) {
				log.Debug(s.ctx, "Incremented rate limit token count from stream",
					log.Int("channel_id", channel.ID),
					log.String("channel_name", channel.Name),
					log.Int64("tokens", event.Usage.TotalTokens),
					log.Int64("current_tpm", s.tracker.GetTokenCount(channel.ID)),
				)
			}
		}
	}

	return event
}

func (s *rateLimitTrackingStream) Next() bool {
	return s.stream.Next()
}

func (s *rateLimitTrackingStream) Close() error {
	return s.stream.Close()
}

func (s *rateLimitTrackingStream) Err() error {
	return s.stream.Err()
}

// noopRateLimitTracking is a no-op middleware when rate limit tracking is disabled.
type noopRateLimitTracking struct {
	pipeline.DummyMiddleware
}

func (m *noopRateLimitTracking) Name() string {
	return "track-rate-limit-noop"
}

func (m *noopRateLimitTracking) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return stream, nil
}
