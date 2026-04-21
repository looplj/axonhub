package orchestrator

import (
	"context"
	"strings"
	"sync"

	"github.com/looplj/axonhub/internal/log"
)

// ModelConnectionTrackerInterface defines the interface for model connection tracking.
// This allows for dependency injection and easier testing.
type ModelConnectionTrackerInterface interface {
	IncrementModelConnection(channelID int, model string)
	DecrementModelConnection(ctx context.Context, channelID int, model string)
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
func (t *ModelConnectionTracker) DecrementModelConnection(ctx context.Context, channelID int, model string) {
	model = strings.ToLower(model)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if channel exists
	models, ok := t.connections[channelID]
	if !ok {
		log.Debug(ctx, "DecrementModelConnection called for unknown channel",
			log.Int("channel_id", channelID),
			log.String("model", model),
		)
		return
	}

	// Check if model exists in channel
	count, ok := models[model]
	if !ok {
		log.Debug(ctx, "DecrementModelConnection called for unknown model",
			log.Int("channel_id", channelID),
			log.String("model", model),
		)
		return
	}

	// Check if count is already zero (double decrement)
	if count <= 0 {
		log.Debug(ctx, "DecrementModelConnection called for model with zero count",
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

