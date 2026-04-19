package orchestrator

import (
	"context"
	"testing"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/stretchr/testify/assert"
)

func TestSortWithScores_SingleCandidate(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{
		RequestTracker: tracker,
	})

	lb := NewLoadBalancer(&mockRetryPolicyProvider{policy: &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 2}}, nil, strategy)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1}}
	candidates := []*ChannelModelsCandidate{
		{Channel: channel, Priority: 0},
	}

	scored := lb.SortWithScores(context.Background(), candidates, "gpt-4", false)

	assert.Len(t, scored, 1)
	assert.Equal(t, channel, scored[0].Candidate.Channel)
	assert.Equal(t, 100.0, scored[0].Score)
	assert.False(t, scored[0].HardExhausted)
}

func TestSortWithScores_SingleCandidate_HardExhausted(t *testing.T) {
	connTracker := NewDefaultConnectionTracker(1)
	connTracker.IncrementConnection(1)

	strategy := NewRateLimitAwareStrategy(RateLimitProvider{
		RequestTracker:    NewChannelRequestTracker(),
		ConnectionTracker: connTracker,
	})

	lb := NewLoadBalancer(&mockRetryPolicyProvider{policy: &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 2}}, nil, strategy)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1}}
	candidates := []*ChannelModelsCandidate{
		{Channel: channel, Priority: 0},
	}

	scored := lb.SortWithScores(context.Background(), candidates, "gpt-4", false)

	assert.Len(t, scored, 1)
	assert.True(t, scored[0].HardExhausted)
}

func TestSortWithScores_MultipleCandidates_HardExhausted(t *testing.T) {
	connTracker := NewDefaultConnectionTracker(1)
	connTracker.IncrementConnection(1)

	strategy := NewRateLimitAwareStrategy(RateLimitProvider{
		RequestTracker:    NewChannelRequestTracker(),
		ConnectionTracker: connTracker,
	})

	lb := NewLoadBalancer(&mockRetryPolicyProvider{policy: &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 2}}, nil, strategy)

	exhaustedChannel := &biz.Channel{Channel: &ent.Channel{ID: 1}}
	availableChannel := &biz.Channel{Channel: &ent.Channel{ID: 2}}
	candidates := []*ChannelModelsCandidate{
		{Channel: exhaustedChannel, Priority: 0},
		{Channel: availableChannel, Priority: 0},
	}

	scored := lb.SortWithScores(context.Background(), candidates, "gpt-4", false)

	var exhaustedFound, availableFound bool
	for _, sc := range scored {
		if sc.Candidate.Channel.ID == 1 {
			exhaustedFound = true
			assert.True(t, sc.HardExhausted)
		}
		if sc.Candidate.Channel.ID == 2 {
			availableFound = true
			assert.False(t, sc.HardExhausted)
		}
	}
	assert.True(t, exhaustedFound)
	assert.True(t, availableFound)
}

func TestSortWithScores_EmptyCandidates(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{
		RequestTracker: tracker,
	})

	lb := NewLoadBalancer(&mockRetryPolicyProvider{policy: &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 2}}, nil, strategy)

	scored := lb.SortWithScores(context.Background(), nil, "gpt-4", false)

	assert.Nil(t, scored)
}
