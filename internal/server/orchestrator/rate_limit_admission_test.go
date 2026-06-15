package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

func channelWithRPM(id int, name string, rpm int64) *biz.Channel {
	rl := &objects.ChannelRateLimit{}
	if rpm > 0 {
		rl.RPM = lo.ToPtr(rpm)
	}

	return &biz.Channel{
		Channel: &ent.Channel{
			ID:   id,
			Name: name,
			Settings: &objects.ChannelSettings{
				RateLimit: rl,
			},
		},
	}
}

func TestRateLimitAdmission_AllowsOnlyConfiguredRPM(t *testing.T) {
	tracker := NewChannelRequestTracker()
	channel := channelWithRPM(1, "strict-rpm", 2)
	outbound := newTestOutbound(channel)
	middleware := withRateLimitAdmission(outbound, tracker).(*rateLimitAdmissionMiddleware)

	_, err := middleware.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	_, err = middleware.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	_, err = middleware.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.Error(t, err)

	var rpmErr *LocalRPMExhaustedError
	require.ErrorAs(t, err, &rpmErr)
	assert.Equal(t, channel.ID, rpmErr.ChannelID)
	assert.Equal(t, int64(2), rpmErr.Limit)
	assert.Equal(t, int64(2), tracker.GetRequestCount(channel.ID))
}

func TestRateLimitAdmission_NoRPMBypasses(t *testing.T) {
	tracker := NewChannelRequestTracker()
	channel := channelWithRPM(2, "no-rpm", 0)
	outbound := newTestOutbound(channel)
	middleware := withRateLimitAdmission(outbound, tracker).(*rateLimitAdmissionMiddleware)

	for range 3 {
		_, err := middleware.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
		require.NoError(t, err)
	}

	assert.Equal(t, int64(0), tracker.GetRequestCount(channel.ID))
}

func TestRateLimitAdmission_SameChannelRetryCannotBypassRPM(t *testing.T) {
	tracker := NewChannelRequestTracker()
	channel := channelWithRPM(5, "retry-rpm", 1)
	outbound := newTestOutbound(channel)
	middleware := withRateLimitAdmission(outbound, tracker).(*rateLimitAdmissionMiddleware)

	_, err := middleware.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.NoError(t, err)

	_, err = middleware.OnOutboundRawRequest(t.Context(), &httpclient.Request{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLocalRPMExhausted)
	assert.Equal(t, int64(1), tracker.GetRequestCount(channel.ID))
}

func TestRateLimitTracking_OnOutboundRawError_LocalRPMExhaustedIgnored(t *testing.T) {
	tracker := NewChannelRequestTracker()
	channel := channelWithRPM(3, "strict-rpm", 1)
	outbound := newTestOutbound(channel)
	middleware := &rateLimitTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	middleware.OnOutboundRawError(context.Background(), newLocalRPMExhaustedError(channel, 1))

	assert.False(t, tracker.IsCoolingDown(channel.ID),
		"local RPM rejection must not trigger upstream-style cooldown")
}

func TestPersistentOutboundTransformer_CanRetry_LocalRPMExhausted(t *testing.T) {
	channel := channelWithRPM(4, "strict-rpm", 1)
	outbound := newTestOutbound(channel)
	err := newLocalRPMExhaustedError(channel, 1)

	assert.False(t, outbound.CanRetry(err))
	assert.True(t, errors.Is(err, ErrLocalRPMExhausted))
}
