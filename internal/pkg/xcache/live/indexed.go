package live

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/watcher"
)

// cacheItem wraps a value with its expiration time.
// When isEmpty is true, value is zero value and represents a "not found" cache entry.
type cacheItem[V any] struct {
	value    V
	expireAt time.Time
	isEmpty  bool // true if this is a negative cache entry (key not found)
}

// IsExpired checks if the cache item has expired.
func (i *cacheItem[V]) IsExpired() bool {
	return !i.expireAt.IsZero() && time.Now().After(i.expireAt)
}

// IndexedCache provides key-based lookups with single-item operations, TTL support, and background incremental refresh.
//
// Features:
//   - O(1) lookups by key with automatic load on miss
//   - TTL support for automatic expiration of cached items
//   - Invalidate(key) to remove single item from cache
//   - Reload(ctx, key) to refresh single item
//   - Periodic incremental refresh as compensation mechanism (RefreshInterval required)
//   - SingleFlight to prevent concurrent loads of the same key
//
// IMPORTANT: The cached value type V must be treated as immutable after being stored.
// Callers MUST NOT mutate values returned by Get() or GetCached(). If V is a pointer, map, or slice,
// either ensure the underlying data is never modified, or use a copy before mutation.
//
// Example usage for API keys:
//
//	cache := NewIndexedCache(IndexedOptions[string, *APIKey]{
//	    Name:            "api_keys",
//	    TTL:             5 * time.Minute,
//	    RefreshInterval: 30 * time.Second,
//	    KeyFunc:         func(v *APIKey) string { return v.Key },
//	    LoadOneFunc: func(ctx context.Context, key string) (*APIKey, error) {
//	        return db.APIKey.Query().Where(apikey.KeyEQ(key)).First(ctx)
//	    },
//	    LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*APIKey, time.Time, error) {
//	        keys, err := db.APIKey.Query().Where(apikey.UpdatedAtGT(since)).All(ctx)
//	        return keys, time.Now(), err
//	    },
//	})
type IndexedCache[K comparable, V any] struct {
	mu    sync.RWMutex
	index map[K]*cacheItem[V]

	sf singleflight.Group

	opts  IndexedOptions[K, V]
	inner *Cache[struct{}] // only for periodic refresh scheduling

	watchStop func()
	stopCh    chan struct{}
	stopOnce  sync.Once
}

// IndexedOptions configures an IndexedCache.
type IndexedOptions[K comparable, V any] struct {
	// Name for logging purposes.
	Name string

	// TTL is the time-to-live for cached items. Zero means no expiration.
	TTL time.Duration

	// KeyFunc extracts the lookup key from a value.
	KeyFunc func(V) K

	// LoadOneFunc loads a single item by key.
	// Return the item, or error if not found (e.g., ent.NotFoundError).
	LoadOneFunc func(ctx context.Context, key K) (V, error)

	// LoadSinceFunc fetches items changed since the given time for incremental refresh.
	// - For initial load (since is zero), return all items.
	// - For incremental refresh, return only items updated after since.
	// Returns: items, latest update timestamp, error.
	LoadSinceFunc func(ctx context.Context, since time.Time) ([]V, time.Time, error)

	// DeletedFunc optionally identifies deleted items (e.g., via soft delete).
	// Return true if the item should be removed from cache.
	// If nil, items are never auto-removed during incremental refresh.
	DeletedFunc func(V) bool

	// RefreshInterval for periodic incremental refresh (compensation mechanism).
	// Must be greater than zero.
	RefreshInterval time.Duration

	// DebounceDelay is applied to on-demand reload signals (Watcher-triggered reload).
	// Defaults to the same value as live.Cache (500ms) when not set.
	DebounceDelay time.Duration

	// Watcher can be used to trigger cross-instance cache operations on demand.
	// Supports all event types:
	//   - EventRefresh: incremental refresh with time comparison
	//   - EventForceRefresh: full refresh ignoring lastUpdate
	//   - EventInvalidateKeys: invalidate specific keys (removed from cache, reloaded on next Get)
	//   - EventReloadKeys: force immediate reload of specific keys
	Watcher watcher.Watcher[CacheEvent[K]]
}

