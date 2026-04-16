package orchestrator

import (
	"strings"
	"sync"

	"github.com/looplj/axonhub/internal/objects"
)

// ModelConnectionTracker tracks active connections per channel+model combination.
// It is separate from the channel-wide connection tracker and focuses on
// per-model concurrency limits.
type ModelConnectionTracker struct {
	mu sync.RWMutex
	// connections tracks active connection count per channel ID and model
	// Structure: channelID -> model -> count
	connections map[int]map[string]int
}

// NewModelConnectionTracker creates a new model connection tracker.
func NewModelConnectionTracker() *ModelConnectionTracker {
	return &ModelConnectionTracker{
		connections: make(map[int]map[string]int),
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

	t.connections[channelID][model]++
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
		return
	}

	// Decrement count if positive
	if count := models[model]; count > 0 {
		models[model]--
	}

	// Clean up model entry if count reaches 0
	if models[model] == 0 {
		delete(models, model)
	}

	// Clean up channel entry if no models remain
	if len(models) == 0 {
		delete(t.connections, channelID)
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

// GetModelConcurrentLimit is a convenience method that delegates to
// ChannelRateLimit.GetModelConcurrentLimit. It does not use any tracker
// state — it simply provides a consistent API surface for callers
// that already have a reference to the tracker.
// It uses the settings.GetModelConcurrentLimit method which returns per-model limit if configured,
// otherwise falls back to MaxConcurrent.
// Returns the limit and a boolean indicating whether a custom per-model limit was found.
func (t *ModelConnectionTracker) GetModelConcurrentLimit(channelID int, model string, settings *objects.ChannelRateLimit) (int64, bool) {
	// Delegate to the settings method which handles fallback logic
	return settings.GetModelConcurrentLimit(model)
}
