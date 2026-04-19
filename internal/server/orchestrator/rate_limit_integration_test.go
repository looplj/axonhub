package orchestrator

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

// ========== Test 1: rpm=10, rpmDuration="5hr" ==========

func TestIntegration_RPM_FiveHourWindow_ExhaustsAtLimit(t *testing.T) {
	tracker := NewChannelRequestTracker()

	rpm := int64(10)
	fiveHour := objects.RateLimitDurationFiveHour
	entChannel := &ent.Channel{
		ID:   1,
		Name: "five-hour-rpm-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM:         &rpm,
				RPMDuration: &fiveHour,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	duration := fiveHour.Duration()

	for range rpm {
		tracker.IncrementRequestForDuration(channel.ID, duration, nil)
	}

	currentCount := tracker.GetRequestCountForDuration(channel.ID, duration, nil)
	assert.Equal(t, rpm, currentCount, "10 requests in 5-hour window should exhaust RPM limit")
	assert.GreaterOrEqual(t, currentCount, rpm)
}

func TestIntegration_RPM_FiveHourWindow_NotExhaustedBelowLimit(t *testing.T) {
	tracker := NewChannelRequestTracker()

	rpm := int64(10)
	fiveHour := objects.RateLimitDurationFiveHour
	entChannel := &ent.Channel{
		ID:   1,
		Name: "five-hour-rpm-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM:         &rpm,
				RPMDuration: &fiveHour,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	duration := fiveHour.Duration()

	for range 9 {
		tracker.IncrementRequestForDuration(channel.ID, duration, nil)
	}

	currentCount := tracker.GetRequestCountForDuration(channel.ID, duration, nil)
	assert.Equal(t, int64(9), currentCount)
	assert.Less(t, currentCount, rpm)
}

func TestIntegration_RPM_FiveHourWindow_WindowReset(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	fiveHour := objects.RateLimitDurationFiveHour
	duration := fiveHour.Duration()

	tracker.IncrementRequestForDuration(1, duration, nil)
	tracker.IncrementRequestForDuration(1, duration, nil)
	assert.Equal(t, int64(2), tracker.GetRequestCountForDuration(1, duration, nil))

	*clockPtr = now.Add(duration + time.Second)

	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(1, duration, nil))

	tracker.IncrementRequestForDuration(1, duration, nil)
	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(1, duration, nil))
}

// ========== Test 2: tpm=1000, tpmDuration="1mo" ==========

func TestIntegration_TPM_MonthlyWindow_TokenCounting(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tpm := int64(1000)
	oneMonth := objects.RateLimitDurationOneMonth
	entChannel := &ent.Channel{
		ID:   2,
		Name: "monthly-tpm-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				TPM:         &tpm,
				TPMDuration: &oneMonth,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	duration := oneMonth.Duration()

	tracker.AddTokensForDuration(channel.ID, 1000, duration, nil)

	currentTokens := tracker.GetTokenCountForDuration(channel.ID, duration, nil)
	assert.Equal(t, int64(1000), currentTokens)
}

func TestIntegration_TPM_MonthlyWindow_BelowLimit(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tpm := int64(1000)
	oneMonth := objects.RateLimitDurationOneMonth
	duration := oneMonth.Duration()

	tracker.AddTokensForDuration(2, 500, duration, nil)
	tracker.AddTokensForDuration(2, 200, duration, nil)

	currentTokens := tracker.GetTokenCountForDuration(2, duration, nil)
	assert.Equal(t, int64(700), currentTokens)
	assert.Less(t, currentTokens, tpm)
}

func TestIntegration_TPM_MonthlyWindow_WindowReset(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	oneMonth := objects.RateLimitDurationOneMonth
	duration := oneMonth.Duration()

	tracker.AddTokensForDuration(2, 500, duration, nil)
	tracker.AddTokensForDuration(2, 500, duration, nil)
	assert.Equal(t, int64(1000), tracker.GetTokenCountForDuration(2, duration, nil))

	*clockPtr = now.Add(duration + time.Second)

	assert.Equal(t, int64(0), tracker.GetTokenCountForDuration(2, duration, nil))
	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(2, duration, nil))

	tracker.AddTokensForDuration(2, 100, duration, nil)
	assert.Equal(t, int64(100), tracker.GetTokenCountForDuration(2, duration, nil))
}

// ========== Test 3: modelConcurrent={"gpt-4": 2} ==========

func TestIntegration_ModelConcurrent_TwoConcurrentExhausts(t *testing.T) {
	modelTracker := NewModelConnectionTracker()

	modelConcurrent := map[string]int64{
		"gpt-4": 2,
	}
	settings := &objects.ChannelRateLimit{
		ModelConcurrent: modelConcurrent,
	}

	modelTracker.IncrementModelConnection(1, "gpt-4")
	modelTracker.IncrementModelConnection(1, "gpt-4")

	currentCount := modelTracker.GetModelConnectionCount(1, "gpt-4")
	assert.Equal(t, 2, currentCount)

	limit, hasCustom := settings.GetModelConcurrentLimit("gpt-4")
	assert.Equal(t, int64(2), limit)
	assert.True(t, hasCustom)
	assert.GreaterOrEqual(t, int64(currentCount), limit)
}

func TestIntegration_ModelConcurrent_OneConcurrentNotExhausted(t *testing.T) {
	modelTracker := NewModelConnectionTracker()

	modelConcurrent := map[string]int64{
		"gpt-4": 2,
	}
	settings := &objects.ChannelRateLimit{
		ModelConcurrent: modelConcurrent,
	}

	modelTracker.IncrementModelConnection(1, "gpt-4")

	currentCount := modelTracker.GetModelConnectionCount(1, "gpt-4")
	assert.Equal(t, 1, currentCount)

	limit, hasCustom := settings.GetModelConcurrentLimit("gpt-4")
	assert.Equal(t, int64(2), limit)
	assert.True(t, hasCustom)
	assert.Less(t, int64(currentCount), limit)
}

