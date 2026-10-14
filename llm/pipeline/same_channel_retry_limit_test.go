package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

// mockOutboundWithRetryLimit extends mockOutbound with the optional
// ChannelSameChannelRetryLimit interface so tests can exercise the channel-level
// override of max same-channel retries.
type mockOutboundWithRetryLimit struct {
	mockOutbound

	sameChannelLimit func() (int, bool)
}

func (m *mockOutboundWithRetryLimit) SameChannelRetryLimit() (int, bool) {
	if m.sameChannelLimit != nil {
		return m.sameChannelLimit()
	}

	return 0, false
}

func TestPipeline_MaxSameChannelRetriesFor(t *testing.T) {
	t.Run("falls back to the global value when the outbound has no override", func(t *testing.T) {
		p := &pipeline{Outbound: &mockOutbound{}, maxSameChannelRetries: 2}
		require.Equal(t, 2, p.maxSameChannelRetriesFor())
	})

	t.Run("falls back to the global value when the outbound does not implement the interface", func(t *testing.T) {
		p := &pipeline{Outbound: &mockOutbound{}, maxSameChannelRetries: 5}
		require.Equal(t, 5, p.maxSameChannelRetriesFor())
	})

	t.Run("uses the channel-level override when provided", func(t *testing.T) {
		outbound := &mockOutboundWithRetryLimit{
			sameChannelLimit: func() (int, bool) { return 7, true },
		}
		p := &pipeline{Outbound: outbound, maxSameChannelRetries: 2}
		require.Equal(t, 7, p.maxSameChannelRetriesFor())
	})

	t.Run("falls back to the global value when the override declines", func(t *testing.T) {
		outbound := &mockOutboundWithRetryLimit{
			sameChannelLimit: func() (int, bool) { return 0, false },
		}
		p := &pipeline{Outbound: outbound, maxSameChannelRetries: 2}
		require.Equal(t, 2, p.maxSameChannelRetriesFor())
	})
}

// TestPipeline_Process_ChannelLevelSameChannelRetries pins that a channel-level
// override actually changes how many times Process retries the same channel.
func TestPipeline_Process_ChannelLevelSameChannelRetries(t *testing.T) {
	ctx := context.Background()

	t.Run("zero override stops same-channel retries even when the global value allows them", func(t *testing.T) {
		execCalls := 0
		executor := &mockExecutor{
			do: func(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				execCalls++
				return nil, errors.New("temporary error")
			},
		}

		outbound := &mockOutboundWithRetryLimit{
			mockOutbound: mockOutbound{
				canRetry:        func(err error) bool { return true },
				hasMoreChannels: func() bool { return false },
			},
			sameChannelLimit: func() (int, bool) { return 0, true },
		}

		p := &pipeline{
			Executor:              executor,
			Inbound:               &mockInbound{},
			Outbound:              outbound,
			maxSameChannelRetries: 3,
		}

		_, err := p.Process(ctx, &httpclient.Request{})
		require.Error(t, err)
		require.Equal(t, 1, execCalls, "a zero channel override must stop same-channel retries despite the global value")
	})

	t.Run("override raises the retry count above the global value", func(t *testing.T) {
		execCalls := 0
		executor := &mockExecutor{
			do: func(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				execCalls++
				if execCalls > 3 {
					return &httpclient.Response{}, nil
				}

				return nil, errors.New("temporary error")
			},
		}

		outbound := &mockOutboundWithRetryLimit{
			mockOutbound: mockOutbound{
				canRetry:        func(err error) bool { return true },
				hasMoreChannels: func() bool { return false },
			},
			sameChannelLimit: func() (int, bool) { return 3, true },
		}

		p := &pipeline{
			Executor:              executor,
			Inbound:               &mockInbound{},
			Outbound:              outbound,
			maxSameChannelRetries: 1,
		}

		res, err := p.Process(ctx, &httpclient.Request{})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, 4, execCalls, "the channel override (3) must drive the retry count, not the global value (1)")
	})
}
