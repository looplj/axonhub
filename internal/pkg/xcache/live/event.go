package live

import "time"

// EventType defines the type of cache invalidation event.
type EventType int

const (
	// EventRefresh triggers an incremental refresh using lastUpdate time comparison.
	EventRefresh EventType = iota
	// EventForceRefresh triggers a full refresh ignoring lastUpdate.
	EventForceRefresh
	// EventInvalidateKeys invalidates specific keys (IndexedCache only).
	// Invalidated keys will be reloaded on next Get call.
	EventInvalidateKeys
	// EventReloadKeys forces immediate reload of specific keys (IndexedCache only).
	EventReloadKeys
)

// CacheEvent represents a cache invalidation signal.
// For Cache, use CacheEvent[struct{}] since keys are not needed.
// For IndexedCache, use CacheEvent[K] where K is the key type.
type CacheEvent[K any] struct {
	// Type specifies the event type.
	Type EventType
	// UpdatedAt is the timestamp of the change (for EventRefresh).
	// Used for time comparison optimization to skip refresh if cache is already up-to-date.
	UpdatedAt time.Time
	// Keys specifies which keys to invalidate or reload (for EventInvalidateKeys/EventReloadKeys).
	Keys []K
}

// NewRefreshEvent creates a refresh event with the given update time.
func NewRefreshEvent[K any](updatedAt time.Time) CacheEvent[K] {
	return CacheEvent[K]{
		Type:      EventRefresh,
		UpdatedAt: updatedAt,
	}
}

// NewForceRefreshEvent creates a force refresh event.
func NewForceRefreshEvent[K any]() CacheEvent[K] {
	return CacheEvent[K]{
		Type: EventForceRefresh,
	}
}

// NewInvalidateKeysEvent creates an event to invalidate specific keys.
func NewInvalidateKeysEvent[K any](keys ...K) CacheEvent[K] {
	return CacheEvent[K]{
		Type: EventInvalidateKeys,
		Keys: keys,
	}
}

// NewReloadKeysEvent creates an event to force reload specific keys.
func NewReloadKeysEvent[K any](keys ...K) CacheEvent[K] {
	return CacheEvent[K]{
		Type: EventReloadKeys,
		Keys: keys,
	}
}