func TestIntegration_ModelConcurrent_DifferentModelsIndependent(t *testing.T) {
	modelTracker := NewModelConnectionTracker()

	modelConcurrent := map[string]int64{
		"gpt-4":    2,
		"claude-3": 3,
	}
	settings := &objects.ChannelRateLimit{
		ModelConcurrent: modelConcurrent,
	}

	modelTracker.IncrementModelConnection(1, "gpt-4")
	modelTracker.IncrementModelConnection(1, "gpt-4")
	modelTracker.IncrementModelConnection(1, "claude-3")

	assert.Equal(t, 2, modelTracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, modelTracker.GetModelConnectionCount(1, "claude-3"))

	gpt4Limit, gpt4Custom := settings.GetModelConcurrentLimit("gpt-4")
	assert.Equal(t, int64(2), gpt4Limit)
	assert.True(t, gpt4Custom)

	claude3Limit, claude3Custom := settings.GetModelConcurrentLimit("claude-3")
	assert.Equal(t, int64(3), claude3Limit)
	assert.True(t, claude3Custom)
}

// ========== Test 4: MaxConcurrent=10 and modelConcurrent={"gpt-4": 2} ==========

func TestIntegration_MixedConcurrent_PerModelTakesPrecedenceForGPT4(t *testing.T) {
	modelTracker := NewModelConnectionTracker()

	maxConcurrent := int64(10)
	modelConcurrent := map[string]int64{
		"gpt-4": 2,
	}
	settings := &objects.ChannelRateLimit{
		MaxConcurrent:   &maxConcurrent,
		ModelConcurrent: modelConcurrent,
	}

	limit, hasCustom := settings.GetModelConcurrentLimit("gpt-4")
	assert.Equal(t, int64(2), limit, "gpt-4 should use per-model limit of 2, not MaxConcurrent of 10")
	assert.True(t, hasCustom)

	modelTracker.IncrementModelConnection(1, "gpt-4")
	modelTracker.IncrementModelConnection(1, "gpt-4")

	currentCount := modelTracker.GetModelConnectionCount(1, "gpt-4")
	assert.GreaterOrEqual(t, int64(currentCount), limit)
}

func TestIntegration_MixedConcurrent_ChannelWideForOtherModels(t *testing.T) {
	modelTracker := NewModelConnectionTracker()

	maxConcurrent := int64(10)
	modelConcurrent := map[string]int64{
		"gpt-4": 2,
	}
	settings := &objects.ChannelRateLimit{
		MaxConcurrent:   &maxConcurrent,
		ModelConcurrent: modelConcurrent,
	}

	limit, hasCustom := settings.GetModelConcurrentLimit("claude-3")
	assert.Equal(t, int64(10), limit, "claude-3 should fall back to MaxConcurrent of 10")
	assert.False(t, hasCustom)

	for range 5 {
		modelTracker.IncrementModelConnection(1, "claude-3")
	}

	currentCount := modelTracker.GetModelConnectionCount(1, "claude-3")
	assert.Less(t, int64(currentCount), limit)
}

func TestIntegration_MixedConcurrent_BothLimitsTrackedIndependently(t *testing.T) {
	modelTracker := NewModelConnectionTracker()

	maxConcurrent := int64(10)
	modelConcurrent := map[string]int64{
		"gpt-4": 2,
	}
	settings := &objects.ChannelRateLimit{
		MaxConcurrent:   &maxConcurrent,
		ModelConcurrent: modelConcurrent,
	}

	modelTracker.IncrementModelConnection(1, "gpt-4")
	modelTracker.IncrementModelConnection(1, "gpt-4")
	modelTracker.IncrementModelConnection(1, "claude-3")

	assert.Equal(t, 2, modelTracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, modelTracker.GetModelConnectionCount(1, "claude-3"))

	gpt4Limit, _ := settings.GetModelConcurrentLimit("gpt-4")
	claude3Limit, _ := settings.GetModelConcurrentLimit("claude-3")
	assert.Equal(t, int64(2), gpt4Limit)
	assert.Equal(t, int64(10), claude3Limit)
}

// ========== Test 5: rpmDuration="1hr" and tpmDuration="5hr" — independent windows ==========

func TestIntegration_IndependentRPMAndTPMDurations(t *testing.T) {
	tracker := NewChannelRequestTracker()

	rpm := int64(100)
	tpm := int64(1000)
	oneHour := objects.RateLimitDurationOneHour
	fiveHour := objects.RateLimitDurationFiveHour

	entChannel := &ent.Channel{
		ID:   5,
		Name: "mixed-duration-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM:         &rpm,
				TPM:         &tpm,
				RPMDuration: &oneHour,
				TPMDuration: &fiveHour,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	rpmDuration := oneHour.Duration()
	tpmDuration := fiveHour.Duration()

	tracker.IncrementRequestForDuration(channel.ID, rpmDuration, nil)
	tracker.IncrementRequestForDuration(channel.ID, rpmDuration, nil)

	tracker.AddTokensForDuration(channel.ID, 500, tpmDuration, nil)
	tracker.AddTokensForDuration(channel.ID, 300, tpmDuration, nil)

	rpmCount := tracker.GetRequestCountForDuration(channel.ID, rpmDuration, nil)
	tpmCount := tracker.GetTokenCountForDuration(channel.ID, tpmDuration, nil)

	assert.Equal(t, int64(2), rpmCount, "RPM should track in 1-hour window")
	assert.Equal(t, int64(800), tpmCount, "TPM should track in 5-hour window")

	rpmTokens := tracker.GetTokenCountForDuration(channel.ID, rpmDuration, nil)
	assert.Equal(t, int64(0), rpmTokens, "1-hour window should not see 5-hour TPM tokens")

	tpmRequests := tracker.GetRequestCountForDuration(channel.ID, tpmDuration, nil)
	assert.Equal(t, int64(0), tpmRequests, "5-hour window should not see 1-hour RPM requests")
}

func TestIntegration_IndependentRPMAndTPMDurations_WindowReset(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	oneHour := objects.RateLimitDurationOneHour
	fiveHour := objects.RateLimitDurationFiveHour
	rpmDuration := oneHour.Duration()
	tpmDuration := fiveHour.Duration()

	tracker.IncrementRequestForDuration(5, rpmDuration, nil)
	tracker.AddTokensForDuration(5, 800, tpmDuration, nil)

	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(5, rpmDuration, nil))
	assert.Equal(t, int64(800), tracker.GetTokenCountForDuration(5, tpmDuration, nil))

	*clockPtr = now.Add(rpmDuration + time.Second)

	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(5, rpmDuration, nil))
	assert.Equal(t, int64(800), tracker.GetTokenCountForDuration(5, tpmDuration, nil))

	*clockPtr = now.Add(tpmDuration + time.Second)

	assert.Equal(t, int64(0), tracker.GetTokenCountForDuration(5, tpmDuration, nil))

	tracker.IncrementRequestForDuration(5, rpmDuration, nil)
	tracker.AddTokensForDuration(5, 100, tpmDuration, nil)

	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(5, rpmDuration, nil))
	assert.Equal(t, int64(100), tracker.GetTokenCountForDuration(5, tpmDuration, nil))
	assert.Equal(t, int64(0), tracker.GetTokenCountForDuration(5, rpmDuration, nil))
	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(5, tpmDuration, nil))
}

