// Package live provides a generic in-memory cache that automatically refreshes
// its data periodically or on demand using polling mechanism.
package live

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/watcher"
)

// Cache provides a generic in-memory cache that automatically refreshes
// its data periodically or on demand. It features:
//   - Thread-safe access via RWMutex
//   - Smart refresh with update-time fingerprinting
//   - SingleFlight to prevent concurrent redundant reloads
//   - Async refresh with debounce/serialization
//   - Periodic refresh via RefreshInterval (required)
//   - Optional OnSwap callback for resource cleanup
//
// IMPORTANT: The cached data type T must be treated as immutable after being stored.
// Callers MUST NOT mutate values returned by GetData(). If T is a pointer, map, or slice,
// either ensure the underlying data is never modified, or use a copy before mutation.
//
// Note: This is a polling-based cache, not a real-time push-based cache.
type Cache[T any] struct {
	mu         sync.RWMutex
	data       T
	lastUpdate time.Time

	sf       singleflight.Group
	reloadMu sync.Mutex

	refreshFunc func(ctx context.Context, current T, lastUpdate time.Time) (newData T, newUpdateTime time.Time, changed bool, err error)
	//nolint:predeclared // Checked.
	onSwap func(old, new T)
	name   string

	debounceDelay time.Duration
	asyncReloadCh chan struct{}

	refreshInterval time.Duration
	refreshTimeout  time.Duration
	ticker          *time.Ticker

	stopCh   chan struct{}
	stopOnce sync.Once

	watchStop func()
}

// Options defines the configuration for Cache.
type Options[T any] struct {
	// Name is used for logging purposes.
	Name string

	// RefreshFunc is called to refresh the cache data.
	// It receives the current cached data and lastUpdate timestamp.
	// For incremental updates, clone current first then modify; for full refresh, ignore current.
	// Returns: new data, new update time, whether data changed, and any error.
	RefreshFunc func(ctx context.Context, current T, lastUpdate time.Time) (newData T, newUpdateTime time.Time, changed bool, err error)

	// OnSwap is called after data is swapped, useful for cleanup (e.g., stopping old token providers).
	// Called with old and new data. May be nil.
	//nolint:predeclared // Checked.
	OnSwap func(old, new T)

	// InitialValue is the initial cached value before first refresh.
	InitialValue T

	// RefreshInterval enables periodic refresh. Must be greater than zero.
	RefreshInterval time.Duration

	// DebounceDelay is the delay before async reload to batch multiple triggers.
	// Defaults to 500ms if not set.
	DebounceDelay time.Duration

	// RefreshTimeout is the timeout for each refresh operation.
	// Defaults to 30s if not set.
	RefreshTimeout time.Duration

	// Watcher can be used to trigger a cross-instance async reload on demand.
	// Supports EventRefresh (with time comparison) and EventForceRefresh.
	// Key-based events (EventInvalidateKeys, EventReloadKeys) are ignored by Cache.
	Watcher watcher.Watcher[CacheEvent[struct{}]]
}

// NewCache creates a new Cache with the given options.
func NewCache[T any](opts Options[T]) *Cache[T] {
	if opts.RefreshFunc == nil {
		panic("live.Cache: RefreshFunc is required")
	}

	if opts.RefreshInterval <= 0 {
		panic("live.Cache: RefreshInterval must be greater than zero")
	}

	debounce := opts.DebounceDelay
	if debounce == 0 {
		debounce = 500 * time.Millisecond
	}

	timeout := opts.RefreshTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	c := &Cache[T]{
		data:            opts.InitialValue,
		refreshFunc:     opts.RefreshFunc,
		onSwap:          opts.OnSwap,
		name:            opts.Name,
		debounceDelay:   debounce,
		refreshInterval: opts.RefreshInterval,
		refreshTimeout:  timeout,
		asyncReloadCh:   make(chan struct{}, 1),
		stopCh:          make(chan struct{}),
	}

	ctx := context.Background()

	c.ticker = time.NewTicker(opts.RefreshInterval)
	go c.worker()

	if opts.Watcher != nil {
		watchCh, stop := opts.Watcher.Watch()

		c.watchStop = stop
		go c.watchWorker(watchCh)
	}

	log.Debug(ctx, "live cache started with periodic refresh",
		log.String("name", c.name),
		log.Duration("interval", opts.RefreshInterval),
		log.Duration("debounce", debounce))

	return c
}

// GetData returns the current cached data.
func (c *Cache[T]) GetData() T {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.data
}

// GetLastUpdate returns the timestamp of the last successful data update.
func (c *Cache[T]) GetLastUpdate() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastUpdate
}

// SetLastUpdate sets the timestamp of the last successful data update.
// This is used by IndexedCache to synchronize the lastUpdate time after initial load.
func (c *Cache[T]) SetLastUpdate(t time.Time) {
	c.mu.Lock()
	c.lastUpdate = t
	c.mu.Unlock()
}

