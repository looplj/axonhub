package orchestrator

import (
	"sync"
	"testing"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/stretchr/testify/assert"
)

func TestModelConnectionTracker_IncrementAndGet(t *testing.T) {
	tracker := NewModelConnectionTracker()

	// Test incrementing for a channel+model
	tracker.IncrementModelConnection(1, "gpt-4")
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "gpt-4"))

	// Test incrementing same model again
	tracker.IncrementModelConnection(1, "gpt-4")
	assert.Equal(t, 2, tracker.GetModelConnectionCount(1, "gpt-4"))

	// Test different model on same channel
	tracker.IncrementModelConnection(1, "claude-3")
	assert.Equal(t, 2, tracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "claude-3"))

	// Test same model on different channel
	tracker.IncrementModelConnection(2, "gpt-4")
	assert.Equal(t, 2, tracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(2, "gpt-4"))
}

func TestModelConnectionTracker_Decrement(t *testing.T) {
	tracker := NewModelConnectionTracker()

	// Setup: add some connections
	tracker.IncrementModelConnection(1, "gpt-4")
	tracker.IncrementModelConnection(1, "gpt-4")
	tracker.IncrementModelConnection(1, "gpt-4")

	// Test decrement
	tracker.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 2, tracker.GetModelConnectionCount(1, "gpt-4"))

	tracker.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "gpt-4"))

	// Test decrement to zero (cleanup)
	tracker.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 0, tracker.GetModelConnectionCount(1, "gpt-4"))

	// Test decrement below zero (should stay at 0)
	tracker.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 0, tracker.GetModelConnectionCount(1, "gpt-4"))
}

func TestModelConnectionTracker_Cleanup(t *testing.T) {
	tracker := NewModelConnectionTracker()

	// Setup: add connections for multiple models
	tracker.IncrementModelConnection(1, "gpt-4")
	tracker.IncrementModelConnection(1, "claude-3")

	// Verify both exist
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "claude-3"))

	// Decrement one model to zero - should clean up model entry but keep channel
	tracker.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 0, tracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "claude-3"))

	// Decrement last model - should clean up channel entry
	tracker.DecrementModelConnection(1, "claude-3")
	assert.Equal(t, 0, tracker.GetModelConnectionCount(1, "claude-3"))

	// Verify internal cleanup by checking non-existent model returns 0
	assert.Equal(t, 0, tracker.GetModelConnectionCount(1, "non-existent"))
}

func TestModelConnectionTracker_CaseInsensitive(t *testing.T) {
	tracker := NewModelConnectionTracker()

	// Test that model names are case-insensitive
	tracker.IncrementModelConnection(1, "GPT-4")
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "GPT-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "Gpt-4"))

	tracker.IncrementModelConnection(1, "gpt-4")
	assert.Equal(t, 2, tracker.GetModelConnectionCount(1, "GPT-4"))

	tracker.DecrementModelConnection(1, "Gpt-4")
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "gpt-4"))
}

func TestModelConnectionTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewModelConnectionTracker()
	channelID := 1
	model := "gpt-4"
	numGoroutines := 100
	incrementsPerGoroutine := 10

	var wg sync.WaitGroup

	// Concurrent increments
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				tracker.IncrementModelConnection(channelID, model)
			}
		}()
	}

	wg.Wait()

	expectedCount := numGoroutines * incrementsPerGoroutine
	assert.Equal(t, expectedCount, tracker.GetModelConnectionCount(channelID, model))

	// Concurrent decrements
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				tracker.DecrementModelConnection(channelID, model)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, 0, tracker.GetModelConnectionCount(channelID, model))
}

func TestModelConnectionTracker_ConcurrentMultiChannelMultiModel(t *testing.T) {
	tracker := NewModelConnectionTracker()
	numChannels := 10
	numModels := 5
	numOperations := 50

	var wg sync.WaitGroup

	// Concurrent operations across multiple channels and models
	for c := 1; c <= numChannels; c++ {
		for m := 0; m < numModels; m++ {
			model := "model-" + string(rune('A'+m))
			wg.Add(1)
			go func(channelID int, modelName string) {
				defer wg.Done()
				for i := 0; i < numOperations; i++ {
					tracker.IncrementModelConnection(channelID, modelName)
					if i%2 == 0 {
						tracker.DecrementModelConnection(channelID, modelName)
					}
				}
			}(c, model)
		}
	}

	wg.Wait()

	// Verify counts are consistent
	for c := 1; c <= numChannels; c++ {
		for m := 0; m < numModels; m++ {
			model := "model-" + string(rune('A'+m))
			count := tracker.GetModelConnectionCount(c, model)
			// Each goroutine does numOperations increments and numOperations/2 decrements
			// So expected count is numOperations - numOperations/2 = numOperations/2
			expectedCount := numOperations - numOperations/2
			assert.Equal(t, expectedCount, count, "Channel %d, Model %s", c, model)
		}
	}
}