// ========== Test 6: Backward compatibility — rpm=100 with 1-minute default ==========

func TestIntegration_BackwardCompatibility_RPMOnly_DefaultOneMinuteWindow(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpm := int64(100)
	entChannel := &ent.Channel{
		ID:   6,
		Name: "legacy-rpm-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM: &rpm,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	rl := entChannel.Settings.RateLimit
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetRPMDuration())

	for range 50 {
		tracker.IncrementRequest(channel.ID)
	}

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	// 50/100 = 0.5 ratio → score = 100 * (1 - 0.5) = 50
	assert.Equal(t, 50.0, score)
}

func TestIntegration_BackwardCompatibility_RPMExhausted_DefaultOneMinuteWindow(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpm := int64(100)
	entChannel := &ent.Channel{
		ID:   6,
		Name: "legacy-rpm-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM: &rpm,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	for range rpm {
		tracker.IncrementRequest(channel.ID)
	}

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	assert.Equal(t, float64(rateLimitExhaustedScore), score)
}

func TestIntegration_BackwardCompatibility_TPMOnly_DefaultOneMinuteWindow(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	tpm := int64(1000)
	entChannel := &ent.Channel{
		ID:   6,
		Name: "legacy-tpm-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				TPM: &tpm,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	rl := entChannel.Settings.RateLimit
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetTPMDuration())

	tracker.AddTokens(channel.ID, 500)

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	// 500/1000 = 0.5 ratio → score = 100 * (1 - 0.5) = 50
	assert.Equal(t, 50.0, score)
}

func TestIntegration_BackwardCompatibility_NoDurationFields(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpm := int64(100)
	tpm := int64(1000)
	entChannel := &ent.Channel{
		ID:   6,
		Name: "legacy-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM: &rpm,
				TPM: &tpm,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	tracker.IncrementRequest(channel.ID)
	tracker.IncrementRequest(channel.ID)
	tracker.AddTokens(channel.ID, 500)

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	// maxRatio = max(2/100, 500/1000) = 0.5 → score = 100 * (1 - 0.5) = 50
	assert.Equal(t, 50.0, score)
}

// ========== Test 7: Load balancer scoring with mixed duration configs ==========

func TestIntegration_LBScoring_MixedDurationConfigs(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpmA := int64(10)
	fiveHour := objects.RateLimitDurationFiveHour
	entChannelA := &ent.Channel{
		ID:   10,
		Name: "channel-a-5hr",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM:         &rpmA,
				RPMDuration: &fiveHour,
			},
		},
	}
	channelA := &biz.Channel{Channel: entChannelA}

	rpmB := int64(100)
	entChannelB := &ent.Channel{
		ID:   11,
		Name: "channel-b-1min",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM: &rpmB,
			},
		},
	}
	channelB := &biz.Channel{Channel: entChannelB}

	entChannelC := &ent.Channel{
		ID:   12,
		Name: "channel-c-unlimited",
	}
	channelC := &biz.Channel{Channel: entChannelC}

	rpmDurationA := fiveHour.Duration()
	for range 5 {
		tracker.IncrementRequestForDuration(channelA.ID, rpmDurationA, nil)
	}

	for range 50 {
		tracker.IncrementRequest(channelB.ID)
	}

	ctx := context.Background()

	scoreA := strategy.Score(ctx, channelA)
	scoreB := strategy.Score(ctx, channelB)
	scoreC := strategy.Score(ctx, channelC)

	assert.Equal(t, 100.0, scoreC, "unlimited channel should get max score")
	// 50/100 = 0.5 → score = 100 * (1 - 0.5) = 50
	assert.Equal(t, 50.0, scoreB, "channel B at 50% usage should get score of 50")
	// Strategy uses 5-hour window for channel A, sees 5/10 requests = 50% usage → score = 50
	assert.Equal(t, 50.0, scoreA, "channel A: strategy uses 5-hour window, sees 5/10 requests")

	fiveHourCount := tracker.GetRequestCountForDuration(channelA.ID, rpmDurationA, nil)
	assert.Equal(t, int64(5), fiveHourCount, "tracker should correctly track 5-hour window requests")
}

