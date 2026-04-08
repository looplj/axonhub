package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestRateLimitAwareStrategy_Score_NoRateLimit(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	ctx := context.Background()
	score := strategy.Score(ctx, channel)

	// No rate limit configured, should return max score
	assert.Equal(t, 100.0, score)
}

func TestRateLimitAwareStrategy_Score_Cooldown(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	// Set cooldown for the channel
	tracker.SetCooldown(channel.ID, time.Now().Add(30*time.Second))

	ctx := context.Background()
	score := strategy.Score(ctx, channel)

	// Channel in cooldown, should return -1000
	assert.Equal(t, -1000.0, score)
}

func TestRateLimitAwareStrategy_Score_RPMExhausted(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	rpm := int64(100)
	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM: &rpm,
			},
		},
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	// Simulate reaching RPM limit
	for range rpm {
		tracker.IncrementRequest(channel.ID)
	}

	ctx := context.Background()
	score := strategy.Score(ctx, channel)

	// RPM exhausted, should return -1000
	assert.Equal(t, -1000.0, score)
}

func TestRateLimitAwareStrategy_Score_TPMExhausted(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	tpm := int64(1000)
	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				TPM: &tpm,
			},
		},
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	// Simulate reaching TPM limit
	tracker.AddTokens(channel.ID, tpm)

	ctx := context.Background()
	score := strategy.Score(ctx, channel)

	// TPM exhausted, should return -1000
	assert.Equal(t, -1000.0, score)
}

func TestRateLimitAwareStrategy_Score_CooldownTakesPriority(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	rpm := int64(100)
	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM: &rpm,
			},
		},
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	// Set cooldown
	tracker.SetCooldown(channel.ID, time.Now().Add(30*time.Second))

	// Also add some requests (but not exhausted)
	tracker.IncrementRequest(channel.ID)

	ctx := context.Background()
	score := strategy.Score(ctx, channel)

	// Cooldown takes priority, should return -1000
	assert.Equal(t, -1000.0, score)
}

func TestRateLimitAwareStrategy_Score_NormalUsage(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	rpm := int64(100)
	tpm := int64(1000)
	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM: &rpm,
				TPM: &tpm,
			},
		},
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	// Simulate 50% usage
	tracker.IncrementRequest(channel.ID) // 1 request
	tracker.IncrementRequest(channel.ID)
	tracker.AddTokens(channel.ID, 500) // 500 tokens

	ctx := context.Background()
	score := strategy.Score(ctx, channel)

	// Should be positive (normal usage)
	// Score = maxScore * (1 - maxRatio)
	// maxRatio = max(1/100, 500/1000) = 0.5
	// score = 100 * (1 - 0.5) = 50
	assert.Equal(t, 50.0, score)
}

func TestRateLimitAwareStrategy_ScoreWithDebug_Cooldown(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	// Set cooldown
	until := time.Now().Add(30 * time.Second)
	tracker.SetCooldown(channel.ID, until)

	ctx := context.Background()
	score, strategyScore := strategy.ScoreWithDebug(ctx, channel)

	// Should return -1000
	assert.Equal(t, -1000.0, score)
	assert.Equal(t, "RateLimitAware", strategyScore.StrategyName)

	// Check debug details
	assert.Equal(t, "channel_in_cooldown", strategyScore.Details["reason"])
	assert.Equal(t, true, strategyScore.Details["exhausted"])

	// Should have cooldown_until field
	_, hasCooldownUntil := strategyScore.Details["cooldown_until"]
	assert.True(t, hasCooldownUntil)
}

func TestRateLimitAwareStrategy_ScoreWithDebug_RPMExhausted(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	rpm := int64(10)
	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM: &rpm,
			},
		},
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	// Exhaust RPM
	for range rpm {
		tracker.IncrementRequest(channel.ID)
	}

	ctx := context.Background()
	score, strategyScore := strategy.ScoreWithDebug(ctx, channel)

	// Should return -1000
	assert.Equal(t, -1000.0, score)

	// Check debug details
	assert.Equal(t, true, strategyScore.Details["rpm_exhausted"])
	assert.Equal(t, rpm, strategyScore.Details["rpm_limit"])
	assert.Equal(t, rpm, strategyScore.Details["rpm_current"])
}

func TestRateLimitAwareStrategy_ScoreWithDebug_NoRateLimit(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	ctx := context.Background()
	score, strategyScore := strategy.ScoreWithDebug(ctx, channel)

	// Should return max score
	assert.Equal(t, 100.0, score)

	// Check debug details
	assert.Equal(t, "no_rate_limit_configured", strategyScore.Details["reason"])
}

func TestRateLimitAwareStrategy_Score_ExpiredCooldown(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	entChannel := &ent.Channel{
		ID:   1,
		Name: "test-channel",
	}
	channel := &biz.Channel{
		Channel: entChannel,
	}

	// Set cooldown in the past (expired)
	tracker.SetCooldown(channel.ID, time.Now().Add(-10*time.Second))

	ctx := context.Background()
	score := strategy.Score(ctx, channel)

	// Cooldown expired, should return max score
	assert.Equal(t, 100.0, score)

	// Verify cooldown was cleaned up
	assert.False(t, tracker.IsCoolingDown(channel.ID))
}

func TestRateLimitAwareStrategy_Score_MultipleChannels(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	entChannel1 := &ent.Channel{ID: 1, Name: "channel-1"}
	entChannel2 := &ent.Channel{ID: 2, Name: "channel-2"}

	channel1 := &biz.Channel{Channel: entChannel1}
	channel2 := &biz.Channel{Channel: entChannel2}

	// Set cooldown for channel 1 only
	tracker.SetCooldown(1, time.Now().Add(30*time.Second))

	ctx := context.Background()

	// Channel 1 should be in cooldown
	score1 := strategy.Score(ctx, channel1)
	assert.Equal(t, -1000.0, score1)

	// Channel 2 should NOT be in cooldown
	score2 := strategy.Score(ctx, channel2)
	assert.Equal(t, 100.0, score2)
}

func TestRateLimitAwareStrategy_Name(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	assert.Equal(t, "RateLimitAware", strategy.Name())
}
