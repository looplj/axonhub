package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
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

func TestRateLimitAdmission_SharesChannelStateAcrossProtocolHandlers(t *testing.T) {
	ctx, client := setupTest(t)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	t.Cleanup(channelService.Stop)
	manager := NewChannelLimiterManager()
	construct := func(inbound transformer.Inbound) *ChatCompletionOrchestrator {
		return NewChatCompletionOrchestrator(channelService, nil, requestService, nil, inbound, systemService,
			usageLogService, nil, nil, nil, nil, manager, nil, PluginIntegration{})
	}
	chat := construct(openai.NewInboundTransformer())
	responses := construct(responses.NewInboundTransformer())
	channel := channelWithRPM(1, "shared", 1)
	firstOutbound, secondOutbound := newTestOutbound(channel), newTestOutbound(channel)

	firstAdmission := withRateLimitAdmission(firstOutbound, chat.rateLimitTracker)
	secondAdmission := withRateLimitAdmission(secondOutbound, responses.rateLimitTracker)
	_, err := firstAdmission.OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.NoError(t, err)
	_, err = secondAdmission.OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.ErrorIs(t, err, ErrLocalRPMExhausted)

	firstTracking := withRateLimitTracking(firstOutbound, chat.rateLimitTracker)
	_, err = firstTracking.OnOutboundLlmResponse(ctx, &llm.Response{Usage: &llm.Usage{TotalTokens: 9}})
	require.NoError(t, err)
	require.EqualValues(t, 9, responses.rateLimitTracker.GetTokenCount(channel.ID))
	firstTracking.OnOutboundRawError(ctx, &httpclient.Error{StatusCode: http.StatusTooManyRequests})
	require.True(t, responses.rateLimitTracker.IsCoolingDown(channel.ID))
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