func TestIntegration_LBScoring_MixedDurationConfigs_WithDebug(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpm := int64(10)
	fiveHour := objects.RateLimitDurationFiveHour
	entChannel := &ent.Channel{
		ID:   10,
		Name: "channel-5hr",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM:         &rpm,
				RPMDuration: &fiveHour,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	rpmDuration := fiveHour.Duration()
	for range 5 {
		tracker.IncrementRequestForDuration(channel.ID, rpmDuration, nil)
	}

	ctx := context.Background()
	score, strategyScore := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 50.0, score)
	assert.Equal(t, "RateLimitAware", strategyScore.StrategyName)
	assert.Equal(t, int64(10), strategyScore.Details["rpm_limit"])
	assert.Equal(t, int64(5), strategyScore.Details["rpm_current"])
}

// ========== Test 8: Concurrent access safety ==========

func TestIntegration_ConcurrentAccess_RPMWithCustomDuration(t *testing.T) {
	tracker := NewChannelRequestTracker()

	fiveHour := objects.RateLimitDurationFiveHour
	duration := fiveHour.Duration()

	const (
		goroutines      = 100
		opsPerGoroutine = 100
	)

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range opsPerGoroutine {
				tracker.IncrementRequestForDuration(1, duration, nil)
			}
		}()
	}

	for range goroutines {
		go func() {
			defer wg.Done()
			for range opsPerGoroutine {
				tracker.AddTokensForDuration(1, 10, duration, nil)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(goroutines*opsPerGoroutine), tracker.GetRequestCountForDuration(1, duration, nil))
	assert.Equal(t, int64(goroutines*opsPerGoroutine*10), tracker.GetTokenCountForDuration(1, duration, nil))
}

func TestIntegration_ConcurrentAccess_MultipleDurations(t *testing.T) {
	tracker := NewChannelRequestTracker()

	oneHour := objects.RateLimitDurationOneHour
	fiveHour := objects.RateLimitDurationFiveHour
	oneMonth := objects.RateLimitDurationOneMonth

	durations := []time.Duration{
		oneHour.Duration(),
		fiveHour.Duration(),
		oneMonth.Duration(),
	}

	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(len(durations) * 2)

	for _, d := range durations {
		go func(duration time.Duration) {
			defer wg.Done()
			for range opsPerGoroutine {
				tracker.IncrementRequestForDuration(1, duration, nil)
			}
		}(d)

		go func(duration time.Duration) {
			defer wg.Done()
			for range opsPerGoroutine {
				tracker.AddTokensForDuration(1, 10, duration, nil)
			}
		}(d)
	}

	wg.Wait()

	for _, d := range durations {
		assert.Equal(t, int64(opsPerGoroutine), tracker.GetRequestCountForDuration(1, d, nil),
			"request count mismatch for duration %v", d)
		assert.Equal(t, int64(opsPerGoroutine*10), tracker.GetTokenCountForDuration(1, d, nil),
			"token count mismatch for duration %v", d)
	}
}

func TestIntegration_ConcurrentAccess_ReadWriteWithCustomDurations(t *testing.T) {
	tracker := NewChannelRequestTracker()

	fiveHour := objects.RateLimitDurationFiveHour
	duration := fiveHour.Duration()

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for range 1000 {
			tracker.IncrementRequestForDuration(1, duration, nil)
		}
	}()

	go func() {
		defer wg.Done()
		for range 1000 {
			_ = tracker.GetRequestCountForDuration(1, duration, nil)
		}
	}()

	go func() {
		defer wg.Done()
		for range 1000 {
			_ = tracker.GetTokenCountForDuration(1, duration, nil)
		}
	}()

	wg.Wait()

	assert.Equal(t, int64(1000), tracker.GetRequestCountForDuration(1, duration, nil))
}

func TestIntegration_ConcurrentAccess_ModelConnectionTracker(t *testing.T) {
	modelTracker := NewModelConnectionTracker()

	const (
		goroutines      = 50
		opsPerGoroutine = 20
	)

	var wg sync.WaitGroup

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range opsPerGoroutine {
				modelTracker.IncrementModelConnection(1, "gpt-4")
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, goroutines*opsPerGoroutine, modelTracker.GetModelConnectionCount(1, "gpt-4"))

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range opsPerGoroutine / 2 {
				modelTracker.DecrementModelConnection(1, "gpt-4")
			}
		}()
	}
	wg.Wait()

	expectedCount := goroutines*opsPerGoroutine - goroutines*(opsPerGoroutine/2)
	assert.Equal(t, expectedCount, modelTracker.GetModelConnectionCount(1, "gpt-4"))
}

func TestIntegration_ConcurrentAccess_MixedTrackersAndStrategy(t *testing.T) {
	tracker := NewChannelRequestTracker()
	connectionTracker := NewDefaultConnectionTracker(10)
	modelTracker := NewModelConnectionTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker, ConnectionTracker: connectionTracker, ModelConnTracker: modelTracker})

	rpm := int64(100)
	tpm := int64(1000)
	maxConcurrent := int64(10)
	modelConcurrent := map[string]int64{
		"gpt-4": 2,
	}
	entChannel := &ent.Channel{
		ID:   1,
		Name: "concurrent-test-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM:             &rpm,
				TPM:             &tpm,
				MaxConcurrent:   &maxConcurrent,
				ModelConcurrent: modelConcurrent,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	const ops = 500

	var wg sync.WaitGroup
	wg.Add(ops * 4)

	for range ops {
		go func() {
			defer wg.Done()
			tracker.IncrementRequest(1)
		}()
	}

	for range ops {
		go func() {
			defer wg.Done()
			tracker.AddTokens(1, 1)
		}()
	}

	for range ops {
		go func() {
			defer wg.Done()
			connectionTracker.IncrementConnection(1)
		}()
	}

	for range ops {
		go func() {
			defer wg.Done()
			modelTracker.IncrementModelConnection(1, "gpt-4")
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(ops), tracker.GetRequestCount(1))
	assert.Equal(t, int64(ops), tracker.GetTokenCount(1))
	assert.Equal(t, ops, connectionTracker.GetActiveConnections(1))
	assert.Equal(t, ops, modelTracker.GetModelConnectionCount(1, "gpt-4"))

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	assert.Equal(t, float64(rateLimitExhaustedScore), score)
}

// ========== Duration parsing and defaults ==========

func TestIntegration_RateLimitDuration_Parsing(t *testing.T) {
	tests := []struct {
		duration objects.RateLimitDuration
		expected time.Duration
	}{
		{objects.RateLimitDurationOneMin, time.Minute},
		{objects.RateLimitDurationOneHour, time.Hour},
		{objects.RateLimitDurationFiveHour, 5 * time.Hour},
		{objects.RateLimitDurationOneWeek, 7 * 24 * time.Hour},
		{objects.RateLimitDurationOneMonth, 30 * 24 * time.Hour},
		{objects.RateLimitDuration("unknown"), time.Minute},
	}

	for _, tt := range tests {
		t.Run(string(tt.duration), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.duration.Duration())
		})
	}
}

