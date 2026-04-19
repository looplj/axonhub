package orchestrator

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTrackerWithClock(now time.Time) (*ChannelRequestTracker, *time.Time) {
	clockPtr := &now
	tracker := NewChannelRequestTracker(WithClock(func() time.Time { return *clockPtr }))
	return tracker, clockPtr
}

func TestNewChannelRequestTracker(t *testing.T) {
	tracker := NewChannelRequestTracker()
	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.counters)
}

func TestChannelRequestTracker_IncrementRequest(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.IncrementRequest(1)
	tracker.IncrementRequest(1)
	tracker.IncrementRequest(1)

	assert.Equal(t, int64(3), tracker.GetRequestCount(1))
}

func TestChannelRequestTracker_AddTokens(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.AddTokens(1, 100)
	tracker.AddTokens(1, 200)

	assert.Equal(t, int64(300), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_AddTokens_IgnoresNonPositive(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.AddTokens(1, 0)
	tracker.AddTokens(1, -10)

	assert.Equal(t, int64(0), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_GetRequestCount_UnknownChannel(t *testing.T) {
	tracker := NewChannelRequestTracker()
	assert.Equal(t, int64(0), tracker.GetRequestCount(999))
}

func TestChannelRequestTracker_GetTokenCount_UnknownChannel(t *testing.T) {
	tracker := NewChannelRequestTracker()
	assert.Equal(t, int64(0), tracker.GetTokenCount(999))
}

func TestChannelRequestTracker_MultipleChannels(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.IncrementRequest(1)
	tracker.IncrementRequest(1)
	tracker.IncrementRequest(2)
	tracker.AddTokens(1, 100)
	tracker.AddTokens(2, 500)

	assert.Equal(t, int64(2), tracker.GetRequestCount(1))
	assert.Equal(t, int64(1), tracker.GetRequestCount(2))
	assert.Equal(t, int64(100), tracker.GetTokenCount(1))
	assert.Equal(t, int64(500), tracker.GetTokenCount(2))
}

func TestChannelRequestTracker_WindowReset(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	tracker.IncrementRequest(1)
	assert.Equal(t, int64(1), tracker.GetRequestCount(1))
	assert.Equal(t, int64(0), tracker.GetTokenCount(1))

	*clockPtr = now.Add(time.Minute + time.Second)

	assert.Equal(t, int64(0), tracker.GetRequestCount(1))
	assert.Equal(t, int64(0), tracker.GetTokenCount(1))

	tracker.IncrementRequest(1)
	assert.Equal(t, int64(1), tracker.GetRequestCount(1))
	assert.Equal(t, int64(0), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_WindowReset_AddTokens(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	tracker.AddTokens(1, 500)
	assert.Equal(t, int64(500), tracker.GetTokenCount(1))

	*clockPtr = now.Add(time.Minute + time.Second)

	assert.Equal(t, int64(0), tracker.GetTokenCount(1))
	assert.Equal(t, int64(0), tracker.GetRequestCount(1))

	tracker.AddTokens(1, 200)
	assert.Equal(t, int64(200), tracker.GetTokenCount(1))
	assert.Equal(t, int64(0), tracker.GetRequestCount(1))
}

func TestChannelRequestTracker_Concurrent(t *testing.T) {
	tracker := NewChannelRequestTracker()

	const (
		goroutines      = 100
		opsPerGoroutine = 100
	)

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Concurrent IncrementRequest
	for range goroutines {
		go func() {
			defer wg.Done()

			for range opsPerGoroutine {
				tracker.IncrementRequest(1)
			}
		}()
	}

	// Concurrent AddTokens
	for range goroutines {
		go func() {
			defer wg.Done()

			for range opsPerGoroutine {
				tracker.AddTokens(1, 10)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(goroutines*opsPerGoroutine), tracker.GetRequestCount(1))
	assert.Equal(t, int64(goroutines*opsPerGoroutine*10), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_ConcurrentReadWrite(t *testing.T) {
	tracker := NewChannelRequestTracker()

	var wg sync.WaitGroup
	wg.Add(3)

	// Writer
	go func() {
		defer wg.Done()

		for range 1000 {
			tracker.IncrementRequest(1)
		}
	}()

	// Reader 1
	go func() {
		defer wg.Done()

		for range 1000 {
			_ = tracker.GetRequestCount(1)
		}
	}()

	// Reader 2
	go func() {
		defer wg.Done()

		for range 1000 {
			_ = tracker.GetTokenCount(1)
		}
	}()

	wg.Wait()

	assert.Equal(t, int64(1000), tracker.GetRequestCount(1))
}

func TestChannelRequestTracker_SetCooldown(t *testing.T) {
	tracker := NewChannelRequestTracker()

	// Set cooldown for 30 seconds from now
	until := time.Now().Add(30 * time.Second)
	tracker.SetCooldown(1, until)

	assert.True(t, tracker.IsCoolingDown(1))
	assert.False(t, tracker.IsCoolingDown(2))
}

func TestChannelRequestTracker_IsCoolingDown_NotSet(t *testing.T) {
	tracker := NewChannelRequestTracker()

	// No cooldown set
	assert.False(t, tracker.IsCoolingDown(1))
}

func TestChannelRequestTracker_IsCoolingDown_Expired(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	tracker.SetCooldown(1, now.Add(-10*time.Second))

	assert.False(t, tracker.IsCoolingDown(1))

	tracker.SetCooldown(1, now.Add(30*time.Second))
	assert.True(t, tracker.IsCoolingDown(1))
}

func TestChannelRequestTracker_GetCooldownUntil(t *testing.T) {
	tracker := NewChannelRequestTracker()

	// No cooldown set
	_, ok := tracker.GetCooldownUntil(1)
	assert.False(t, ok)

	// Set cooldown
	until := time.Now().Add(30 * time.Second)
	tracker.SetCooldown(1, until)

	// Get cooldown time
	gotUntil, ok := tracker.GetCooldownUntil(1)
	assert.True(t, ok)
	assert.Equal(t, until, gotUntil)
}

func TestChannelRequestTracker_GetCooldownUntil_Expired(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	tracker.SetCooldown(1, now.Add(-10*time.Second))

	_, ok := tracker.GetCooldownUntil(1)
	assert.False(t, ok)

	tracker.SetCooldown(1, now.Add(30*time.Second))
	gotUntil, ok := tracker.GetCooldownUntil(1)
	assert.True(t, ok)
	assert.Equal(t, now.Add(30*time.Second), gotUntil)
}

func TestChannelRequestTracker_ClearExpiredCooldown_DoesNotDeleteNewerValue(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	tracker.SetCooldown(1, now.Add(-10*time.Second))

	newUntil := now.Add(30 * time.Second)
	tracker.SetCooldown(1, newUntil)

	tracker.clearExpiredCooldown(1, now.Add(-10*time.Second), now)

	gotUntil, ok := tracker.GetCooldownUntil(1)
	assert.True(t, ok)
	assert.Equal(t, newUntil, gotUntil)
}

func TestChannelRequestTracker_MultipleChannels_Cooldown(t *testing.T) {
	tracker := NewChannelRequestTracker()

	now := time.Now()
	tracker.SetCooldown(1, now.Add(10*time.Second))
	tracker.SetCooldown(2, now.Add(20*time.Second))
	tracker.SetCooldown(3, now.Add(30*time.Second))

	assert.True(t, tracker.IsCoolingDown(1))
	assert.True(t, tracker.IsCoolingDown(2))
	assert.True(t, tracker.IsCoolingDown(3))

	// Channel 4 not in cooldown
	assert.False(t, tracker.IsCoolingDown(4))
}

func TestChannelRequestTracker_Cooldown_Concurrent(t *testing.T) {
	tracker := NewChannelRequestTracker()

	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	now := time.Now()

	// Concurrent SetCooldown
	for i := range goroutines {
		go func(channelID int) {
			defer wg.Done()

			tracker.SetCooldown(channelID, now.Add(30*time.Second))
		}(i)
	}

	wg.Wait()

	// All channels should be in cooldown
	for i := range goroutines {
		assert.True(t, tracker.IsCoolingDown(i))
	}
}

func TestChannelRequestTracker_Cooldown_ConcurrentReadWrite(t *testing.T) {
	tracker := NewChannelRequestTracker()

	const ops = 1000

	var wg sync.WaitGroup
	wg.Add(ops * 2)

	now := time.Now()

	// Writer: SetCooldown
	for range ops {
		go func() {
			defer wg.Done()

			tracker.SetCooldown(1, now.Add(30*time.Second))
		}()
	}

	// Reader: IsCoolingDown
	for range ops {
		go func() {
			defer wg.Done()

			_ = tracker.IsCoolingDown(1)
		}()
	}

	wg.Wait()

	// Should still be in cooldown
	assert.True(t, tracker.IsCoolingDown(1))
}

func TestChannelRequestTracker_ForDuration_MultiWindow(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.IncrementRequestForDuration(1, time.Hour, nil)
	tracker.IncrementRequestForDuration(1, time.Hour, nil)
	tracker.IncrementRequestForDuration(1, time.Hour, nil)
	tracker.IncrementRequestForDuration(1, 5*time.Hour, nil)

	minuteCount := tracker.GetRequestCountForDuration(1, time.Minute, nil)
	hourCount := tracker.GetRequestCountForDuration(1, time.Hour, nil)
	fiveHourCount := tracker.GetRequestCountForDuration(1, 5*time.Hour, nil)

	assert.Equal(t, int64(2), minuteCount)
	assert.Equal(t, int64(3), hourCount)
	assert.Equal(t, int64(1), fiveHourCount)
}

func TestChannelRequestTracker_ForDuration_WindowExpiry(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.AddTokensForDuration(1, 500, time.Minute, nil)

	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(1, time.Minute, nil))
	assert.Equal(t, int64(500), tracker.GetTokenCountForDuration(1, time.Minute, nil))

	*clockPtr = now.Add(2*time.Minute + time.Second)

	requestCount := tracker.GetRequestCountForDuration(1, time.Minute, nil)
	tokenCount := tracker.GetTokenCountForDuration(1, time.Minute, nil)

	assert.Equal(t, int64(0), requestCount)
	assert.Equal(t, int64(0), tokenCount)
}

func TestChannelRequestTracker_ForDuration_Tokens(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.AddTokensForDuration(1, 1000, time.Minute, nil)
	tracker.AddTokensForDuration(1, 2000, time.Hour, nil)
	tracker.AddTokensForDuration(1, 5000, 5*time.Hour, nil)

	minuteTokens := tracker.GetTokenCountForDuration(1, time.Minute, nil)
	hourTokens := tracker.GetTokenCountForDuration(1, time.Hour, nil)
	fiveHourTokens := tracker.GetTokenCountForDuration(1, 5*time.Hour, nil)

	assert.Equal(t, int64(1000), minuteTokens)
	assert.Equal(t, int64(2000), hourTokens)
	assert.Equal(t, int64(5000), fiveHourTokens)
}

func TestChannelRequestTracker_ForDuration_BackwardCompat(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.IncrementRequest(1)
	tracker.IncrementRequest(1)
	tracker.AddTokens(1, 500)

	backwardCompatRequestCount := tracker.GetRequestCount(1)
	forDurationRequestCount := tracker.GetRequestCountForDuration(1, time.Minute, nil)
	assert.Equal(t, backwardCompatRequestCount, forDurationRequestCount)
	assert.Equal(t, int64(2), forDurationRequestCount)

	backwardCompatTokenCount := tracker.GetTokenCount(1)
	forDurationTokenCount := tracker.GetTokenCountForDuration(1, time.Minute, nil)
	assert.Equal(t, backwardCompatTokenCount, forDurationTokenCount)
	assert.Equal(t, int64(500), forDurationTokenCount)

	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.AddTokensForDuration(1, 100, time.Minute, nil)

	assert.Equal(t, int64(3), tracker.GetRequestCount(1))
	assert.Equal(t, int64(600), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_Anchor_NonNilAnchor(t *testing.T) {
	tracker := NewChannelRequestTracker()
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	tracker.IncrementRequestForDuration(1, time.Hour, &anchor)
	tracker.IncrementRequestForDuration(1, time.Hour, &anchor)

	count := tracker.GetRequestCountForDuration(1, time.Hour, &anchor)
	assert.Equal(t, int64(2), count)
}

func TestChannelRequestTracker_Anchor_NilAndNonNilAreSeparate(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.IncrementRequestForDuration(1, time.Hour, nil)
	tracker.IncrementRequestForDuration(1, time.Hour, nil)

	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	tracker.IncrementRequestForDuration(1, time.Hour, &anchor)

	nilCount := tracker.GetRequestCountForDuration(1, time.Hour, nil)
	anchorCount := tracker.GetRequestCountForDuration(1, time.Hour, &anchor)

	assert.Equal(t, int64(0), nilCount)
	assert.Equal(t, int64(1), anchorCount)
}

func TestChannelRequestTracker_Anchor_AnchorChangeResetsWindow(t *testing.T) {
	tracker := NewChannelRequestTracker()
	anchor1 := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	anchor2 := time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC)

	tracker.IncrementRequestForDuration(1, time.Hour, &anchor1)
	tracker.IncrementRequestForDuration(1, time.Hour, &anchor1)
	assert.Equal(t, int64(2), tracker.GetRequestCountForDuration(1, time.Hour, &anchor1))

	tracker.IncrementRequestForDuration(1, time.Hour, &anchor2)
	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(1, time.Hour, &anchor2))

	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(1, time.Hour, &anchor1))
}

func TestChannelRequestTracker_Anchor_TokensWithAnchor(t *testing.T) {
	tracker := NewChannelRequestTracker()
	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	tracker.AddTokensForDuration(1, 500, time.Hour, &anchor)
	tracker.AddTokensForDuration(1, 300, time.Hour, &anchor)

	tokens := tracker.GetTokenCountForDuration(1, time.Hour, &anchor)
	assert.Equal(t, int64(800), tokens)
}

func TestChannelRequestTracker_Anchor_ReadPathRejectsDifferentAnchor(t *testing.T) {
	tracker := NewChannelRequestTracker()
	anchor1 := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	anchor2 := time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC)

	tracker.IncrementRequestForDuration(1, time.Hour, &anchor1)

	count := tracker.GetRequestCountForDuration(1, time.Hour, &anchor2)
	assert.Equal(t, int64(0), count)
}

func TestChannelRequestTracker_Anchor_FutureAnchor(t *testing.T) {
	tracker := NewChannelRequestTracker()
	futureAnchor := time.Now().Add(1 * time.Hour)

	tracker.IncrementRequestForDuration(1, time.Minute, &futureAnchor)
	count := tracker.GetRequestCountForDuration(1, time.Minute, &futureAnchor)
	assert.Equal(t, int64(1), count)
}

func TestChannelRequestTracker_Anchor_ZeroTimeAnchor(t *testing.T) {
	tracker := NewChannelRequestTracker()
	zeroTime := time.Time{}

	tracker.IncrementRequestForDuration(1, time.Minute, &zeroTime)
	count := tracker.GetRequestCountForDuration(1, time.Minute, &zeroTime)
	assert.Equal(t, int64(1), count)
}

func TestChannelRequestTracker_GetWindowResetTimeForDuration_ActiveWindow(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	tracker.IncrementRequestForDuration(1, time.Minute, nil)

	resetTime := tracker.GetWindowResetTimeForDuration(1, time.Minute, nil)
	expectedResetTime := now.Add(time.Minute)

	assert.Equal(t, expectedResetTime, resetTime)
}

func TestChannelRequestTracker_GetWindowResetTimeForDuration_ExpiredWindow(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	tracker.IncrementRequestForDuration(1, time.Minute, nil)

	*clockPtr = now.Add(2*time.Minute + time.Second)

	resetTime := tracker.GetWindowResetTimeForDuration(1, time.Minute, nil)

	assert.True(t, resetTime.IsZero())
}

func TestChannelRequestTracker_GetWindowResetTimeForDuration_NoWindow(t *testing.T) {
	tracker := NewChannelRequestTracker()

	resetTime := tracker.GetWindowResetTimeForDuration(1, time.Minute, nil)

	assert.True(t, resetTime.IsZero())
}

func TestChannelRequestTracker_GetWindowResetTimeForDuration_WithAnchor(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	anchor := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	tracker.IncrementRequestForDuration(1, time.Hour, &anchor)

	resetTime := tracker.GetWindowResetTimeForDuration(1, time.Hour, &anchor)
	windowStart := now.Truncate(time.Hour)
	expectedResetTime := windowStart.Add(time.Hour)

	assert.Equal(t, expectedResetTime, resetTime)
}

func TestChannelRequestTracker_GetWindowResetTimeForDuration_WrongAnchor(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	anchor1 := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	anchor2 := time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC)

	tracker.IncrementRequestForDuration(1, time.Hour, &anchor1)

	resetTime := tracker.GetWindowResetTimeForDuration(1, time.Hour, &anchor2)

	assert.True(t, resetTime.IsZero())
}

func TestChannelRequestTracker_ForDuration_Concurrent(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	const (
		goroutines      = 50
		opsPerGoroutine = 100
	)

	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	for range goroutines {
		go func() {
			defer wg.Done()

			for range opsPerGoroutine {
				tracker.IncrementRequestForDuration(1, time.Minute, nil)
			}
		}()
	}

	for range goroutines {
		go func() {
			defer wg.Done()

			for range opsPerGoroutine {
				tracker.AddTokensForDuration(1, 10, time.Minute, nil)
			}
		}()
	}

	for range goroutines {
		go func() {
			defer wg.Done()

			for range opsPerGoroutine {
				_ = tracker.GetRequestCountForDuration(1, time.Minute, nil)
			}
		}()
	}

	for range goroutines {
		go func() {
			defer wg.Done()

			for range opsPerGoroutine {
				_ = tracker.GetTokenCountForDuration(1, time.Minute, nil)
			}
		}()
	}

	wg.Wait()

	requestCount := tracker.GetRequestCountForDuration(1, time.Minute, nil)
	tokenCount := tracker.GetTokenCountForDuration(1, time.Minute, nil)

	assert.Equal(t, int64(goroutines*opsPerGoroutine), requestCount)
	assert.Equal(t, int64(goroutines*opsPerGoroutine*10), tokenCount)
}

func TestChannelRequestTracker_ForDuration_Concurrent_MixedDurations(t *testing.T) {
	tracker := NewChannelRequestTracker()

	const goroutines = 30

	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	durations := []time.Duration{time.Minute, 5 * time.Minute, time.Hour}

	for range goroutines {
		go func(d time.Duration) {
			defer wg.Done()
			for range 50 {
				tracker.IncrementRequestForDuration(1, d, nil)
			}
		}(durations[0])
	}

	for range goroutines {
		go func(d time.Duration) {
			defer wg.Done()
			for range 50 {
				tracker.AddTokensForDuration(1, 100, d, nil)
			}
		}(durations[1])
	}

	for range goroutines {
		go func(d time.Duration) {
			defer wg.Done()
			for range 50 {
				_ = tracker.GetRequestCountForDuration(1, d, nil)
			}
		}(durations[2])
	}

	for range goroutines {
		go func(d time.Duration) {
			defer wg.Done()
			for range 50 {
				_ = tracker.GetTokenCountForDuration(1, d, nil)
			}
		}(durations[0])
	}

	wg.Wait()

	assert.Equal(t, int64(goroutines*50), tracker.GetRequestCountForDuration(1, time.Minute, nil))
	assert.Equal(t, int64(goroutines*50*100), tracker.GetTokenCountForDuration(1, 5*time.Minute, nil))
	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(1, time.Hour, nil))
	assert.Equal(t, int64(0), tracker.GetTokenCountForDuration(1, time.Minute, nil))
}

func TestSeedRequestCountBasic(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	tracker.SeedRequestCountForDuration(1, 50, time.Minute, nil)

	assert.Equal(t, int64(50), tracker.GetRequestCountForDuration(1, time.Minute, nil))
}

func TestSeedRequestCountAfterIncrement(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.IncrementRequestForDuration(1, time.Minute, nil)

	tracker.SeedRequestCountForDuration(1, 50, time.Minute, nil)

	assert.Equal(t, int64(50), tracker.GetRequestCountForDuration(1, time.Minute, nil))
}

func TestSeedRequestCountBelowExisting(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	for range 10 {
		tracker.IncrementRequestForDuration(1, time.Minute, nil)
	}

	tracker.SeedRequestCountForDuration(1, 5, time.Minute, nil)

	assert.Equal(t, int64(10), tracker.GetRequestCountForDuration(1, time.Minute, nil))
}

func TestSeedThenIncrement(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	tracker.SeedRequestCountForDuration(1, 50, time.Minute, nil)

	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.IncrementRequestForDuration(1, time.Minute, nil)

	assert.Equal(t, int64(55), tracker.GetRequestCountForDuration(1, time.Minute, nil))
}

func TestSeedResetsOnWindowRotation(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	tracker.SeedRequestCountForDuration(1, 50, time.Minute, nil)
	assert.Equal(t, int64(50), tracker.GetRequestCountForDuration(1, time.Minute, nil))

	*clockPtr = now.Add(2*time.Minute + time.Second)

	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(1, time.Minute, nil))

	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(1, time.Minute, nil))
}

func TestSeedTokenCountBasic(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	tracker.SeedTokenCountForDuration(1, 1000, time.Minute, nil)

	assert.Equal(t, int64(1000), tracker.GetTokenCountForDuration(1, time.Minute, nil))
}

func TestSeedWithZeroOrNegativeCount(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	tracker.IncrementRequestForDuration(1, time.Minute, nil)
	tracker.AddTokensForDuration(1, 100, time.Minute, nil)

	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(1, time.Minute, nil))
	assert.Equal(t, int64(100), tracker.GetTokenCountForDuration(1, time.Minute, nil))

	tracker.SeedRequestCountForDuration(1, 0, time.Minute, nil)
	tracker.SeedRequestCountForDuration(1, -5, time.Minute, nil)
	tracker.SeedTokenCountForDuration(1, 0, time.Minute, nil)
	tracker.SeedTokenCountForDuration(1, -5, time.Minute, nil)

	assert.Equal(t, int64(1), tracker.GetRequestCountForDuration(1, time.Minute, nil))
	assert.Equal(t, int64(100), tracker.GetTokenCountForDuration(1, time.Minute, nil))
}

func TestIsRequestWindowDbQueried(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	assert.False(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))

	tracker.MarkRequestWindowDbQueried(1, time.Minute, nil)

	assert.True(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))
}