// Load performs a synchronous refresh of the cache.
// If force is true, it will refresh regardless of whether data has changed.
func (c *Cache[T]) Load(ctx context.Context, force bool) error {
	log.Debug(ctx, "live cache load requested", log.String("name", c.name), log.Bool("force", force))

	// Use singleflight to ensure only one reload happens at a time across all callers
	_, err, shared := c.sf.Do("load", func() (any, error) {
		return nil, c.loadInternal(ctx, force)
	})

	if shared {
		log.Debug(ctx, "live cache load deduplicated via singleflight", log.String("name", c.name))
	}

	if err != nil {
		log.Warn(ctx, "live cache load failed", log.String("name", c.name), log.Cause(err))
	}

	return err
}

func (c *Cache[T]) loadInternal(ctx context.Context, force bool) error {
	c.reloadMu.Lock()
	defer c.reloadMu.Unlock()

	c.mu.RLock()
	currentData := c.data
	currentLastUpdate := c.lastUpdate
	c.mu.RUnlock()

	// If we are not forcing, the refreshFunc should check if updates are needed
	// using the fingerprint (lastUpdate) we provide.
	lookupUpdate := currentLastUpdate
	if force {
		lookupUpdate = time.Time{}
	}

	newData, newUpdateTime, changed, err := c.refreshFunc(ctx, currentData, lookupUpdate)
	if err != nil {
		return err
	}

	if !changed && !force {
		log.Debug(ctx, "cache refresh skipped: no changes detected", log.String("name", c.name))
		return nil
	}

	c.mu.Lock()
	old := c.data
	c.data = newData
	c.lastUpdate = newUpdateTime
	c.mu.Unlock()

	// Call OnSwap callback for cleanup with panic recovery
	if c.onSwap != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error(ctx, "live cache onSwap callback panicked",
						log.String("name", c.name),
						log.Any("panic", r))
				}
			}()

			c.onSwap(old, newData)
		}()
	}

	log.Info(ctx, "cache refreshed", log.String("name", c.name), log.Time("update_time", newUpdateTime))

	return nil
}

// TriggerAsyncReload signals the background worker to perform an async reload.
// Multiple calls will be debounced according to DebounceDelay.
func (c *Cache[T]) TriggerAsyncReload() {
	select {
	case c.asyncReloadCh <- struct{}{}:
		log.Debug(context.Background(), "live cache async reload triggered", log.String("name", c.name))
	default:
		log.Debug(context.Background(), "live cache async reload already pending, skipped", log.String("name", c.name))
	}
}

// Stop gracefully stops the cache's background workers.
// After calling Stop, the cache should not be used.
func (c *Cache[T]) Stop() {
	c.stopOnce.Do(func() {
		log.Info(context.Background(), "live cache stopping", log.String("name", c.name))

		if c.watchStop != nil {
			c.watchStop()
		}

		close(c.stopCh)

		if c.ticker != nil {
			c.ticker.Stop()
		}
	})
}

func (c *Cache[T]) worker() {
	var (
		debounceTimer *time.Timer
		debounceCh    <-chan time.Time
	)

	for {
		select {
		case <-c.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			log.Debug(context.Background(), "live cache worker stopped", log.String("name", c.name))

			return

		case <-c.ticker.C:
			log.Debug(context.Background(), "live cache periodic refresh starting", log.String("name", c.name))
			c.doRefresh("periodic")

		case <-c.asyncReloadCh:
			// Debounce: reset timer on each trigger to batch multiple signals
			if debounceTimer == nil {
				debounceTimer = time.NewTimer(c.debounceDelay)
				debounceCh = debounceTimer.C
			} else {
				if !debounceTimer.Stop() {
					select {
					case <-debounceTimer.C:
					default:
					}
				}

				debounceTimer.Reset(c.debounceDelay)
			}

			log.Debug(context.Background(), "live cache async reload debouncing",
				log.String("name", c.name), log.Duration("delay", c.debounceDelay))

		case <-debounceCh:
			log.Debug(context.Background(), "live cache async refresh starting", log.String("name", c.name))
			c.doRefresh("async")
		}
	}
}

func (c *Cache[T]) doRefresh(source string) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(context.Background(), "live cache doRefresh panicked",
				log.String("name", c.name),
				log.String("source", source),
				log.Any("panic", r))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), c.refreshTimeout)
	defer cancel()

	if err := c.loadInternal(ctx, false); err != nil {
		log.Error(ctx, "live cache refresh failed", log.String("name", c.name), log.String("source", source), log.Cause(err))
	}
}

func (c *Cache[T]) watchWorker(ch <-chan CacheEvent[struct{}]) {
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
				c.TriggerAsyncReload()
			case EventRefresh:
				if event.UpdatedAt.IsZero() {
					c.TriggerAsyncReload()
					continue
				}

				c.mu.RLock()
				lastUpdate := c.lastUpdate
				c.mu.RUnlock()

				if event.UpdatedAt.After(lastUpdate) {
					c.TriggerAsyncReload()
				}
			case EventInvalidateKeys, EventReloadKeys:
				log.Debug(context.Background(), "cache ignoring key-based event",
					log.String("name", c.name),
					log.Int("event_type", int(event.Type)))
			}
		}
	}
}
