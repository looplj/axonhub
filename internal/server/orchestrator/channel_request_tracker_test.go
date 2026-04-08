package orchestrator

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
	tracker := NewChannelRequestTracker()

	// Manually insert a window that belongs to a past minute
	tracker.mu.Lock()
	tracker.counters[1] = &rateLimitWindow{
		requests:    10,
		tokens:      500,
		windowStart: time.Now().Truncate(time.Minute).Add(-time.Minute),
	}
	tracker.mu.Unlock()

	// Reads should return 0 because the window is expired
	assert.Equal(t, int64(0), tracker.GetRequestCount(1))
	assert.Equal(t, int64(0), tracker.GetTokenCount(1))

	// Writes should create a new window
	tracker.IncrementRequest(1)
	assert.Equal(t, int64(1), tracker.GetRequestCount(1))
	assert.Equal(t, int64(0), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_WindowReset_AddTokens(t *testing.T) {
	tracker := NewChannelRequestTracker()

	// Manually insert a window that belongs to a past minute
	tracker.mu.Lock()
	tracker.counters[1] = &rateLimitWindow{
		requests:    10,
		tokens:      500,
		windowStart: time.Now().Truncate(time.Minute).Add(-time.Minute),
	}
	tracker.mu.Unlock()

	tracker.AddTokens(1, 200)
	assert.Equal(t, int64(200), tracker.GetTokenCount(1))
	// Requests should have been reset too since AddTokens creates a new window
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

// ========== Cooldown Tests ==========

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
	tracker := NewChannelRequestTracker()

	// Set cooldown in the past (expired)
	tracker.mu.Lock()
	tracker.cooldowns[1] = time.Now().Add(-10 * time.Second)
	tracker.mu.Unlock()

	// Should return false and clean up
	assert.False(t, tracker.IsCoolingDown(1))

	// Verify entry was deleted
	tracker.mu.RLock()
	_, exists := tracker.cooldowns[1]
	tracker.mu.RUnlock()
	assert.False(t, exists)
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
	tracker := NewChannelRequestTracker()

	// Set cooldown in the past (expired)
	tracker.mu.Lock()
	tracker.cooldowns[1] = time.Now().Add(-10 * time.Second)
	tracker.mu.Unlock()

	// Should return false and clean up
	_, ok := tracker.GetCooldownUntil(1)
	assert.False(t, ok)

	// Verify entry was deleted
	tracker.mu.RLock()
	_, exists := tracker.cooldowns[1]
	tracker.mu.RUnlock()
	assert.False(t, exists)
}

func TestChannelRequestTracker_ClearExpiredCooldown_DoesNotDeleteNewerValue(t *testing.T) {
	tracker := NewChannelRequestTracker()

	expiredUntil := time.Now().Add(-10 * time.Second)
	newUntil := time.Now().Add(30 * time.Second)
	now := time.Now()

	tracker.mu.Lock()
	tracker.cooldowns[1] = expiredUntil
	tracker.mu.Unlock()

	tracker.SetCooldown(1, newUntil)
	tracker.clearExpiredCooldown(1, expiredUntil, now)

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