func TestIntegration_GetRPMDuration_Default(t *testing.T) {
	var rl *objects.ChannelRateLimit
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetRPMDuration())
	assert.Equal(t, objects.RateLimitDurationOneMin, rl.GetTPMDuration())

	rl2 := &objects.ChannelRateLimit{}
	assert.Equal(t, objects.RateLimitDurationOneMin, rl2.GetRPMDuration())
	assert.Equal(t, objects.RateLimitDurationOneMin, rl2.GetTPMDuration())
}

func TestIntegration_GetRPMDuration_Configured(t *testing.T) {
	fiveHour := objects.RateLimitDurationFiveHour
	oneMonth := objects.RateLimitDurationOneMonth

	rl := &objects.ChannelRateLimit{
		RPMDuration: &fiveHour,
		TPMDuration: &oneMonth,
	}

	assert.Equal(t, objects.RateLimitDurationFiveHour, rl.GetRPMDuration())
	assert.Equal(t, objects.RateLimitDurationOneMonth, rl.GetTPMDuration())
}

// ========== getRPMDuration/getTPMDuration helpers ==========

func TestIntegration_getRPMDuration_Helper(t *testing.T) {
	entChannel1 := &ent.Channel{ID: 1, Name: "no-settings"}
	channel1 := &biz.Channel{Channel: entChannel1}
	assert.Equal(t, time.Minute, getRPMDuration(channel1))
	assert.Equal(t, time.Minute, getTPMDuration(channel1))

	entChannel2 := &ent.Channel{
		ID:       2,
		Name:     "no-rate-limit",
		Settings: &objects.ChannelSettings{},
	}
	channel2 := &biz.Channel{Channel: entChannel2}
	assert.Equal(t, time.Minute, getRPMDuration(channel2))
	assert.Equal(t, time.Minute, getTPMDuration(channel2))

	fiveHour := objects.RateLimitDurationFiveHour
	oneMonth := objects.RateLimitDurationOneMonth
	entChannel3 := &ent.Channel{
		ID:   3,
		Name: "custom-durations",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPMDuration: &fiveHour,
				TPMDuration: &oneMonth,
			},
		},
	}
	channel3 := &biz.Channel{Channel: entChannel3}
	assert.Equal(t, 5*time.Hour, getRPMDuration(channel3))
	assert.Equal(t, 30*24*time.Hour, getTPMDuration(channel3))
}

// ========== End-to-end model concurrent + channel-wide limits ==========

func TestIntegration_ModelConcurrent_WithChannelWideLimit(t *testing.T) {
	modelTracker := NewModelConnectionTracker()
	connectionTracker := NewDefaultConnectionTracker(10)

	maxConcurrent := int64(10)
	modelConcurrent := map[string]int64{
		"gpt-4":    2,
		"claude-3": 5,
	}
	settings := &objects.ChannelRateLimit{
		MaxConcurrent:   &maxConcurrent,
		ModelConcurrent: modelConcurrent,
	}

	gpt4Limit, gpt4Custom := settings.GetModelConcurrentLimit("gpt-4")
	assert.Equal(t, int64(2), gpt4Limit)
	assert.True(t, gpt4Custom)

	claude3Limit, claude3Custom := settings.GetModelConcurrentLimit("claude-3")
	assert.Equal(t, int64(5), claude3Limit)
	assert.True(t, claude3Custom)

	otherLimit, otherCustom := settings.GetModelConcurrentLimit("other-model")
	assert.Equal(t, int64(10), otherLimit)
	assert.False(t, otherCustom)

	modelTracker.IncrementModelConnection(1, "gpt-4")
	modelTracker.IncrementModelConnection(1, "gpt-4")

	modelTracker.IncrementModelConnection(1, "claude-3")
	modelTracker.IncrementModelConnection(1, "claude-3")
	modelTracker.IncrementModelConnection(1, "claude-3")

	modelTracker.IncrementModelConnection(1, "other-model")

	for range 6 {
		connectionTracker.IncrementConnection(1)
	}

	assert.Equal(t, 6, connectionTracker.GetActiveConnections(1))
	assert.Equal(t, 2, modelTracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 3, modelTracker.GetModelConnectionCount(1, "claude-3"))
	assert.Equal(t, 1, modelTracker.GetModelConnectionCount(1, "other-model"))
}

// ========== Cooldown / 429 integration tests ==========

func TestIntegration_Cooldown_ChannelExhaustedDuringCooldown(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})
	rpm := int64(100)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "cooldown-test",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm}},
	}}

	// Channel should score normally before cooldown
	score := strategy.Score(context.Background(), ch)
	assert.Equal(t, 100.0, score)

	// Set cooldown
	until := time.Now().Add(30 * time.Second)
	tracker.SetCooldown(ch.ID, until)

	// Channel should be exhausted during cooldown
	score = strategy.Score(context.Background(), ch)
	assert.Equal(t, float64(rateLimitExhaustedScore), score)

	// IsCoolingDown should return true
	assert.True(t, tracker.IsCoolingDown(ch.ID))

	// GetCooldownUntil should return the set time
	gotUntil, ok := tracker.GetCooldownUntil(ch.ID)
	assert.True(t, ok)
	assert.Equal(t, until, gotUntil)
}

