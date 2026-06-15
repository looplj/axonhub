package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
)

// withRateLimitAdmission enforces strict per-instance RPM before an attempt is
// sent upstream. It intentionally runs after channel concurrency admission so
// queue rejections never consume RPM.
func withRateLimitAdmission(outbound *PersistentOutboundTransformer, tracker *ChannelRequestTracker) pipeline.Middleware {
	if tracker == nil {
		return &noopRateLimitAdmission{}
	}

	return &rateLimitAdmissionMiddleware{
		outbound: outbound,
		tracker:  tracker,
	}
}

type rateLimitAdmissionMiddleware struct {
	pipeline.DummyMiddleware

	outbound *PersistentOutboundTransformer
	tracker  *ChannelRequestTracker
}

func (m *rateLimitAdmissionMiddleware) Name() string {
	return "rate-limit-admission"
}

func (m *rateLimitAdmissionMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	channel := m.outbound.GetCurrentChannel()
	if channel == nil || channel.Settings == nil || channel.Settings.RateLimit == nil {
		return request, nil
	}

	limit := channel.Settings.RateLimit.RPM
	if limit == nil || *limit <= 0 {
		return request, nil
	}

	if !m.tracker.TryAcquireRequest(channel.ID, *limit) {
		log.Debug(ctx, "channel local RPM admission rejected",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int64("rpm_limit", *limit),
		)

		return nil, newLocalRPMExhaustedError(channel, *limit)
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "channel local RPM admission acquired",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int64("rpm_limit", *limit),
		)
	}

	return request, nil
}

type noopRateLimitAdmission struct {
	pipeline.DummyMiddleware
}

func (m *noopRateLimitAdmission) Name() string {
	return "rate-limit-admission-noop"
}
