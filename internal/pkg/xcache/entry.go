package xcache

import "time"

// Entry represents a cache entry with support for negative caching.
// It can represent either a valid value or a "not found" marker.
type Entry[T any] struct {
	Value    T
	IsEmpty  bool // true if this is a negative cache entry (key not found)
	ExpireAt time.Time
}

// IsExpired checks if the entry has expired.
func (e *Entry[T]) IsExpired() bool {
	return !e.ExpireAt.IsZero() && time.Now().After(e.ExpireAt)
}

// NewEntry creates a new cache entry with the given value and expiration.
func NewEntry[T any](value T, ttl time.Duration) *Entry[T] {
	var expireAt time.Time
	if ttl > 0 {
		expireAt = time.Now().Add(ttl)
	}

	return &Entry[T]{
		Value:    value,
		IsEmpty:  false,
		ExpireAt: expireAt,
	}
}

// NewEmptyEntry creates a new negative cache entry (representing "not found").
func NewEmptyEntry[T any](ttl time.Duration) *Entry[T] {
	return &Entry[T]{
		IsEmpty:  true,
		ExpireAt: time.Now().Add(ttl),
	}
}