func TestIntegration_Cooldown_CooldownExpiration(t *testing.T) {
	tracker := NewChannelRequestTracker()
	rpm := int64(100)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "cooldown-expiry",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm}},
	}}

	// Set cooldown that expires immediately
	tracker.SetCooldown(ch.ID, time.Now().Add(-1*time.Second))

	// Channel should NOT be cooling down after expiration
	assert.False(t, tracker.IsCoolingDown(ch.ID))

	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})
	score := strategy.Score(context.Background(), ch)
	assert.Equal(t, 100.0, score, "Channel should score normally after cooldown expires")
}

func TestIntegration_Cooldown_SetCooldownMonotonic(t *testing.T) {
	tracker := NewChannelRequestTracker()

	// Set a long cooldown
	longUntil := time.Now().Add(1 * time.Hour)
	tracker.SetCooldown(1, longUntil)

	// Try to set a shorter cooldown — should not overwrite
	shortUntil := time.Now().Add(30 * time.Second)
	tracker.SetCooldown(1, shortUntil)

	gotUntil, ok := tracker.GetCooldownUntil(1)
	assert.True(t, ok)
	assert.Equal(t, longUntil, gotUntil, "Shorter cooldown should not overwrite longer one")

	// Set a longer cooldown — should overwrite
	longerUntil := time.Now().Add(2 * time.Hour)
	tracker.SetCooldown(1, longerUntil)

	gotUntil, ok = tracker.GetCooldownUntil(1)
	assert.True(t, ok)
	assert.Equal(t, longerUntil, gotUntil, "Longer cooldown should overwrite shorter one")
}

func TestIntegration_Cooldown_ScoreWithDebug(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})
	rpm := int64(100)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "cooldown-debug",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm}},
	}}

	tracker.SetCooldown(ch.ID, time.Now().Add(30*time.Second))

	score, ss := strategy.ScoreWithDebug(context.Background(), ch)
	assert.Equal(t, float64(rateLimitExhaustedScore), score)
	assert.Equal(t, true, ss.Details["exhausted"])
	assert.Equal(t, "channel_in_cooldown", ss.Details["reason"])
}

// ========== Model-in-context Score() integration tests ==========

func TestIntegration_ModelConcurrent_ScoreWithModelContext(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mt := NewModelConnectionTracker()
	ct := NewDefaultConnectionTracker(10)
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker, ConnectionTracker: ct, ModelConnTracker: mt})
	maxConcurrent := int64(10)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "model-ctx-score",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{
			MaxConcurrent:   &maxConcurrent,
			ModelConcurrent: map[string]int64{"gpt-4": 2},
		}},
	}}

	// Without model context, per-model limit is not enforced
	score := strategy.Score(context.Background(), ch)
	assert.Greater(t, score, float64(rateLimitExhaustedScore))

	// With model context and under limit
	ctx := contextWithRequestedModel(context.Background(), "gpt-4")
	score = strategy.Score(ctx, ch)
	assert.Greater(t, score, float64(rateLimitExhaustedScore))

	// With model context and at limit
	mt.IncrementModelConnection(1, "gpt-4")
	mt.IncrementModelConnection(1, "gpt-4")
	score = strategy.Score(ctx, ch)
	assert.Equal(t, float64(rateLimitExhaustedScore), score, "Should be exhausted when per-model limit reached")

	// Other model should still be allowed (falls back to MaxConcurrent)
	otherCtx := contextWithRequestedModel(context.Background(), "claude-3")
	score = strategy.Score(otherCtx, ch)
	assert.Greater(t, score, float64(rateLimitExhaustedScore))
}

func TestIntegration_ModelConcurrent_ScoreWithDebugModelContext(t *testing.T) {
	tracker := NewChannelRequestTracker()
	mt := NewModelConnectionTracker()
	ct := NewDefaultConnectionTracker(10)
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker, ConnectionTracker: ct, ModelConnTracker: mt})
	maxConcurrent := int64(10)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "model-ctx-debug",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{
			MaxConcurrent:   &maxConcurrent,
			ModelConcurrent: map[string]int64{"gpt-4": 2},
		}},
	}}

	mt.IncrementModelConnection(1, "gpt-4")

	ctx := contextWithRequestedModel(context.Background(), "gpt-4")
	score, ss := strategy.ScoreWithDebug(ctx, ch)
	assert.Greater(t, score, float64(rateLimitExhaustedScore))
	assert.Equal(t, int64(2), ss.Details["model_concurrent_limit"])
	assert.Equal(t, int64(1), ss.Details["model_concurrent_current"])
	assert.Equal(t, "gpt-4", ss.Details["model_concurrent_model"])
}