// NewIndexedCache creates an IndexedCache with the given options.
func NewIndexedCache[K comparable, V any](opts IndexedOptions[K, V]) *IndexedCache[K, V] {
	if opts.LoadOneFunc == nil {
		panic("live.IndexedCache: LoadOneFunc is required")
	}

	if opts.KeyFunc == nil {
		panic("live.IndexedCache: KeyFunc is required")
	}

	if opts.RefreshInterval <= 0 {
		panic("live.IndexedCache: RefreshInterval must be greater than zero")
	}

	if opts.LoadSinceFunc == nil {
		panic("live.IndexedCache: LoadSinceFunc is required")
	}

	ic := &IndexedCache[K, V]{
		index:  make(map[K]*cacheItem[V]),
		opts:   opts,
		stopCh: make(chan struct{}),
	}

	refreshFunc := func(ctx context.Context, _ struct{}, lastUpdate time.Time) (struct{}, time.Time, bool, error) {
		items, newUpdateTime, err := opts.LoadSinceFunc(ctx, lastUpdate)
		if err != nil {
			return struct{}{}, lastUpdate, false, err
		}

		if len(items) == 0 {
			ic.CleanupExpired()
			return struct{}{}, newUpdateTime, false, nil
		}

		ic.mu.Lock()

		for _, item := range items {
			key := opts.KeyFunc(item)
			if opts.DeletedFunc != nil && opts.DeletedFunc(item) {
				delete(ic.index, key)
			} else {
				ic.index[key] = &cacheItem[V]{
					value:    item,
					expireAt: ic.calcExpireAt(),
				}
			}
		}

		ic.mu.Unlock()

		ic.CleanupExpired()

		log.Info(ctx, "indexed cache incremental refresh completed",
			log.String("name", opts.Name),
			log.Int("updated", len(items)))

		return struct{}{}, newUpdateTime, true, nil
	}

	ic.inner = NewCache(Options[struct{}]{
		Name:            opts.Name + "_refresh",
		RefreshFunc:     refreshFunc,
		RefreshInterval: opts.RefreshInterval,
		DebounceDelay:   opts.DebounceDelay,
	})

	if opts.Watcher != nil {
		watchCh, stop := opts.Watcher.Watch()

		ic.watchStop = stop
		go ic.watchWorker(watchCh)
	}

	return ic
}

// calcExpireAt calculates the expiration time for a new cache item.
func (c *IndexedCache[K, V]) calcExpireAt() time.Time {
	if c.opts.TTL <= 0 {
		return time.Time{}
	}

	return time.Now().Add(c.opts.TTL)
}

func (c *IndexedCache[K, V]) calcNegativeExpireAt() time.Time {
	return time.Now().Add(5 * time.Second)
}

// Get retrieves a value by key.
// If not in cache and LoadOneFunc is configured, it will load from source.
// Returns the value and nil error if found or loaded successfully.
// Returns zero value and error if not found or load fails.
func (c *IndexedCache[K, V]) Get(ctx context.Context, key K) (V, error) {
	// Fast path: check cache first
	c.mu.RLock()
	item, ok := c.index[key]
	c.mu.RUnlock()

	if ok {
		if item.IsExpired() {
			c.mu.Lock()

			if current, exists := c.index[key]; exists && current.IsExpired() {
				delete(c.index, key)
			}

			c.mu.Unlock()

			return c.loadOne(ctx, key)
		}

		if item.isEmpty {
			var zero V
			return zero, ErrKeyNotFound
		}

		return item.value, nil
	}

	return c.loadOne(ctx, key)
}

// GetCached retrieves a value only from cache, without loading from source.
// Returns the value and true if found and not expired.
// Returns false if the key is cached as "not found" (negative cache).
func (c *IndexedCache[K, V]) GetCached(key K) (V, bool) {
	c.mu.RLock()
	item, ok := c.index[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	if item.IsExpired() {
		c.mu.Lock()

		if current, exists := c.index[key]; exists && current.IsExpired() {
			delete(c.index, key)
		}

		c.mu.Unlock()

		var zero V

		return zero, false
	}

	if item.isEmpty {
		var zero V
		return zero, false
	}

	return item.value, true
}

// loadOne loads a single item with singleflight deduplication.
func (c *IndexedCache[K, V]) loadOne(ctx context.Context, key K) (V, error) {
	// Use singleflight to prevent concurrent loads of the same key
	// Convert key to string for singleflight (it requires string keys)
	sfKey := fmt.Sprintf("%T:%v", key, key)

	result, err, _ := c.sf.Do(sfKey, func() (any, error) {
		// Double-check cache after acquiring singleflight
		c.mu.RLock()

		if item, ok := c.index[key]; ok && !item.IsExpired() {
			c.mu.RUnlock()

			if item.isEmpty {
				return nil, ErrKeyNotFound
			}

			return item.value, nil
		}

		c.mu.RUnlock()

		// Load from source
		v, err := c.opts.LoadOneFunc(ctx, key)
		if err != nil {
			// Cache negative result for ErrKeyNotFound to prevent cache penetration
			if errors.Is(err, ErrKeyNotFound) {
				c.mu.Lock()
				c.index[key] = &cacheItem[V]{
					expireAt: c.calcNegativeExpireAt(),
					isEmpty:  true,
				}
				c.mu.Unlock()

				log.Debug(ctx, "indexed cache cached negative result",
					log.String("name", c.opts.Name),
					log.Any("key", key))
			}

			return nil, err
		}

		// Check if deleted
		if c.opts.DeletedFunc != nil && c.opts.DeletedFunc(v) {
			log.Debug(ctx, "indexed cache item marked as deleted",
				log.String("name", c.opts.Name),
				log.Any("key", key))

			return nil, ErrKeyNotFound
		}

		// Store in cache
		c.mu.Lock()
		c.index[key] = &cacheItem[V]{
			value:    v,
			expireAt: c.calcExpireAt(),
		}
		c.mu.Unlock()

		log.Debug(ctx, "indexed cache loaded item",
			log.String("name", c.opts.Name),
			log.Any("key", key))

		return v, nil
	})
	if err != nil {
		log.Warn(ctx, "indexed cache load failed",
			log.String("name", c.opts.Name),
			log.Any("key", key),
			log.Cause(err))

		var zero V

		return zero, err
	}

	//nolint:forcetypeassert // Checked.
	return result.(V), nil
}

// Invalidate removes a single item from cache.
// The item will be reloaded on next Get call.
func (c *IndexedCache[K, V]) Invalidate(key K) {
	c.mu.Lock()
	delete(c.index, key)
	c.mu.Unlock()

	log.Debug(context.Background(), "indexed cache invalidated",
		log.String("name", c.opts.Name),
		log.Any("key", key))
}

// Reload forces a reload of a single item from source.
// Returns the loaded value and nil error if successful.
func (c *IndexedCache[K, V]) Reload(ctx context.Context, key K) (V, error) {
	// Invalidate first to ensure fresh load
	c.Invalidate(key)

	return c.loadOne(ctx, key)
}

// Set manually sets a value in the cache with TTL.
// Useful for updating cache after mutations without reloading.
func (c *IndexedCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.index[key] = &cacheItem[V]{
		value:    value,
		expireAt: c.calcExpireAt(),
	}
	c.mu.Unlock()
}

// GetAll returns a copy of all cached items (excluding expired items and negative cache entries).
func (c *IndexedCache[K, V]) GetAll() map[K]V {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[K]V, len(c.index))
	for k, item := range c.index {
		if !item.IsExpired() && !item.isEmpty {
			result[k] = item.value
		}
	}

	return result
}