func TestModelConnectionTracker_GetModelConcurrentLimit(t *testing.T) {
	tracker := NewModelConnectionTracker()

	t.Run("nil settings", func(t *testing.T) {
		limit, hasCustom := tracker.GetModelConcurrentLimit(1, "gpt-4", nil)
		assert.Equal(t, int64(0), limit)
		assert.False(t, hasCustom)
	})

	t.Run("empty settings", func(t *testing.T) {
		settings := &objects.ChannelRateLimit{}
		limit, hasCustom := tracker.GetModelConcurrentLimit(1, "gpt-4", settings)
		assert.Equal(t, int64(0), limit)
		assert.False(t, hasCustom)
	})

	t.Run("max concurrent only", func(t *testing.T) {
		maxConcurrent := int64(100)
		settings := &objects.ChannelRateLimit{
			MaxConcurrent: &maxConcurrent,
		}
		limit, hasCustom := tracker.GetModelConcurrentLimit(1, "gpt-4", settings)
		assert.Equal(t, int64(100), limit)
		assert.False(t, hasCustom)
	})

	t.Run("per-model limit", func(t *testing.T) {
		maxConcurrent := int64(100)
		settings := &objects.ChannelRateLimit{
			MaxConcurrent: &maxConcurrent,
			ModelConcurrent: map[string]int64{
				"gpt-4": 50,
			},
		}
		limit, hasCustom := tracker.GetModelConcurrentLimit(1, "gpt-4", settings)
		assert.Equal(t, int64(50), limit)
		assert.True(t, hasCustom)
	})

	t.Run("per-model limit case insensitive", func(t *testing.T) {
		maxConcurrent := int64(100)
		settings := &objects.ChannelRateLimit{
			MaxConcurrent: &maxConcurrent,
			ModelConcurrent: map[string]int64{
				"gpt-4": 50,
			},
		}
		limit, hasCustom := tracker.GetModelConcurrentLimit(1, "GPT-4", settings)
		assert.Equal(t, int64(50), limit)
		assert.True(t, hasCustom)
	})

	t.Run("fallback to max concurrent when model not in map", func(t *testing.T) {
		maxConcurrent := int64(100)
		settings := &objects.ChannelRateLimit{
			MaxConcurrent: &maxConcurrent,
			ModelConcurrent: map[string]int64{
				"gpt-4": 50,
			},
		}
		limit, hasCustom := tracker.GetModelConcurrentLimit(1, "claude-3", settings)
		assert.Equal(t, int64(100), limit)
		assert.False(t, hasCustom)
	})

	t.Run("model concurrent without max concurrent", func(t *testing.T) {
		settings := &objects.ChannelRateLimit{
			ModelConcurrent: map[string]int64{
				"gpt-4": 50,
			},
		}
		limit, hasCustom := tracker.GetModelConcurrentLimit(1, "gpt-4", settings)
		assert.Equal(t, int64(50), limit)
		assert.True(t, hasCustom)

		// Fallback for unknown model should return 0
		limit, hasCustom = tracker.GetModelConcurrentLimit(1, "claude-3", settings)
		assert.Equal(t, int64(0), limit)
		assert.False(t, hasCustom)
	})
}

func TestModelConnectionTracker_MultipleChannelsIsolation(t *testing.T) {
	tracker := NewModelConnectionTracker()

	// Add connections to different channels
	tracker.IncrementModelConnection(1, "gpt-4")
	tracker.IncrementModelConnection(1, "gpt-4")
	tracker.IncrementModelConnection(2, "gpt-4")
	tracker.IncrementModelConnection(3, "gpt-4")

	// Verify isolation
	assert.Equal(t, 2, tracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(2, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(3, "gpt-4"))

	// Decrement from one channel shouldn't affect others
	tracker.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(2, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(3, "gpt-4"))
}

func TestModelConnectionTracker_DecrementNonExistent(t *testing.T) {
	tracker := NewModelConnectionTracker()

	// Should not panic when decrementing non-existent channel/model
	tracker.DecrementModelConnection(999, "non-existent")
	assert.Equal(t, 0, tracker.GetModelConnectionCount(999, "non-existent"))
}

func TestModelConnectionTracker_GetCountNonExistent(t *testing.T) {
	tracker := NewModelConnectionTracker()

	// Should return 0 for non-existent channel/model
	assert.Equal(t, 0, tracker.GetModelConnectionCount(999, "non-existent"))
}

func TestModelConnectionTracker_DecrementBelowZero(t *testing.T) {
	mt := NewModelConnectionTracker()
	mt.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 0, mt.GetModelConnectionCount(1, "gpt-4"))

	mt.IncrementModelConnection(1, "gpt-4")
	mt.DecrementModelConnection(1, "gpt-4")
	mt.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 0, mt.GetModelConnectionCount(1, "gpt-4"))
}

func TestModelConnectionTracker_CleanupOnZero(t *testing.T) {
	mt := NewModelConnectionTracker()

	mt.IncrementModelConnection(1, "gpt-4")
	assert.Equal(t, 1, mt.GetModelConnectionCount(1, "gpt-4"))
	mt.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 0, mt.GetModelConnectionCount(1, "gpt-4"))

	mt.mu.RLock()
	models := mt.connections[1]
	_, exists := models["gpt-4"]
	mt.mu.RUnlock()
	assert.False(t, exists, "Model entry should be cleaned up when count reaches 0")

	mt.IncrementModelConnection(2, "claude-3")
	mt.DecrementModelConnection(2, "claude-3")
	mt.mu.RLock()
	_, channelExists := mt.connections[2]
	mt.mu.RUnlock()
	assert.False(t, channelExists, "Channel entry should be cleaned up when no models remain")
}

func TestModelConnectionTracker_DoubleDecrementIdempotent(t *testing.T) {
	mt := NewModelConnectionTracker()
	mt.IncrementModelConnection(1, "gpt-4")

	mt.DecrementModelConnection(1, "gpt-4")
	mt.DecrementModelConnection(1, "gpt-4")
	mt.DecrementModelConnection(1, "gpt-4")
	assert.Equal(t, 0, mt.GetModelConnectionCount(1, "gpt-4"))
}
