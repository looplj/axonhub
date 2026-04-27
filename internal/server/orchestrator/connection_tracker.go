package orchestrator

import (
	"maps"
	"sync"
)

type ConnectionTracker interface {
	GetActiveConnections(channelID int) int
	GetMaxConnections(channelID int) int
	IncrementConnection(channelID int)
	DecrementConnection(channelID int)
}

type DefaultConnectionTracker struct {
	mu                      sync.RWMutex
	channelConnections      map[int]int
	maxConnectionsPerChannel int
}

func NewDefaultConnectionTracker(maxConnectionsPerChannel int) *DefaultConnectionTracker {
	return &DefaultConnectionTracker{
		channelConnections:      make(map[int]int),
		maxConnectionsPerChannel: maxConnectionsPerChannel,
	}
}

func (t *DefaultConnectionTracker) GetActiveConnections(channelID int) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.channelConnections[channelID]
}

func (t *DefaultConnectionTracker) GetMaxConnections(channelID int) int {
	return t.maxConnectionsPerChannel
}

func (t *DefaultConnectionTracker) IncrementConnection(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.channelConnections[channelID]++
}

func (t *DefaultConnectionTracker) DecrementConnection(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.channelConnections[channelID] > 0 {
		t.channelConnections[channelID]--
	}
	if t.channelConnections[channelID] == 0 {
		delete(t.channelConnections, channelID)
	}
}

func (t *DefaultConnectionTracker) GetAllConnections() map[int]int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snapshot := make(map[int]int, len(t.channelConnections))
	maps.Copy(snapshot, t.channelConnections)

	return snapshot
}
