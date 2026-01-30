package antigravity

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAntigravityHealthTracker_RecordFailure(t *testing.T) {
	tracker := NewAntigravityHealthTracker()

	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)

	failure := tracker.GetFailure("claude-sonnet-4-5", EndpointDaily)
	require.NotNil(t, failure)
	assert.Equal(t, "claude-sonnet-4-5", failure.Model)
	assert.Equal(t, EndpointDaily, failure.Endpoint)
	assert.Equal(t, 429, failure.StatusCode)
	assert.WithinDuration(t, time.Now(), failure.LastFailedAt, 1*time.Second)
	assert.WithinDuration(t, time.Now().Add(DefaultCooldownDuration), failure.CooldownUntil, 1*time.Second)
}

func TestAntigravityHealthTracker_RecordSuccess(t *testing.T) {
	tracker := NewAntigravityHealthTracker()

	// Record a failure first
	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)
	assert.NotNil(t, tracker.GetFailure("claude-sonnet-4-5", EndpointDaily))

	// Record success should clear the failure
	tracker.RecordSuccess("claude-sonnet-4-5", EndpointDaily)
	assert.Nil(t, tracker.GetFailure("claude-sonnet-4-5", EndpointDaily))
}

func TestAntigravityHealthTracker_ShouldSkip_WithinCooldown(t *testing.T) {
	// Use a short cooldown for testing
	tracker := NewAntigravityHealthTrackerWithConfig(100*time.Millisecond, 10*time.Minute)

	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)

	// Should skip immediately after failure
	assert.True(t, tracker.ShouldSkip("claude-sonnet-4-5", EndpointDaily))
}

func TestAntigravityHealthTracker_ShouldSkip_AfterCooldown(t *testing.T) {
	// Use a very short cooldown for testing
	tracker := NewAntigravityHealthTrackerWithConfig(10*time.Millisecond, 10*time.Minute)

	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)

	// Should skip within cooldown
	assert.True(t, tracker.ShouldSkip("claude-sonnet-4-5", EndpointDaily))

	// Wait for cooldown to expire
	time.Sleep(20 * time.Millisecond)

	// Should not skip after cooldown
	assert.False(t, tracker.ShouldSkip("claude-sonnet-4-5", EndpointDaily))
}

func TestAntigravityHealthTracker_PerModel_Isolation(t *testing.T) {
	tracker := NewAntigravityHealthTracker()

	// Record failure for claude on Daily
	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)

	// Claude should be in cooldown for Daily
	assert.True(t, tracker.ShouldSkip("claude-sonnet-4-5", EndpointDaily))

	// Gemini should NOT be in cooldown for Daily (different model)
	assert.False(t, tracker.ShouldSkip("gemini-2.5-pro", EndpointDaily))

	// Claude should NOT be in cooldown for Prod (different endpoint)
	assert.False(t, tracker.ShouldSkip("claude-sonnet-4-5", EndpointProd))
}

func TestAntigravityHealthTracker_TTL_Expiration(t *testing.T) {
	// Use very short TTL for testing
	tracker := NewAntigravityHealthTrackerWithConfig(60*time.Second, 20*time.Millisecond)

	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)

	// Entry should exist initially
	failure := tracker.GetFailure("claude-sonnet-4-5", EndpointDaily)
	require.NotNil(t, failure)

	// Wait for TTL to expire
	time.Sleep(30 * time.Millisecond)

	// Entry should be expired and return nil
	failure = tracker.GetFailure("claude-sonnet-4-5", EndpointDaily)
	assert.Nil(t, failure)

	// ShouldSkip should also clean up and return false
	assert.False(t, tracker.ShouldSkip("claude-sonnet-4-5", EndpointDaily))
}

func TestAntigravityHealthTracker_LazyCleanup(t *testing.T) {
	// Use very short TTL for testing
	tracker := NewAntigravityHealthTrackerWithConfig(60*time.Second, 20*time.Millisecond)

	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)

	// Stats should show 1 entry
	stats := tracker.Stats()
	assert.Equal(t, 1, stats.TotalEntries)

	// Wait for TTL to expire
	time.Sleep(30 * time.Millisecond)

	// Entry is still in map (not yet cleaned)
	stats = tracker.Stats()
	assert.Equal(t, 1, stats.TotalEntries)
	assert.Equal(t, 1, stats.Expired)

	// Accessing via ShouldSkip triggers lazy cleanup
	tracker.ShouldSkip("claude-sonnet-4-5", EndpointDaily)

	// Entry should now be removed from map
	stats = tracker.Stats()
	assert.Equal(t, 0, stats.TotalEntries)
}

