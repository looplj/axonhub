package xcache

import (
	"context"
	"errors"

	"github.com/eko/gocache/lib/v4/store"
)

// ErrCacheNotConfigured is returned when trying to get from a noop cache.
var ErrCacheNotConfigured = errors.New("cache not configured")

// noopCache is a cache implementation that does nothing.
// Get operations always return ErrCacheNotConfigured.
// Set, Delete, Invalidate, and Clear operations are no-ops.
type noopCache[T any] struct{}

// NewNoop creates a new noop cache that implements the Cache interface.
// This is useful when cache is not configured but you want to avoid nil checks.
func NewNoop[T any]() Cache[T] {
	return &noopCache[T]{}
}

// Get always returns ErrCacheNotConfigured.
func (n *noopCache[T]) Get(ctx context.Context, key any) (T, error) {
	var zero T
	return zero, store.NotFoundWithCause(ErrCacheNotConfigured)
}

// Set does nothing and always returns nil.
func (n *noopCache[T]) Set(ctx context.Context, key any, object T, options ...Option) error {
	return nil
}

// Delete does nothing and always returns nil.
func (n *noopCache[T]) Delete(ctx context.Context, key any) error {
	return nil
}

// Invalidate does nothing and always returns nil.
func (n *noopCache[T]) Invalidate(ctx context.Context, options ...store.InvalidateOption) error {
	return nil
}

// Clear does nothing and always returns nil.
func (n *noopCache[T]) Clear(ctx context.Context) error {
	return nil
}

// GetType returns "noop".
func (n *noopCache[T]) GetType() string {
	return "noop"
}
