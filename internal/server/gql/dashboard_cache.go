package gql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/authz"
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
	resultCh := dashboardQueryCacheGroup.DoChan(key, func() (any, error) {
		if value, ok := loadDashboardCache[T](key); ok {
			return value, nil
		}

		// The first caller may disconnect while other callers are waiting.
		// Detach cancellation but keep authz values from the request context.
		queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()

		value, err := compute(queryCtx)
		if err != nil {
			return nil, err
		}

		expiresAt := time.Now().Add(ttl)
		dashboardQueryCache.Store(key, dashboardCacheEntry{
			value:     value,
			expiresAt: expiresAt,
		})
		// Remove entries that are never looked up again. A refreshed entry has
		// a later expiresAt, so the stale timer leaves it in place.
		time.AfterFunc(ttl, func() {
			raw, ok := dashboardQueryCache.Load(key)
			if !ok {
				return
			}
			entry, ok := raw.(dashboardCacheEntry)
			if ok && !time.Now().Before(entry.expiresAt) {
				dashboardQueryCache.Delete(key)
			}
		})

		return value, nil
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

func loadDashboardCache[T any](key string) (T, bool) {
	var zero T

	raw, ok := dashboardQueryCache.Load(key)
	if !ok {
		return zero, false
	}

	entry, ok := raw.(dashboardCacheEntry)
	if !ok || time.Now().After(entry.expiresAt) {
		dashboardQueryCache.Delete(key)
		return zero, false
	}

	value, ok := entry.value.(T)
	if !ok {
		return zero, false
	}

	return value, true
}