func TestIntegration_Anchor_RPMWithAnchor(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpm := int64(10)
	fiveHour := objects.RateLimitDurationFiveHour
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	entChannel := &ent.Channel{
		ID:   1,
		Name: "anchored-rpm-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM:             &rpm,
				RPMDuration:     &fiveHour,
				RPMWindowAnchor: &anchor,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	duration := fiveHour.Duration()
	for range 5 {
		tracker.IncrementRequestForDuration(channel.ID, duration, &anchor)
	}

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	assert.Equal(t, 50.0, score)
}

func TestIntegration_Anchor_RPMExhaustedWithAnchor(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpm := int64(5)
	fiveHour := objects.RateLimitDurationFiveHour
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	entChannel := &ent.Channel{
		ID:   1,
		Name: "anchored-exhausted-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM:             &rpm,
				RPMDuration:     &fiveHour,
				RPMWindowAnchor: &anchor,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	duration := fiveHour.Duration()
	for range rpm {
		tracker.IncrementRequestForDuration(channel.ID, duration, &anchor)
	}

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	assert.Equal(t, float64(rateLimitExhaustedScore), score)
}

func TestIntegration_Anchor_TPMWithAnchor(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	tpm := int64(1000)
	oneHour := objects.RateLimitDurationOneHour
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	entChannel := &ent.Channel{
		ID:   1,
		Name: "anchored-tpm-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				TPM:             &tpm,
				TPMDuration:     &oneHour,
				TPMWindowAnchor: &anchor,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	duration := oneHour.Duration()
	tracker.AddTokensForDuration(channel.ID, 500, duration, &anchor)

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	assert.Equal(t, 50.0, score)
}

func TestIntegration_Anchor_CostWithAnchor(t *testing.T) {
	tracker := NewChannelRequestTracker()
	costTracker := NewChannelCostTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker, CostTracker: costTracker})

	cost := decimal.NewFromFloat(10.0)
	oneWeek := objects.RateLimitDurationOneWeek
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	entChannel := &ent.Channel{
		ID:   1,
		Name: "anchored-cost-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				Cost:             &cost,
				CostDuration:     &oneWeek,
				CostWindowAnchor: &anchor,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	windowStart := objects.ComputeWindowStart(time.Now(), oneWeek.Duration(), &anchor)
	windowEnd := windowStart.Add(oneWeek.Duration())

	costTracker.SetCachedCost(channel.ID, decimal.NewFromFloat(5.0), windowEnd)

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	assert.Equal(t, 50.0, score)
}

func TestIntegration_Anchor_MixedAnchors(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpm := int64(100)
	tpm := int64(1000)
	oneHour := objects.RateLimitDurationOneHour
	fiveHour := objects.RateLimitDurationFiveHour
	rpmAnchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	tpmAnchor := time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC)

	entChannel := &ent.Channel{
		ID:   1,
		Name: "mixed-anchor-channel",
		Settings: &objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{
				RPM:             &rpm,
				TPM:             &tpm,
				RPMDuration:     &oneHour,
				TPMDuration:     &fiveHour,
				RPMWindowAnchor: &rpmAnchor,
				TPMWindowAnchor: &tpmAnchor,
			},
		},
	}
	channel := &biz.Channel{Channel: entChannel}

	tracker.IncrementRequestForDuration(channel.ID, oneHour.Duration(), &rpmAnchor)
	tracker.AddTokensForDuration(channel.ID, 500, fiveHour.Duration(), &tpmAnchor)

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	assert.Equal(t, 50.0, score)
}

func TestIntegration_ZeroLimitsMeanNoLimit(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})
	zeroRPM := int64(0)
	zeroTPM := int64(0)
	zeroConcurrent := int64(0)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "zero-limit",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{
			RPM: &zeroRPM, TPM: &zeroTPM, MaxConcurrent: &zeroConcurrent,
		}},
	}}
	tracker.IncrementRequest(1)
	tracker.AddTokens(1, 1000)
	score := strategy.Score(context.Background(), ch)
	assert.Equal(t, 100.0, score)
}

func TestIntegration_NilRateLimitPointer(t *testing.T) {
	tracker := NewChannelRequestTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "nil-rl",
		Settings: &objects.ChannelSettings{RateLimit: nil},
	}}
	score := strategy.Score(context.Background(), ch)
	assert.Equal(t, 100.0, score)
}

func TestIntegration_ScoreRecoveryAfterDecrement(t *testing.T) {
	tracker := NewChannelRequestTracker()
	ct := NewDefaultConnectionTracker(10)
	mt := NewModelConnectionTracker()
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker, ConnectionTracker: ct, ModelConnTracker: mt})
	maxConcurrent := int64(2)
	modelConcurrent := map[string]int64{"gpt-4": 1}
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "recovery-test",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{
			MaxConcurrent: &maxConcurrent, ModelConcurrent: modelConcurrent,
		}},
	}}

	ctx := contextWithRequestedModel(context.Background(), "gpt-4")

	mt.IncrementModelConnection(1, "gpt-4")
	score := strategy.Score(ctx, ch)
	assert.Equal(t, float64(rateLimitExhaustedScore), score)

	mt.DecrementModelConnection(1, "gpt-4")
	score = strategy.Score(ctx, ch)
	assert.Greater(t, score, float64(rateLimitExhaustedScore))

	ct.IncrementConnection(1)
	ct.IncrementConnection(1)
	score = strategy.Score(context.Background(), ch)
	assert.Equal(t, float64(rateLimitExhaustedScore), score)

	ct.DecrementConnection(1)
	score = strategy.Score(context.Background(), ch)
	assert.Greater(t, score, float64(rateLimitExhaustedScore))
}

func TestIntegration_AddTokensForDuration_OverflowIsDetectable(t *testing.T) {
	tracker := NewChannelRequestTracker()
	tracker.AddTokensForDuration(1, math.MaxInt64, time.Minute, nil)
	tracker.AddTokensForDuration(1, 1, time.Minute, nil)
	count := tracker.GetTokenCountForDuration(1, time.Minute, nil)
	assert.Equal(t, int64(math.MaxInt64), count)
}

// ========== Rate limit tracking middleware with duration-aware paths ==========

func TestIntegration_RateLimitTracking_MiddlewareWithCustomDuration(t *testing.T) {
	// This test verifies that the middleware correctly tracks requests
	// when a channel has a custom RPM duration (e.g., ONE_HOUR).
	tracker := NewChannelRequestTracker()

	// Create channel with hourly RPM duration
	hourDuration := objects.RateLimitDurationOneHour
	entCh := &ent.Channel{ID: 1, Name: "test-hourly-channel", Settings: &objects.ChannelSettings{
		RateLimit: &objects.ChannelRateLimit{
			RPM:         lo.ToPtr(int64(100)),
			RPMDuration: &hourDuration,
		},
	}}
	channel := &biz.Channel{Channel: entCh}

	// Set up state with channel
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	// Create middleware with tracker
	middleware := withRateLimitTracking(outbound, tracker)

	ctx := context.Background()
	req := &httpclient.Request{}
	_, err := middleware.OnOutboundRawRequest(ctx, req)
	require.NoError(t, err)

	// Verify count is tracked under hourly window
	count := tracker.GetRequestCountForDuration(1, time.Hour, nil)
	assert.Equal(t, int64(1), count, "Request should be tracked under hourly window")

	// Verify count is NOT tracked under default 1-minute window
	minuteCount := tracker.GetRequestCountForDuration(1, time.Minute, nil)
	assert.Equal(t, int64(0), minuteCount, "Request should NOT be tracked under 1-minute window")
}

