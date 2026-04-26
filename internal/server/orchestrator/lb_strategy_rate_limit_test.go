package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestRateLimitAwareStrategy_Score_NoRateLimit(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}

	ctx := context.Background()
	assert.Equal(t, 100.0, strategy.Score(ctx, channel))
}

func TestRateLimitAwareStrategy_Score_NoRateLimitChannelIgnoresManagerStats(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mgr := NewChannelLimiterManager()
	strategy := NewRateLimitAwareStrategy(tracker, mgr)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}

	ctx := context.Background()
	// No limiter is created for the channel because RateLimit is nil; full score expected.
	assert.Equal(t, 100.0, strategy.Score(ctx, channel))
}

func TestRateLimitAwareStrategy_Score_Cooldown(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}

	tracker.SetCooldown(channel.ID, time.Now().Add(30*time.Second))

	ctx := context.Background()
	assert.Equal(t, float64(rateLimitExhaustedScore), strategy.Score(ctx, channel))
}

func TestRateLimitAwareStrategy_Score_RPMExhausted(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	rpm := int64(100)
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{RPM: &rpm},
			},
		},
	}

	for range rpm {
		tracker.IncrementRequest(channel.ID)
	}

	ctx := context.Background()
	assert.Equal(t, float64(rateLimitExhaustedScore), strategy.Score(ctx, channel))
}

func TestRateLimitAwareStrategy_Score_TPMExhausted(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	tpm := int64(1000)
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{TPM: &tpm},
			},
		},
	}

	tracker.AddTokens(channel.ID, tpm)

	ctx := context.Background()
	assert.Equal(t, float64(rateLimitExhaustedScore), strategy.Score(ctx, channel))
}

func TestRateLimitAwareStrategy_Score_CooldownTakesPriority(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	rpm := int64(100)
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{RPM: &rpm},
			},
		},
	}

	tracker.SetCooldown(channel.ID, time.Now().Add(30*time.Second))
	tracker.IncrementRequest(channel.ID)

	ctx := context.Background()
	assert.Equal(t, float64(rateLimitExhaustedScore), strategy.Score(ctx, channel))
}

func TestRateLimitAwareStrategy_Score_NormalUsage(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	rpm := int64(100)
	tpm := int64(1000)
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{RPM: &rpm, TPM: &tpm},
			},
		},
	}

	tracker.IncrementRequest(channel.ID)
	tracker.IncrementRequest(channel.ID)
	tracker.AddTokens(channel.ID, 500)

	// maxRatio = max(2/100, 500/1000) = 0.5 -> score 50
	ctx := context.Background()
	assert.Equal(t, 50.0, strategy.Score(ctx, channel))
}

func TestRateLimitAwareStrategy_Score_SoftMode_ConcurrencyContributes(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mgr := NewChannelLimiterManager()
	strategy := NewRateLimitAwareStrategy(tracker, mgr)

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{
					MaxConcurrent: lo.ToPtr(int64(10)),
				},
			},
		},
	}

	lim := mgr.GetOrCreate(channel)
	require.NotNil(t, lim)

	// 8 / 10 in flight -> ratio 0.8 -> concurrency score 20
	for range 8 {
		require.NoError(t, lim.Acquire(t.Context()))
	}

	ctx := context.Background()
	assert.InDelta(t, 20.0, strategy.Score(ctx, channel), 0.0001)

	// Saturate to capacity -> exhausted (PR #1322 soft-mode parity)
	for range 2 {
		require.NoError(t, lim.Acquire(t.Context()))
	}
	assert.Equal(t, float64(rateLimitExhaustedScore), strategy.Score(ctx, channel))

	for range 10 {
		lim.Release()
	}
}

func TestRateLimitAwareStrategy_Score_HardMode_QueuingCappedScore(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mgr := NewChannelLimiterManager()
	strategy := NewRateLimitAwareStrategy(tracker, mgr)

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{
					MaxConcurrent: lo.ToPtr(int64(2)),
					QueueSize:     lo.ToPtr(int64(10)),
				},
			},
		},
	}

	lim := mgr.GetOrCreate(channel)
	require.NotNil(t, lim)

	// Saturate capacity (no queue yet) -> should already be at "spilled" boundary.
	require.NoError(t, lim.Acquire(t.Context()))
	require.NoError(t, lim.Acquire(t.Context()))

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	// inFlight==capacity but waiting==0 -> waitingRatio=0 -> score = 100 * 0.3 * 1 = 30
	assert.InDelta(t, 30.0, score, 0.0001)

	// Push 5 waiters into the queue. Drive blocking acquires and let them stay queued.
	waitCtx, cancel := context.WithCancel(t.Context())
	defer cancel()

	enqueued := launchAcquire(t, lim, waitCtx, 5)

	require.Eventually(t, func() bool {
		_, w := lim.Stats()
		return w == 5
	}, time.Second, 5*time.Millisecond)

	// waitingRatio = 5/10 = 0.5 -> score = 100 * 0.3 * 0.5 = 15
	assert.InDelta(t, 15.0, strategy.Score(ctx, channel), 0.0001)

	// Now fully load capacity + queue -> exhausted.
	enqueuedExtra := launchAcquire(t, lim, waitCtx, 5)
	require.Eventually(t, func() bool {
		_, w := lim.Stats()
		return w == 10
	}, time.Second, 5*time.Millisecond)

	assert.Equal(t, float64(rateLimitExhaustedScore), strategy.Score(ctx, channel))

	cancel()
	drainGrants(t, lim, enqueued)
	drainGrants(t, lim, enqueuedExtra)
	lim.Release()
	lim.Release()
}

