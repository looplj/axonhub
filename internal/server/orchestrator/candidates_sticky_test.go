package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

type fakePreviousChannelProvider struct {
	traceChannelIDs  map[int]int
	threadChannelIDs map[int]int
}

func (p *fakePreviousChannelProvider) GetPreviousChannelID(_ context.Context, traceID int) (int, error) {
	return p.traceChannelIDs[traceID], nil
}

func (p *fakePreviousChannelProvider) GetPreviousChannelIDByThread(_ context.Context, threadID int) (int, error) {
	return p.threadChannelIDs[threadID], nil
}

func stickyTestCandidate(channelID, priority int) *ChannelModelsCandidate {
	return &ChannelModelsCandidate{
		Channel: &biz.Channel{
			Channel: &ent.Channel{
				ID:   channelID,
				Name: "channel",
			},
		},
		Priority: priority,
	}
}

func TestLoadBalancedSelector_TraceStickySelection(t *testing.T) {
	trace := &ent.Trace{ID: 10, ThreadID: 20}
	thread := &ent.Thread{ID: 20}
	ctx := contexts.WithThread(contexts.WithTrace(context.Background(), trace), thread)

	t.Run("trace overrides association priority and removes duplicate fallback", func(t *testing.T) {
		candidates := []*ChannelModelsCandidate{
			stickyTestCandidate(1, 0),
			stickyTestCandidate(2, 2),
			stickyTestCandidate(3, 1),
			stickyTestCandidate(2, 4),
		}
		policy := &mockRetryPolicyProvider{policy: &biz.RetryPolicy{
			Enabled:                 true,
			MaxChannelRetries:       2,
			MaxSingleChannelRetries: 2,
			TraceStickyMode:         biz.TraceStickyPreferPreviousChannel,
		}}
		tracker := &mockSelectionTracker{}
		selector := WithTraceStickyLoadBalancedSelector(
			&staticChannelSelector{candidates: candidates},
			NewLoadBalancer(policy, tracker),
			policy,
			&fakePreviousChannelProvider{
				traceChannelIDs:  map[int]int{trace.ID: 2},
				threadChannelIDs: map[int]int{thread.ID: 3},
			},
		)

		result, err := selector.Select(ctx, &llm.Request{Model: "gpt-4"})
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Equal(t, 2, result[0].Channel.ID)
		require.True(t, result[0].TraceSticky)
		require.Equal(t, []int{1, 3}, []int{result[1].Channel.ID, result[2].Channel.ID})
		require.Equal(t, 1, tracker.selections[2])
		require.Zero(t, tracker.selections[1])
		require.Zero(t, tracker.selections[3])
	})

	t.Run("thread is used when trace has no previous channel", func(t *testing.T) {
		candidates := []*ChannelModelsCandidate{
			stickyTestCandidate(1, 0),
			stickyTestCandidate(2, 1),
			stickyTestCandidate(3, 2),
		}
		policy := &mockRetryPolicyProvider{policy: &biz.RetryPolicy{
			Enabled:           true,
			MaxChannelRetries: 2,
			TraceStickyMode:   biz.TraceStickyPreferPreviousChannel,
		}}
		selector := WithTraceStickyLoadBalancedSelector(
			&staticChannelSelector{candidates: candidates},
			NewLoadBalancer(policy, nil),
			policy,
			&fakePreviousChannelProvider{
				threadChannelIDs: map[int]int{thread.ID: 3},
			},
		)

		result, err := selector.Select(ctx, &llm.Request{Model: "gpt-4"})
		require.NoError(t, err)
		require.Equal(t, 3, result[0].Channel.ID)
		require.True(t, result[0].TraceSticky)
	})

	t.Run("disabled mode keeps normal priority ordering", func(t *testing.T) {
		candidates := []*ChannelModelsCandidate{
			stickyTestCandidate(1, 0),
			stickyTestCandidate(2, 2),
			stickyTestCandidate(3, 1),
		}
		policy := &mockRetryPolicyProvider{policy: &biz.RetryPolicy{
			Enabled:           true,
			MaxChannelRetries: 2,
			TraceStickyMode:   biz.TraceStickyDisabled,
		}}
		selector := WithTraceStickyLoadBalancedSelector(
			&staticChannelSelector{candidates: candidates},
			NewLoadBalancer(policy, nil),
			policy,
			&fakePreviousChannelProvider{
				traceChannelIDs: map[int]int{trace.ID: 2},
			},
		)

		result, err := selector.Select(ctx, &llm.Request{Model: "gpt-4"})
		require.NoError(t, err)
		require.Equal(t, []int{1, 3, 2}, []int{result[0].Channel.ID, result[1].Channel.ID, result[2].Channel.ID})
		require.False(t, result[0].TraceSticky)
	})

	t.Run("sticky candidate has no fallback when cross-channel retries are disabled", func(t *testing.T) {
		candidates := []*ChannelModelsCandidate{
			stickyTestCandidate(1, 0),
			stickyTestCandidate(2, 1),
		}
		policy := &mockRetryPolicyProvider{policy: &biz.RetryPolicy{
			Enabled:         false,
			TraceStickyMode: biz.TraceStickyPreferPreviousChannel,
		}}
		selector := WithTraceStickyLoadBalancedSelector(
			&staticChannelSelector{candidates: candidates},
			NewLoadBalancer(policy, nil),
			policy,
			&fakePreviousChannelProvider{
				traceChannelIDs: map[int]int{trace.ID: 2},
			},
		)

		result, err := selector.Select(ctx, &llm.Request{Model: "gpt-4"})
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, 2, result[0].Channel.ID)
		require.True(t, result[0].TraceSticky)
	})
}
