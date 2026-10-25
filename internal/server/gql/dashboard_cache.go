package gql

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/scopes"
)

// Dashboard aggregates can scan hundreds of thousands of rows. They change
// slowly relative to the dashboard refresh cadence, so cache them briefly and
// collapse concurrent loads from multiple admins into one database query.
//
// This cache is process-local. Multiple replicas and process restarts still do
// one cold computation each. The effective age on screen is the server TTL
// plus the client's polling interval, so these values are intentionally
// approximate rather than real-time.
const (
	dashboardCacheTTL     = time.Minute
	dashboardSlowCacheTTL = 5 * time.Minute
)

type dashboardCacheEntry struct {
	value     any
	expiresAt time.Time
}

var (
	dashboardQueryCache      sync.Map
	dashboardQueryCacheGroup singleflight.Group
)

// resetDashboardQueryCache clears cached aggregates. Tests use it after
// mutating fixture data.
func resetDashboardQueryCache() {
	dashboardQueryCache.Range(func(key, _ any) bool {
		dashboardQueryCache.Delete(key)
		return true
	})
}

// cachedDashboardQuery returns a cached result when one is fresh, otherwise
// runs compute once for all concurrent callers. The cache is keyed by the
// caller-provided key, which must include the ent client and normalized query
// parameters. Callers that lack the dashboard scope are rejected before any
// cached value or computation is touched; some dashboard resolvers execute
// raw SQL that does not go through Ent's privacy policy.
func cachedDashboardQuery[T any](
	ctx context.Context,
	key string,
	ttl time.Duration,
	compute func(context.Context) (T, error),
) (T, error) {
	var zero T

	if err := authz.RequireScope(ctx, scopes.ScopeReadDashboard); err != nil {
		return zero, err
	}

	if value, ok := loadDashboardCache[T](key); ok {
		return value, nil
	}

	// DoChan lets each waiter stop when its own request is canceled, while the
	// shared computation continues for the other dashboard viewers.
	// DoChan runs the callback in its own goroutine, so a panic inside compute
	// would bypass the request's recovery handlers. Recover here and turn the
	// panic into an error shared by every waiter on this flight.
	resultCh := dashboardQueryCacheGroup.DoChan(key, func() (any, error) {
		return computeDashboardCache[T](ctx, key, ttl, compute)
	})

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return zero, result.Err
		}

		value, ok := result.Val.(T)
		if !ok {
			return zero, fmt.Errorf("unexpected dashboard cache result type %T", result.Val)
		}

		return value, nil
	}
}

// computeDashboardCache runs the shared computation for one flight: re-checks
// the cache, computes, stores the result, and schedules expiry. Panics in
// compute are recovered and returned as an error so singleflight propagates
// them to every waiter as a normal resolver error.
func computeDashboardCache[T any](
	ctx context.Context,
	key string,
	ttl time.Duration,
	compute func(context.Context) (T, error),
) (value any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			// compute runs in the singleflight goroutine, outside the request's
			// recovery handlers, so log the panic before surfacing it as an
			// error for every waiter.
			log.Error(ctx, "dashboard cache computation panicked",
				log.String("cache_key", key),
				log.Any("panic", rec),
				log.String("stack", string(debug.Stack())),
			)
			value = nil
			err = fmt.Errorf("dashboard cache computation panicked: %v", rec)
		}
	}()

	if cached, ok := loadDashboardCache[T](key); ok {
		return cached, nil
	}

	// The first caller may disconnect while other callers are waiting.
	// Detach cancellation but keep authz values from the request context.
	queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()

	value, err = compute(queryCtx)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(ttl)
	// Stored as a pointer so the expiry timer can CAS with interface values
	// that compare by pointer identity. The entry's value field holds an
	// uncomparable slice, and sync.Map panics on value comparisons of
	// uncomparable dynamic values.
	entry := &dashboardCacheEntry{
		value:     value,
		expiresAt: expiresAt,
	}
	dashboardQueryCache.Store(key, entry)
	// Remove entries that are never looked up again. CompareAndDelete on the
	// exact observed entry pointer means a refreshed entry (a new allocation,
	// a different pointer) survives the stale timer.
	time.AfterFunc(ttl, func() {
		if time.Now().Before(expiresAt) {
			return
		}
		dashboardQueryCache.CompareAndDelete(key, entry)
	})

	return value, nil
}

func loadDashboardCache[T any](key string) (T, bool) {
	var zero T

	raw, ok := dashboardQueryCache.Load(key)
	if !ok {
		return zero, false
	}

	entry, ok := raw.(*dashboardCacheEntry)
	if !ok || time.Now().After(entry.expiresAt) {
		// Only remove the exact stale entry that was observed; a concurrent
		// refresh stored a new allocation with a different pointer and must
		// survive.
		if ok {
			dashboardQueryCache.CompareAndDelete(key, entry)
		} else {
			// A value that is not *dashboardCacheEntry should not happen here.
			// CompareAndDelete on the exact observed value is the safe
			// conditional delete: it can only match what Load returned, so a
			// concurrent replacement cannot be removed.
			dashboardQueryCache.CompareAndDelete(key, raw)
		}
		return zero, false
	}

	value, ok := entry.value.(T)
	if !ok {
		return zero, false
	}

	return value, true
}
