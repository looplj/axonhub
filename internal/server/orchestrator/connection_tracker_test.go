package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDefaultConnectionTracker(t *testing.T) {
	tracker := NewDefaultConnectionTracker(10)
	assert.NotNil(t, tracker)
	assert.Equal(t, 10, tracker.maxConnectionsPerChannel)
	assert.NotNil(t, tracker.channelConnections)
}

func TestConnectionTracker_IncrementAndGet(t *testing.T) {
	tracker := NewDefaultConnectionTracker(10)
	channelID := 1

	// Initial count should be 0
	assert.Equal(t, 0, tracker.GetActiveConnections(channelID))

	// Increment once
	tracker.IncrementConnection(channelID)
	assert.Equal(t, 1, tracker.GetActiveConnections(channelID))

	// Increment again
	tracker.IncrementConnection(channelID)
	assert.Equal(t, 2, tracker.GetActiveConnections(channelID))
}

func TestConnectionTracker_DecrementConnection(t *testing.T) {
	tracker := NewDefaultConnectionTracker(10)
	channelID := 1

	// Increment to 3
	tracker.IncrementConnection(channelID)
	tracker.IncrementConnection(channelID)
	tracker.IncrementConnection(channelID)
	assert.Equal(t, 3, tracker.GetActiveConnections(channelID))

	// Decrement once
	tracker.DecrementConnection(channelID)
	assert.Equal(t, 2, tracker.GetActiveConnections(channelID))

	// Decrement again
	tracker.DecrementConnection(channelID)
	assert.Equal(t, 1, tracker.GetActiveConnections(channelID))

	// Decrement to zero
	tracker.DecrementConnection(channelID)
	assert.Equal(t, 0, tracker.GetActiveConnections(channelID))
}

func TestConnectionTracker_DecrementBelowZero(t *testing.T) {
	tracker := NewDefaultConnectionTracker(10)
	channelID := 1

	// Decrement when already at 0 should not go negative
	tracker.DecrementConnection(channelID)
	assert.Equal(t, 0, tracker.GetActiveConnections(channelID))

	// Try again
	tracker.DecrementConnection(channelID)
	assert.Equal(t, 0, tracker.GetActiveConnections(channelID))
}

func TestConnectionTracker_MultipleChannels(t *testing.T) {
	tracker := NewDefaultConnectionTracker(10)

	// Increment different channels
	tracker.IncrementConnection(1)
	tracker.IncrementConnection(1)
	tracker.IncrementConnection(2)
	tracker.IncrementConnection(3)
	tracker.IncrementConnection(3)
	tracker.IncrementConnection(3)

	// Verify each channel's count
	assert.Equal(t, 2, tracker.GetActiveConnections(1))
	assert.Equal(t, 1, tracker.GetActiveConnections(2))
	assert.Equal(t, 3, tracker.GetActiveConnections(3))
	assert.Equal(t, 0, tracker.GetActiveConnections(4)) // Non-existent channel
}

func TestConnectionTracker_GetMaxConnections(t *testing.T) {
	maxConns := 15
	tracker := NewDefaultConnectionTracker(maxConns)

	// All channels should have the same max
	assert.Equal(t, maxConns, tracker.GetMaxConnections(1))
	assert.Equal(t, maxConns, tracker.GetMaxConnections(2))
	assert.Equal(t, maxConns, tracker.GetMaxConnections(999))
}

func TestConnectionTracker_GetAllConnections(t *testing.T) {
	tracker := NewDefaultConnectionTracker(10)

	// Add connections for multiple channels
	tracker.IncrementConnection(1)
	tracker.IncrementConnection(1)
	tracker.IncrementConnection(2)
	tracker.IncrementConnection(3)

	all := tracker.GetAllConnections()
	assert.Len(t, all, 3)
	assert.Equal(t, 2, all[1])
	assert.Equal(t, 1, all[2])
	assert.Equal(t, 1, all[3])
}

func TestConnectionTracker_CleanupOnZero(t *testing.T) {
	tracker := NewDefaultConnectionTracker(10)
	channelID := 1

	// Increment and then decrement to zero
	tracker.IncrementConnection(channelID)
	tracker.IncrementConnection(channelID)
	tracker.DecrementConnection(channelID)
	tracker.DecrementConnection(channelID)

	// Channel should be removed from map
	all := tracker.GetAllConnections()
	_, exists := all[channelID]
	assert.False(t, exists, "Channel should be cleaned up when count reaches 0")
}

func TestConnectionTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)
	channelID := 1

	// This test verifies that the mutex locks work correctly
	// by attempting concurrent increments and decrements
	done := make(chan bool)

	// Increment 50 times concurrently
	for i := 0; i < 50; i++ {
		go func() {
			tracker.IncrementConnection(channelID)

			done <- true
		}()
	}

	// Wait for all increments
	for i := 0; i < 50; i++ {
		<-done
	}

	// Should have exactly 50
	assert.Equal(t, 50, tracker.GetActiveConnections(channelID))

	// Decrement 25 times concurrently
	for i := 0; i < 25; i++ {
		go func() {
			tracker.DecrementConnection(channelID)

			done <- true
		}()
	}

	// Wait for all decrements
	for i := 0; i < 25; i++ {
		<-done
	}

	// Should have exactly 25 remaining
	assert.Equal(t, 25, tracker.GetActiveConnections(channelID))
}

func TestConnectionTracker_ZeroMaxConnections(t *testing.T) {
	tracker := NewDefaultConnectionTracker(0)

	// Should still track connections
	tracker.IncrementConnection(1)
	assert.Equal(t, 1, tracker.GetActiveConnections(1))

	// Max should return 0 (unlimited)
	assert.Equal(t, 0, tracker.GetMaxConnections(1))
}