func TestIsRequestWindowDbQueried_WindowRotation(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, clockPtr := newTrackerWithClock(now)

	tracker.MarkRequestWindowDbQueried(1, time.Minute, nil)
	assert.True(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))

	*clockPtr = now.Add(2*time.Minute + time.Second)

	assert.False(t, tracker.IsRequestWindowDbQueried(1, time.Minute, nil))
}

func TestIsTokenWindowDbQueried(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	assert.False(t, tracker.IsTokenWindowDbQueried(1, time.Minute, nil))

	tracker.MarkTokenWindowDbQueried(1, time.Minute, nil)

	assert.True(t, tracker.IsTokenWindowDbQueried(1, time.Minute, nil))
}

func TestDBFallbackPattern_Integration(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tracker, _ := newTrackerWithClock(now)

	RPMDur := time.Hour

	RPM := int64(100)
	assert.Equal(t, int64(0), tracker.GetRequestCountForDuration(1, RPMDur, nil))
	assert.False(t, tracker.IsRequestWindowDbQueried(1, RPMDur, nil))

	DBReturnedCount := int64(50)
	tracker.SeedRequestCountForDuration(1, DBReturnedCount, RPMDur, nil)
	tracker.MarkRequestWindowDbQueried(1, RPMDur, nil)

	assert.Equal(t, DBReturnedCount, tracker.GetRequestCountForDuration(1, RPMDur, nil))
	assert.True(t, tracker.IsRequestWindowDbQueried(1, RPMDur, nil))

	RPMUsed := tracker.GetRequestCountForDuration(1, RPMDur, nil)
	ratio := float64(RPMUsed) / float64(RPM)
	expectedScore := 100.0 * (1 - ratio)
	assert.Equal(t, 50.0, expectedScore)
}
