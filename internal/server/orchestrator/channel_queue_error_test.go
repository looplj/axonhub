package orchestrator

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

func makeBizChannel(id int, name string) *biz.Channel {
	return &biz.Channel{
		Channel: &ent.Channel{ID: id, Name: name},
	}
}

func TestAsChannelQueueError_QueueFull(t *testing.T) {
	t.Parallel()

	wrapped := asChannelQueueError(makeBizChannel(1, "kimi"), ErrChannelQueueFull)
	require.NotNil(t, wrapped)
	assert.Equal(t, channelQueueReasonFull, wrapped.Reason)

	// errors.Is finds the typed sentinel.
	assert.ErrorIs(t, wrapped, ErrChannelQueueFull)

	// errors.AsType finds the synthetic httpclient.Error in the chain.
	httpErr, ok := errors.AsType[*httpclient.Error](wrapped)
	require.True(t, ok)
	assert.Equal(t, http.StatusTooManyRequests, httpErr.StatusCode)
	assert.Empty(t, httpErr.Headers, "queue rejection must not advertise Retry-After")
	assert.Contains(t, string(httpErr.Body), "rate_limit_error")
	assert.Contains(t, string(httpErr.Body), "channel_queue_full")
	assert.Contains(t, string(httpErr.Body), "kimi")
}

func TestAsChannelQueueError_QueueTimeout(t *testing.T) {
	t.Parallel()

	wrapped := asChannelQueueError(makeBizChannel(2, "openai"), ErrChannelQueueTimeout)
	require.NotNil(t, wrapped)
	assert.Equal(t, channelQueueReasonTimeout, wrapped.Reason)
	assert.ErrorIs(t, wrapped, ErrChannelQueueTimeout)

	httpErr, ok := errors.AsType[*httpclient.Error](wrapped)
	require.True(t, ok)
	assert.Equal(t, http.StatusTooManyRequests, httpErr.StatusCode)
	assert.Contains(t, string(httpErr.Body), "channel_queue_timeout")
}

func TestAsChannelQueueError_PassThrough(t *testing.T) {
	t.Parallel()

	// Non-queue errors are not wrapped.
	assert.Nil(t, asChannelQueueError(makeBizChannel(1, "x"), errors.New("other")))
	assert.Nil(t, asChannelQueueError(nil, ErrChannelQueueFull))
	assert.Nil(t, asChannelQueueError(makeBizChannel(1, "x"), nil))
}

func TestAsChannelQueueError_TriggersRateLimitDetection(t *testing.T) {
	t.Parallel()

	wrapped := asChannelQueueError(makeBizChannel(3, "anthropic"), ErrChannelQueueFull)
	require.NotNil(t, wrapped)

	// IsRateLimitErr matches via the inner httpErr (StatusCode == 429), but
	// HasRetryAfterHeader must NOT match: the rate_limit_tracking middleware
	// uses it to decide whether to set an upstream-style cooldown, and we never
	// want a local queue rejection to trigger that path.
	assert.True(t, httpclient.IsRateLimitErr(wrapped))
	assert.False(t, httpclient.HasRetryAfterHeader(wrapped))
}
