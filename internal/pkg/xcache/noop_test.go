package xcache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoopCache(t *testing.T) {
	ctx := context.Background()
	cache := NewNoop[string]()

	// Test Get always returns error
	_, err := cache.Get(ctx, "test-key")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCacheNotConfigured)

	// Test Set does nothing and returns no error
	err = cache.Set(ctx, "test-key", "test-value")
	assert.NoError(t, err)

	// Test Get still returns error after Set
	_, err = cache.Get(ctx, "test-key")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCacheNotConfigured)

	// Test Delete does nothing and returns no error
	err = cache.Delete(ctx, "test-key")
	assert.NoError(t, err)

	// Test Clear does nothing and returns no error
	err = cache.Clear(ctx)
	assert.NoError(t, err)

	// Test Invalidate does nothing and returns no error
	err = cache.Invalidate(ctx)
	assert.NoError(t, err)

	// Test GetType returns "noop"
	assert.Equal(t, "noop", cache.GetType())
}

func TestNewFromConfigWithEmptyMode(t *testing.T) {
	cfg := Config{} // Empty config with no mode set
	cache := NewFromConfig[string](cfg)

	// Should return noop cache
	assert.Equal(t, "noop", cache.GetType())

	// Test that it behaves like noop
	ctx := context.Background()
	_, err := cache.Get(ctx, "test")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCacheNotConfigured)
}

func TestNewFromConfigWithInvalidMode(t *testing.T) {
	cfg := Config{
		Mode: "invalid-mode",
	}
	cache := NewFromConfig[string](cfg)

	// Should return noop cache
	assert.Equal(t, "noop", cache.GetType())

	// Test that it behaves like noop
	ctx := context.Background()
	_, err := cache.Get(ctx, "test")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCacheNotConfigured)
}
