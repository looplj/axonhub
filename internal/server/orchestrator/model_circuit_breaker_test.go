package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type stubResponseStream struct {
	events []*llm.Response
	index  int
}

func (s *stubResponseStream) Next() bool {
	if s.index >= len(s.events) {
		return false
	}
	s.index++
	return true
}

func (s *stubResponseStream) Current() *llm.Response {
	if s.index == 0 || s.index > len(s.events) {
		return nil
	}
	return s.events[s.index-1]
}

func (s *stubResponseStream) Err() error { return nil }

func (s *stubResponseStream) Close() error { return nil }

func TestModelCircuitBreakerTracker_StreamSuccessUsesCurrentChannel(t *testing.T) {
	cb := biz.NewModelCircuitBreaker()
	ctx := context.Background()
	policy := biz.DefaultModelCircuitBreakerPolicy()
	for i := 0; i < policy.OpenThreshold; i++ {
		cb.RecordError(ctx, 42, "gpt-4", false)
	}
	require.Equal(t, biz.StateOpen, cb.GetModelCircuitBreakerStats(ctx, 42, "gpt-4").State)

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			OriginalModel: "gpt-4",
			RoutingPolicy: EffectiveRoutingPolicy{
				LoadBalancerStrategy: biz.LoadBalancerStrategyCircuitBreaker,
			},
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: &biz.Channel{
					Channel: &ent.Channel{ID: 42, Name: "primary"},
				},
			},
		},
	}
	tracker := withModelCircuitBreaker(outbound, cb).(*modelCircuitBreakerTracker)

	stream, err := tracker.OnOutboundLlmStream(ctx, &stubResponseStream{
		events: []*llm.Response{{
			Usage: &llm.Usage{CompletionTokens: 8},
		}},
	})
	require.NoError(t, err)

	require.True(t, stream.Next())
	require.NotNil(t, stream.Current())
	require.NoError(t, stream.Close())

	stats := cb.GetModelCircuitBreakerStats(ctx, 42, "gpt-4")
	require.Equal(t, biz.StateClosed, stats.State)
	require.Zero(t, stats.ConsecutiveFailures)
}

func TestModelCircuitBreakerTracker_SkipCandidateDoesNotRecordError(t *testing.T) {
	cb := biz.NewModelCircuitBreaker()
	ctx := context.Background()
	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			OriginalModel: "gpt-4",
			RoutingPolicy: EffectiveRoutingPolicy{
				LoadBalancerStrategy: biz.LoadBalancerStrategyCircuitBreaker,
			},
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: &biz.Channel{
					Channel: &ent.Channel{ID: 42, Name: "primary"},
				},
			},
		},
	}
	tracker := withModelCircuitBreaker(outbound, cb).(*modelCircuitBreakerTracker)

	// Force open so the next request is skipped instead of probed.
	policy := biz.DefaultModelCircuitBreakerPolicy()
	for i := 0; i < policy.OpenThreshold; i++ {
		cb.RecordError(ctx, 42, "gpt-4", false)
	}
	before := cb.GetModelCircuitBreakerStats(ctx, 42, "gpt-4")
	require.Equal(t, biz.StateOpen, before.State)
	failuresBefore := before.ConsecutiveFailures
	lastFailureBefore := before.LastFailureAt

	_, err := tracker.OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.ErrorIs(t, err, errSkipCandidateByCircuitBreaker)
	tracker.OnOutboundRawError(ctx, err)

	after := cb.GetModelCircuitBreakerStats(ctx, 42, "gpt-4")
	require.Equal(t, failuresBefore, after.ConsecutiveFailures)
	require.Equal(t, lastFailureBefore, after.LastFailureAt)
}

func TestModelCircuitBreakerTracker_NonCircuitBreakerSkipsStreamTracking(t *testing.T) {
	cb := biz.NewModelCircuitBreaker()
	ctx := context.Background()
	policy := biz.DefaultModelCircuitBreakerPolicy()
	for i := 0; i < policy.OpenThreshold; i++ {
		cb.RecordError(ctx, 42, "gpt-4", false)
	}
	require.Equal(t, biz.StateOpen, cb.GetModelCircuitBreakerStats(ctx, 42, "gpt-4").State)

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			OriginalModel: "gpt-4",
			RoutingPolicy: EffectiveRoutingPolicy{
				LoadBalancerStrategy: biz.LoadBalancerStrategyAdaptive,
			},
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: &biz.Channel{
					Channel: &ent.Channel{ID: 42, Name: "primary"},
				},
			},
		},
	}
	tracker := withModelCircuitBreaker(outbound, cb).(*modelCircuitBreakerTracker)

	raw := &stubResponseStream{
		events: []*llm.Response{{
			Usage: &llm.Usage{CompletionTokens: 8},
		}},
	}
	stream, err := tracker.OnOutboundLlmStream(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, streams.Stream[*llm.Response](raw), stream)

	require.True(t, stream.Next())
	require.NotNil(t, stream.Current())

	stats := cb.GetModelCircuitBreakerStats(ctx, 42, "gpt-4")
	require.Equal(t, biz.StateOpen, stats.State)
	require.Equal(t, policy.OpenThreshold, stats.ConsecutiveFailures)
}
