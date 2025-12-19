package orchestrator

import (
	"sync"
)

// DefaultConnectionTracker implements ConnectionTracker using an in-memory counter.
type DefaultConnectionTracker struct {
	mu sync.RWMutex
	// channelConnections tracks active connection count per channel ID
	channelConnections map[int]int
	// maxConnectionsPerChannel is the default max connections per channel (0 = unlimited)
	maxConnectionsPerChannel int
}

// NewDefaultConnectionTracker creates a new connection tracker.
func NewDefaultConnectionTracker(maxConnectionsPerChannel int) *DefaultConnectionTracker {
	return &DefaultConnectionTracker{
		channelConnections:       make(map[int]int),
		maxConnectionsPerChannel: maxConnectionsPerChannel,
	}
}

// GetActiveConnections returns the number of active connections for a channel.
func (t *DefaultConnectionTracker) GetActiveConnections(channelID int) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.channelConnections[channelID]
}

// GetMaxConnections returns the maximum connections allowed for a channel.
func (t *DefaultConnectionTracker) GetMaxConnections(channelID int) int {
	// For now, return the same limit for all channels
	// Could be extended to per-channel limits
	return t.maxConnectionsPerChannel
}

// IncrementConnection increments the active connection count for a channel.
func (t *DefaultConnectionTracker) IncrementConnection(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.channelConnections[channelID]++
}

// DecrementConnection decrements the active connection count for a channel.
func (t *DefaultConnectionTracker) DecrementConnection(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if count := t.channelConnections[channelID]; count > 0 {
		t.channelConnections[channelID]--
	}
	// Clean up if count reaches 0 to prevent memory leak
	if t.channelConnections[channelID] == 0 {
		delete(t.channelConnections, channelID)
	}
}

// GetAllConnections returns a snapshot of all active connections.
func (t *DefaultConnectionTracker) GetAllConnections() map[int]int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snapshot := make(map[int]int, len(t.channelConnections))
	for k, v := range t.channelConnections {
		snapshot[k] = v
	}

	return snapshot
}