func TestIntegration_RateLimitTracking_MiddlewareWithNoChannel(t *testing.T) {
	// Test that middleware handles nil channel gracefully
	tracker := NewChannelRequestTracker()

	// Set up state with no channel
	state := &PersistenceState{
		CurrentCandidate: nil,
	}
	outbound := &PersistentOutboundTransformer{state: state}

	// Create middleware with tracker
	middleware := withRateLimitTracking(outbound, tracker)

	ctx := context.Background()
	req := &httpclient.Request{}
	returnedReq, err := middleware.OnOutboundRawRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, req, returnedReq)

	// Verify no requests were tracked
	count := tracker.GetRequestCountForDuration(1, time.Minute, nil)
	assert.Equal(t, int64(0), count)
}

// ========== Cost-based rate limiting with nil quota service ==========

func TestIntegration_Cost_NilQuotaService_FailOpenBehavior(t *testing.T) {
	// When quotaService is nil, cost-based rate limiting should be skipped
	// (no enforcement, score returns maxScore)
	tracker := NewChannelRequestTracker()
	costTracker := NewChannelCostTracker()
	// Note: quotaService is nil here - this is the key behavior being tested
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker, CostTracker: costTracker})

	costLimit := decimal.NewFromFloat(100.0)
	entCh := &ent.Channel{ID: 1, Name: "cost-limit-channel", Settings: &objects.ChannelSettings{
		RateLimit: &objects.ChannelRateLimit{
			Cost: &costLimit,
		},
	}}
	ch := &biz.Channel{Channel: entCh}

	ctx := context.Background()
	score := strategy.Score(ctx, ch)
	// With no quota service and no cached cost, cost limit is not enforced
	// The max score (100.0) should be returned (fail-open behavior)
	assert.Equal(t, 100.0, score, "With nil quota service and no cached cost, score should be max (fail-open)")

	// Verify that cost-based scoring returns max when there's no cost data
	_, debugInfo := strategy.ScoreWithDebug(ctx, ch)
	assert.Equal(t, "RateLimitAware", debugInfo.StrategyName)
}

// ========== Cooldown tests with injectable clock ==========

func TestIntegration_Cooldown_ChannelExhaustedDuringCooldown_WithClock(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpm := int64(100)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "cooldown-test-clock",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm}},
	}}

	// Channel should score normally before cooldown
	score := strategy.Score(context.Background(), ch)
	assert.Equal(t, 100.0, score)

	// Set cooldown using the injected clock time
	*clockPtr = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	until := clockPtr.Add(30 * time.Second)
	tracker.SetCooldown(ch.ID, until)

	// Channel should be exhausted during cooldown
	score = strategy.Score(context.Background(), ch)
	assert.Equal(t, float64(rateLimitExhaustedScore), score)

	// IsCoolingDown should return true
	assert.True(t, tracker.IsCoolingDown(ch.ID))

	// GetCooldownUntil should return the set time
	gotUntil, ok := tracker.GetCooldownUntil(ch.ID)
	assert.True(t, ok)
	assert.Equal(t, until, gotUntil)
}

func TestIntegration_Cooldown_CooldownExpiration_WithClock(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	rpm := int64(100)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "cooldown-expiry-clock",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm}},
	}}

	// Set cooldown that expires immediately (in the past relative to clock)
	until := now.Add(-1 * time.Second)
	tracker.SetCooldown(ch.ID, until)

	// Channel should NOT be cooling down after expiration
	assert.False(t, tracker.IsCoolingDown(ch.ID))

	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})
	score := strategy.Score(context.Background(), ch)
	assert.Equal(t, 100.0, score, "Channel should score normally after cooldown expires")
}

func TestIntegration_Cooldown_SetCooldownMonotonic_WithClock(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	// Set a long cooldown
	*clockPtr = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	longUntil := clockPtr.Add(1 * time.Hour)
	tracker.SetCooldown(1, longUntil)

	// Try to set a shorter cooldown — should not overwrite
	*clockPtr = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	shortUntil := clockPtr.Add(30 * time.Second)
	tracker.SetCooldown(1, shortUntil)

	gotUntil, ok := tracker.GetCooldownUntil(1)
	assert.True(t, ok)
	assert.Equal(t, longUntil, gotUntil, "Shorter cooldown should not overwrite longer one")

	// Set a longer cooldown — should overwrite
	*clockPtr = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	longerUntil := clockPtr.Add(2 * time.Hour)
	tracker.SetCooldown(1, longerUntil)

	gotUntil, ok = tracker.GetCooldownUntil(1)
	assert.True(t, ok)
	assert.Equal(t, longerUntil, gotUntil, "Longer cooldown should overwrite shorter one")
}

func TestIntegration_Cooldown_ScoreWithDebug_WithClock(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)
	strategy := NewRateLimitAwareStrategy(RateLimitProvider{RequestTracker: tracker})

	rpm := int64(100)
	ch := &biz.Channel{Channel: &ent.Channel{
		ID: 1, Name: "cooldown-debug-clock",
		Settings: &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm}},
	}}

	*clockPtr = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker.SetCooldown(ch.ID, clockPtr.Add(30*time.Second))

	score, ss := strategy.ScoreWithDebug(context.Background(), ch)
	assert.Equal(t, float64(rateLimitExhaustedScore), score)
	assert.Equal(t, true, ss.Details["exhausted"])
	assert.Equal(t, "channel_in_cooldown", ss.Details["reason"])
}
