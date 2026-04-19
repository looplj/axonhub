package orchestrator

import (
	"sync"
	"testing"

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
	incrementsPerModel := 30

	var incWg sync.WaitGroup
	for c := 1; c <= numChannels; c++ {
		for m := 0; m < numModels; m++ {
			model := "model-" + string(rune('A'+m))
			incWg.Add(1)
			go func(channelID int, modelName string) {
				defer incWg.Done()
				for i := 0; i < incrementsPerModel; i++ {
					tracker.IncrementModelConnection(channelID, modelName)
				}
			}(c, model)
		}
	}
	incWg.Wait()

	decrementsPerModel := 10
	expectedPerModel := incrementsPerModel - decrementsPerModel

	var decWg sync.WaitGroup
	for c := 1; c <= numChannels; c++ {
		for m := 0; m < numModels; m++ {
			model := "model-" + string(rune('A'+m))
			decWg.Add(1)
			go func(channelID int, modelName string) {
				defer decWg.Done()
				for i := 0; i < decrementsPerModel; i++ {
					tracker.DecrementModelConnection(channelID, modelName)
				}
			}(c, model)
		}
	}
	decWg.Wait()

	for c := 1; c <= numChannels; c++ {
		for m := 0; m < numModels; m++ {
			model := "model-" + string(rune('A'+m))
			count := tracker.GetModelConnectionCount(c, model)
			assert.Equal(t, expectedPerModel, count, "Channel %d, Model %s", c, model)
		}
	}
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