func TestRateLimitAwareStrategy_Score_HardMode_Monotonic(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mgr := NewChannelLimiterManager()
	strategy := NewRateLimitAwareStrategy(tracker, mgr)

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{
					MaxConcurrent: lo.ToPtr(int64(10)),
					QueueSize:     lo.ToPtr(int64(10)),
				},
			},
		},
	}

	lim := mgr.GetOrCreate(channel)
	require.NotNil(t, lim)

	ctx := context.Background()

	// 9/10 below capacity: score should stay strictly above the
	// at-capacity-empty-queue score so the LB never prefers a saturated
	// channel over one with free headroom.
	for range 9 {
		require.NoError(t, lim.Acquire(t.Context()))
	}

	belowCapacityScore := strategy.Score(ctx, channel)

	// Saturate to capacity (no waiters yet).
	require.NoError(t, lim.Acquire(t.Context()))

	atCapacityScore := strategy.Score(ctx, channel)

	assert.Greater(t, belowCapacityScore, atCapacityScore,
		"below-capacity must score higher than at-capacity-with-empty-queue")

	// Sanity: at-capacity-empty-queue equals queueingScoreCeiling × maxScore = 30.
	assert.InDelta(t, 30.0, atCapacityScore, 0.0001)

	for range 10 {
		lim.Release()
	}
}

func TestRateLimitAwareStrategy_Score_StaleEntryAfterLimiterDisabled(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mgr := NewChannelLimiterManager()
	strategy := NewRateLimitAwareStrategy(tracker, mgr)

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{
					MaxConcurrent: lo.ToPtr(int64(5)),
				},
			},
		},
	}

	require.NotNil(t, mgr.GetOrCreate(channel))

	// Simulate the user clearing the rate limit between GetOrCreate (which
	// admits a request) and the next Score call (which still sees the
	// stale manager entry until the next admission triggers cleanup).
	channel.Settings.RateLimit = nil

	ctx := context.Background()

	// Must not panic and must treat the channel as unlimited.
	assert.Equal(t, 100.0, strategy.Score(ctx, channel))
}

func TestRateLimitAwareStrategy_Score_MinOfRPMAndConcurrency(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mgr := NewChannelLimiterManager()
	strategy := NewRateLimitAwareStrategy(tracker, mgr)

	rpm := int64(100)
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{
					RPM:           &rpm,
					MaxConcurrent: lo.ToPtr(int64(10)),
				},
			},
		},
	}

	// 30% RPM (score 70) and 80% inFlight (score 20) -> min = 20.
	for range 30 {
		tracker.IncrementRequest(channel.ID)
	}

	lim := mgr.GetOrCreate(channel)
	require.NotNil(t, lim)
	for range 8 {
		require.NoError(t, lim.Acquire(t.Context()))
	}

	ctx := context.Background()
	assert.InDelta(t, 20.0, strategy.Score(ctx, channel), 0.0001)

	for range 8 {
		lim.Release()
	}
}

func TestRateLimitAwareStrategy_ScoreWithDebug_Cooldown(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}

	until := time.Now().Add(30 * time.Second)
	tracker.SetCooldown(channel.ID, until)

	ctx := context.Background()
	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, float64(rateLimitExhaustedScore), score)
	assert.Equal(t, "RateLimitAware", debug.StrategyName)
	assert.Equal(t, "channel_in_cooldown", debug.Details["reason"])
	assert.Equal(t, true, debug.Details["exhausted"])

	_, hasCooldownUntil := debug.Details["cooldown_until"]
	assert.True(t, hasCooldownUntil)
}

func TestRateLimitAwareStrategy_ScoreWithDebug_RPMExhausted(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	rpm := int64(10)
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{RPM: &rpm},
			},
		},
	}

	for range rpm {
		tracker.IncrementRequest(channel.ID)
	}

	ctx := context.Background()
	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, float64(rateLimitExhaustedScore), score)
	assert.Equal(t, true, debug.Details["rpm_exhausted"])
	assert.Equal(t, rpm, debug.Details["rpm_limit"])
	assert.Equal(t, rpm, debug.Details["rpm_current"])
}

func TestRateLimitAwareStrategy_ScoreWithDebug_NoRateLimit(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}

	ctx := context.Background()
	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 100.0, score)
	assert.Equal(t, "no_rpm_tpm_configured", debug.Details["rpm_tpm_reason"])
}

func TestRateLimitAwareStrategy_Score_ExpiredCooldown(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}

	tracker.SetCooldown(channel.ID, time.Now().Add(-10*time.Second))

	ctx := context.Background()
	assert.Equal(t, 100.0, strategy.Score(ctx, channel))
	assert.False(t, tracker.IsCoolingDown(channel.ID))
}

func TestRateLimitAwareStrategy_Score_MultipleChannels(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)

	c1 := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "c1"}}
	c2 := &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "c2"}}

	tracker.SetCooldown(1, time.Now().Add(30*time.Second))

	ctx := context.Background()
	assert.Equal(t, float64(rateLimitExhaustedScore), strategy.Score(ctx, c1))
	assert.Equal(t, 100.0, strategy.Score(ctx, c2))
}

func TestRateLimitAwareStrategy_Name(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(tracker, nil)
	assert.Equal(t, "RateLimitAware", strategy.Name())
}
