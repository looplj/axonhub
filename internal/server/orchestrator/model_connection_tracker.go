package orchestrator

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/log"
)

// ModelConnectionTrackerInterface defines the interface for model connection tracking.
// This allows for dependency injection and easier testing.
type ModelConnectionTrackerInterface interface {
	IncrementModelConnection(channelID int, model string)
	DecrementModelConnection(channelID int, model string)
	GetModelConnectionCount(channelID int, model string) int
}

// ModelConnectionTracker tracks active connections per channel+model combination.
// It is separate from the channel-wide connection tracker and focuses on
// per-model concurrency limits.
type ModelConnectionTracker struct {
	mu sync.RWMutex
	// connections tracks active connection count per channel ID and model
	// Structure: channelID -> model -> count
	connections map[int]map[string]int
	// lastActivity tracks the last activity time for each channel+model combination
	// Used for detecting and cleaning up orphaned connections
	lastActivity map[int]map[string]time.Time
}

// NewModelConnectionTracker creates a new model connection tracker.
func NewModelConnectionTracker() *ModelConnectionTracker {
	return &ModelConnectionTracker{
		connections:  make(map[int]map[string]int),
		lastActivity: make(map[int]map[string]time.Time),
	}
}

// IncrementModelConnection increments the active connection count for a channel+model.
// Model names are normalized to lowercase for case-insensitive matching.
func (t *ModelConnectionTracker) IncrementModelConnection(channelID int, model string) {
	model = strings.ToLower(model)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Create inner map if it doesn't exist
	if t.connections[channelID] == nil {
		t.connections[channelID] = make(map[string]int)
	}
	if t.lastActivity[channelID] == nil {
		t.lastActivity[channelID] = make(map[string]time.Time)
	}

	t.connections[channelID][model]++
	t.lastActivity[channelID][model] = time.Now()
}

// DecrementModelConnection decrements the active connection count for a channel+model.
// Cleans up the model entry when count reaches 0, and the channel entry when no models remain.
// Model names are normalized to lowercase for case-insensitive matching.
func (t *ModelConnectionTracker) DecrementModelConnection(channelID int, model string) {
	model = strings.ToLower(model)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if channel exists
	models, ok := t.connections[channelID]
	if !ok {
		log.Debug(context.Background(), "DecrementModelConnection called for unknown channel",
			log.Int("channel_id", channelID),
			log.String("model", model),
		)
		return
	}

	// Check if model exists in channel
	count, ok := models[model]
	if !ok {
		log.Debug(context.Background(), "DecrementModelConnection called for unknown model",
			log.Int("channel_id", channelID),
			log.String("model", model),
		)
		return
	}

	// Check if count is already zero (double decrement)
	if count <= 0 {
		log.Debug(context.Background(), "DecrementModelConnection called for model with zero count",
			log.Int("channel_id", channelID),
			log.String("model", model),
			log.Int("current_count", count),
		)
		return
	}

	// Decrement count
	models[model]--

	// Clean up model entry if count reaches 0
	if models[model] == 0 {
		delete(models, model)
	}

	// Clean up last activity entry
	if t.lastActivity[channelID] != nil {
		delete(t.lastActivity[channelID], model)
	}

	// Clean up channel entry if no models remain
	if len(models) == 0 {
		delete(t.connections, channelID)
	}

	// Clean up channel last activity if no models remain
	if t.lastActivity[channelID] != nil && len(t.lastActivity[channelID]) == 0 {
		delete(t.lastActivity, channelID)
	}
}

// GetModelConnectionCount returns the current connection count for a channel+model.
// Model names are normalized to lowercase for case-insensitive matching.
func (t *ModelConnectionTracker) GetModelConnectionCount(channelID int, model string) int {
	model = strings.ToLower(model)

	t.mu.RLock()
	defer t.mu.RUnlock()

	if models, ok := t.connections[channelID]; ok {
		return models[model]
	}

	return 0
}

// EvictOrphaned removes entries that have been idle for longer than maxAge
// and have a connection count greater than 0 (leaked connections).
// This should be called periodically to clean up orphaned connections.
func (t *ModelConnectionTracker) EvictOrphaned(maxAge time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	evictedCount := 0

	for channelID, models := range t.connections {
		for model, count := range models {
			if lastAct, ok := t.lastActivity[channelID][model]; ok {
				if now.Sub(lastAct) > maxAge && count > 0 {
					// Found orphaned connection
					log.Warn(context.Background(), "Evicting orphaned model connection",
						log.Int("channel_id", channelID),
						log.String("model", model),
						log.Int("orphaned_count", count),
						log.Duration("idle_time", now.Sub(lastAct)),
					)
					delete(models, model)
					delete(t.lastActivity[channelID], model)
					evictedCount += count
				}
			}
		}

		// Clean up channel if no models remain
		if len(models) == 0 {
			delete(t.connections, channelID)
		}

		// Clean up channel last activity if no models remain
		if len(t.lastActivity[channelID]) == 0 {
			delete(t.lastActivity, channelID)
		}
	}

	return evictedCount
}

// GetLastActivity returns the last activity time for a channel+model combination.
// Returns the zero time if not found.
func (t *ModelConnectionTracker) GetLastActivity(channelID int, model string) time.Time {
	model = strings.ToLower(model)

	t.mu.RLock()
	defer t.mu.RUnlock()

	if lastActivity, ok := t.lastActivity[channelID]; ok {
		if t, ok := lastActivity[model]; ok {
			return t
		}
	}
	return time.Time{}
}