// Len returns the number of cached items (excluding expired items and negative cache entries).
func (c *IndexedCache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0

	for _, item := range c.index {
		if !item.IsExpired() && !item.isEmpty {
			count++
		}
	}

	return count
}

// Load performs a full synchronous load using LoadSinceFunc with zero time.
// This is typically called during initialization.
func (c *IndexedCache[K, V]) Load(ctx context.Context) error {
	items, updateTime, err := c.opts.LoadSinceFunc(ctx, time.Time{})
	if err != nil {
		return err
	}

	c.mu.Lock()

	for _, item := range items {
		key := c.opts.KeyFunc(item)
		if c.opts.DeletedFunc != nil && c.opts.DeletedFunc(item) {
			delete(c.index, key)
		} else {
			c.index[key] = &cacheItem[V]{
				value:    item,
				expireAt: c.calcExpireAt(),
			}
		}
	}

	c.mu.Unlock()

	// Update inner cache's lastUpdate time if exists
	if c.inner != nil {
		c.inner.SetLastUpdate(updateTime)
	}

	log.Info(ctx, "indexed cache initial load completed",
		log.String("name", c.opts.Name),
		log.Int("count", len(items)))

	return nil
}

// Stop gracefully stops background workers.
func (c *IndexedCache[K, V]) Stop() {
	c.stopOnce.Do(func() {
		if c.watchStop != nil {
			c.watchStop()
		}

		close(c.stopCh)

		if c.inner != nil {
			c.inner.Stop()
		}
	})
}

func (c *IndexedCache[K, V]) watchWorker(ch <-chan CacheEvent[K]) {
	for {
		select {
		case <-c.stopCh:
			return
		case event, ok := <-ch:
			if !ok {
				return
			}

			switch event.Type {
			case EventForceRefresh:
				if c.inner != nil {
					c.inner.TriggerAsyncReload()
				}
			case EventRefresh:
				if c.inner != nil {
					if event.UpdatedAt.IsZero() {
						c.inner.TriggerAsyncReload()
						continue
					}

					lastUpdate := c.inner.GetLastUpdate()
					if event.UpdatedAt.After(lastUpdate) {
						c.inner.TriggerAsyncReload()
					}
				}
			case EventInvalidateKeys:
				for _, key := range event.Keys {
					c.Invalidate(key)
				}
			case EventReloadKeys:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				for _, key := range event.Keys {
					if _, err := c.Reload(ctx, key); err != nil {
						log.Warn(ctx, "indexed cache reload key failed",
							log.String("name", c.opts.Name),
							log.Any("key", key),
							log.Cause(err))
					}
				}

				cancel()
			}
		}
	}
}

// CleanupExpired removes all expired items from the cache.
// This can be called periodically to free up memory.
func (c *IndexedCache[K, V]) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiredCount := 0

	for key, item := range c.index {
		if item.IsExpired() {
			delete(c.index, key)

			expiredCount++
		}
	}

	if expiredCount > 0 {
		log.Debug(context.Background(), "indexed cache cleanup expired",
			log.String("name", c.opts.Name),
			log.Int("expired_count", expiredCount))
	}
}