func TestAntigravityHealthTracker_Stats(t *testing.T) {
	// Use custom durations for predictable testing
	tracker := NewAntigravityHealthTrackerWithConfig(100*time.Millisecond, 10*time.Minute)

	// Initially empty
	stats := tracker.Stats()
	assert.Equal(t, 0, stats.TotalEntries)
	assert.Equal(t, 0, stats.InCooldown)

	// Add some failures
	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)
	tracker.RecordFailure("gemini-2.5-pro", EndpointProd, 503)

	// Check stats
	stats = tracker.Stats()
	assert.Equal(t, 2, stats.TotalEntries)
	assert.Equal(t, 2, stats.InCooldown)
	assert.Len(t, stats.CooldownEntries, 2)

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	// Check stats again
	stats = tracker.Stats()
	assert.Equal(t, 2, stats.TotalEntries)
	assert.Equal(t, 0, stats.InCooldown) // No longer in cooldown
	assert.Len(t, stats.CooldownEntries, 0)
}

func TestAntigravityHealthTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewAntigravityHealthTracker()

	const (
		numGoroutines = 100
		numOperations = 100
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Simulate concurrent access from multiple goroutines
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()

			model := "test-model"
			endpoint := EndpointDaily

			for j := range numOperations {
				// Mix of operations
				switch j % 4 {
				case 0:
					tracker.RecordFailure(model, endpoint, 429)
				case 1:
					tracker.ShouldSkip(model, endpoint)
				case 2:
					tracker.RecordSuccess(model, endpoint)
				case 3:
					tracker.GetFailure(model, endpoint)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Should not panic or deadlock
	stats := tracker.Stats()
	assert.GreaterOrEqual(t, stats.TotalEntries, 0) // Just checking we can read stats
}

func TestAntigravityHealthTracker_Clear(t *testing.T) {
	tracker := NewAntigravityHealthTracker()

	// Add some failures
	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)
	tracker.RecordFailure("gemini-2.5-pro", EndpointProd, 503)

	stats := tracker.Stats()
	assert.Equal(t, 2, stats.TotalEntries)

	// Clear all entries
	tracker.Clear()

	stats = tracker.Stats()
	assert.Equal(t, 0, stats.TotalEntries)
	assert.Nil(t, tracker.GetFailure("claude-sonnet-4-5", EndpointDaily))
}

func TestAntigravityHealthTracker_MultipleModels_MultipleEndpoints(t *testing.T) {
	tracker := NewAntigravityHealthTracker()

	// Simulate various failures across models and endpoints
	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)
	tracker.RecordFailure("claude-sonnet-4-5", EndpointProd, 503)
	tracker.RecordFailure("gemini-2.5-pro", EndpointDaily, 404)
	tracker.RecordFailure("gemini-3-flash", EndpointAutopush, 500)

	// Verify each combination is tracked independently
	assert.True(t, tracker.ShouldSkip("claude-sonnet-4-5", EndpointDaily))
	assert.True(t, tracker.ShouldSkip("claude-sonnet-4-5", EndpointProd))
	assert.True(t, tracker.ShouldSkip("gemini-2.5-pro", EndpointDaily))
	assert.True(t, tracker.ShouldSkip("gemini-3-flash", EndpointAutopush))

	// Verify non-failed combinations are not skipped
	assert.False(t, tracker.ShouldSkip("claude-sonnet-4-5", EndpointAutopush))
	assert.False(t, tracker.ShouldSkip("gemini-2.5-pro", EndpointProd))
	assert.False(t, tracker.ShouldSkip("gemini-3-flash", EndpointDaily))

	// Stats should reflect 4 entries
	stats := tracker.Stats()
	assert.Equal(t, 4, stats.TotalEntries)
	assert.Equal(t, 4, stats.InCooldown)
}

func TestAntigravityHealthTracker_UpdateExistingFailure(t *testing.T) {
	tracker := NewAntigravityHealthTrackerWithConfig(100*time.Millisecond, 10*time.Minute)

	// Record initial failure
	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 429)
	firstFailure := tracker.GetFailure("claude-sonnet-4-5", EndpointDaily)
	require.NotNil(t, firstFailure)

	// Wait a bit
	time.Sleep(20 * time.Millisecond)

	// Record another failure for the same model+endpoint
	tracker.RecordFailure("claude-sonnet-4-5", EndpointDaily, 503)
	secondFailure := tracker.GetFailure("claude-sonnet-4-5", EndpointDaily)
	require.NotNil(t, secondFailure)

	// Should have updated the entry
	assert.Equal(t, 503, secondFailure.StatusCode)
	assert.True(t, secondFailure.LastFailedAt.After(firstFailure.LastFailedAt))
	assert.True(t, secondFailure.CooldownUntil.After(firstFailure.CooldownUntil))
}
