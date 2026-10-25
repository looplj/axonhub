package gql

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
)

func TestCachedDashboardQuerySharesConcurrentAndRepeatedCalls(t *testing.T) {
	resetDashboardQueryCache()
	t.Cleanup(resetDashboardQueryCache)

	ctx := authz.WithTestBypass(context.Background())
	key := "test-dashboard-cache-key"

	var (
		mu    sync.Mutex
		calls int
	)

	compute := func(context.Context) (int, error) {
		mu.Lock()
		calls++
		mu.Unlock()

		// Hold the flight long enough for the other goroutines to join it.
		time.Sleep(25 * time.Millisecond)
		return 42, nil
	}

	const goroutines = 8

	start := make(chan struct{})
	results := make(chan int, goroutines)
	errors := make(chan error, goroutines)

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			<-start
			value, err := cachedDashboardQuery(ctx, key, time.Minute, compute)
			results <- value
			errors <- err
		})
	}

	close(start)
	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}
	for value := range results {
		require.Equal(t, 42, value)
	}

	mu.Lock()
	require.Equal(t, 1, calls, "concurrent callers should share one computation")
	mu.Unlock()

	// A later call within the TTL uses the cached value.
	value, err := cachedDashboardQuery(ctx, key, time.Minute, compute)
	require.NoError(t, err)
	require.Equal(t, 42, value)

	mu.Lock()
	require.Equal(t, 1, calls, "a repeated call within the TTL should not recompute")
	mu.Unlock()
}

func TestCachedDashboardQueryRequiresDashboardScope(t *testing.T) {
	resetDashboardQueryCache()
	t.Cleanup(resetDashboardQueryCache)

	called := false
	_, err := cachedDashboardQuery(context.Background(), "scope-test-key", time.Minute, func(context.Context) (int, error) {
		called = true
		return 1, nil
	})

	require.ErrorContains(t, err, "required scope")
	require.False(t, called, "an unauthorized caller must not trigger the computation")
}

func TestCachedDashboardQueryDoesNotCacheErrors(t *testing.T) {
	resetDashboardQueryCache()
	t.Cleanup(resetDashboardQueryCache)

	ctx := authz.WithTestBypass(context.Background())
	key := "error-test-key"
	computeErr := errors.New("transient failure")
	calls := 0

	_, err := cachedDashboardQuery(ctx, key, time.Minute, func(context.Context) (int, error) {
		calls++
		return 0, computeErr
	})
	require.ErrorIs(t, err, computeErr)

	value, err := cachedDashboardQuery(ctx, key, time.Minute, func(context.Context) (int, error) {
		calls++
		return 42, nil
	})
	require.NoError(t, err)
	require.Equal(t, 42, value)
	require.Equal(t, 2, calls, "a failed computation must not populate the cache")
}

func TestCachedDashboardQueryExpiresEntries(t *testing.T) {
	resetDashboardQueryCache()
	t.Cleanup(resetDashboardQueryCache)

	ctx := authz.WithTestBypass(context.Background())
	key := "expiry-test-key"
	calls := 0
	compute := func(context.Context) (int, error) {
		calls++
		return calls, nil
	}

	value, err := cachedDashboardQuery(ctx, key, 10*time.Millisecond, compute)
	require.NoError(t, err)
	require.Equal(t, 1, value)

	time.Sleep(30 * time.Millisecond)

	value, err = cachedDashboardQuery(ctx, key, 10*time.Millisecond, compute)
	require.NoError(t, err)
	require.Equal(t, 2, value)
	require.Equal(t, 2, calls)
}

func TestCachedDashboardQueryRequiresScopeBeforeReturningCachedValue(t *testing.T) {
	resetDashboardQueryCache()
	t.Cleanup(resetDashboardQueryCache)

	authorized := authz.WithTestBypass(context.Background())
	unauthorized := context.Background()
	key := "scope-before-cache-key"

	compute := func(context.Context) (int, error) {
		return 42, nil
	}

	value, err := cachedDashboardQuery(authorized, key, time.Minute, compute)
	require.NoError(t, err)
	require.Equal(t, 42, value)

	// The entry is fresh in the cache, but a caller without the dashboard
	// scope must be rejected before any cached value is read.
	_, err = cachedDashboardQuery(unauthorized, key, time.Minute, compute)
	require.ErrorContains(t, err, "required scope")
}

func TestCachedDashboardQuerySharesComputePanicWithWaiters(t *testing.T) {
	resetDashboardQueryCache()
	t.Cleanup(resetDashboardQueryCache)

	ctx := authz.WithTestBypass(context.Background())
	key := "panic-fanout-test-key"

	var (
		mu        sync.Mutex
		calls     int
		startedCh = make(chan struct{})
		started   bool
		joinedCh  = make(chan struct{}, 1)
		joined    bool
	)
	compute := func(context.Context) (int, error) {
		mu.Lock()
		calls++
		firstCall := calls == 1
		if firstCall && !started {
			started = true
			close(startedCh)
		}
		secondFlight := !firstCall && !joined
		if secondFlight {
			joined = true
		}
		mu.Unlock()

		if firstCall {
			// Let both callers join the same flight before it panics, so the
			// assertion is about fan-out to joined waiters, not about a
			// recompute after the flight finished. singleflight shares the
			// in-flight call, so a genuinely joined caller never re-enters
			// compute; joinedCh only signals when the second caller missed
			// the flight and started a new one. When the join succeeds the
			// channel stays silent and the timeout below releases the panic
			// to every waiter on the flight.
			select {
			case <-joinedCh:
			case <-time.After(5 * time.Second):
			}
			panic("boom")
		}
		if secondFlight {
			// Tell the first compute the join was missed so it releases its
			// panic immediately instead of waiting out the timeout; guarded
			// so later flights cannot double-signal.
			select {
			case joinedCh <- struct{}{}:
			default:
			}
		}
		return 42, nil
	}

	// Prime the shared flight; the second caller joins below.
	firstError := make(chan error, 1)
	go func() {
		_, err := cachedDashboardQuery(ctx, key, time.Minute, compute)
		firstError <- err
	}()

	require.Eventually(t, func() bool {
		select {
		case <-startedCh:
			return true
		default:
			return false
		}
	}, 5*time.Second, time.Millisecond, "the shared flight should start")

	secondError := make(chan error, 1)
	go func() {
		_, err := cachedDashboardQuery(ctx, key, time.Minute, compute)
		secondError <- err
	}()

	first := <-firstError
	require.Error(t, first, "the panicking computation must surface as an error")
	require.ErrorContains(t, first, "panic")
	require.ErrorContains(t, first, "boom")

	// The second caller joins while the flight is still in progress, so it
	// must receive the same fanned-out error instead of re-running compute.
	second := <-secondError
	require.Error(t, second, "a joined waiter must receive the panic as an error")
	require.Equal(t, first.Error(), second.Error(), "both callers should observe the same recovered panic")

	mu.Lock()
	require.Equal(t, 1, calls, "the panicking compute should run exactly once for both callers")
	mu.Unlock()

	// The panic must not poison the flight key: a later caller starts a
	// fresh flight and computes normally.
	value, err := cachedDashboardQuery(ctx, key, time.Minute, compute)
	require.NoError(t, err)
	require.Equal(t, 42, value)
}

func TestDashboardCacheExpiryTimerPreservesRefreshedEntry(t *testing.T) {
	resetDashboardQueryCache()
	t.Cleanup(resetDashboardQueryCache)

	ctx := authz.WithTestBypass(context.Background())
	key := "refresh-survives-timer-key"
	const (
		staleTTL   = 20 * time.Millisecond
		refreshTTL = 500 * time.Millisecond
	)

	calls := 0
	compute := func(context.Context) (int, error) {
		calls++
		return calls, nil
	}

	value, err := cachedDashboardQuery(ctx, key, staleTTL, compute)
	require.NoError(t, err)
	require.Equal(t, 1, value)

	// The refresh must land after the first entry expires, so it produces a
	// NEW entry pointer. Bounded wait loop; no hard sleep tuned to the TTL.
	require.Eventually(t, func() bool {
		_, ok := loadDashboardCache[int](key)
		return !ok
	}, 2*staleTTL, time.Millisecond, "the original entry should expire")

	refreshed, err := cachedDashboardQuery(ctx, key, refreshTTL, compute)
	require.NoError(t, err)
	require.Equal(t, 2, refreshed)
	require.Equal(t, 2, calls, "the refresh should compute a fresh entry")

	// The first entry's AfterFunc (scheduled at staleTTL) ran or is about to
	// run CompareAndDelete(key, originalPointer): it can only match the
	// stale pointer, so it must not remove the refreshed entry. A read
	// shortly after the timer window still serves the refresh.
	require.Eventually(t, func() bool {
		got, ok := loadDashboardCache[int](key)
		return ok && got == 2
	}, refreshTTL, time.Millisecond, "the refreshed entry must survive the original timer")

	value, err = cachedDashboardQuery(ctx, key, refreshTTL, compute)
	require.NoError(t, err)
	require.Equal(t, 2, value)
	require.Equal(t, 2, calls, "the stale timer must not force a recompute")
}

func TestLoadDashboardCacheStaleEntryPreservesConcurrentRefresh(t *testing.T) {
	resetDashboardQueryCache()
	t.Cleanup(resetDashboardQueryCache)

	key := "stale-cas-test-key"

	entryA := &dashboardCacheEntry{value: 1, expiresAt: time.Now().Add(-time.Second)}
	entryB := &dashboardCacheEntry{value: 2, expiresAt: time.Now().Add(time.Minute)}

	// Deterministic interleaving of the production paths:
	//  1. Entry A (expired) occupies the key.
	//  2. A load observes A: the code path that CASes the exact observed
	//     pointer runs against A.
	//  3. A concurrent refresh stored entry B (a new allocation) and survives.
	dashboardQueryCache.Store(key, entryA)
	_, ok := loadDashboardCache[int](key)
	require.False(t, ok, "the expired entry must not be served")
	dashboardQueryCache.Store(key, entryB)

	// The stale observation CASes A's pointer, which no longer matches the
	// stored value, so the refresh cannot be deleted.
	dashboardQueryCache.CompareAndDelete(key, entryA)

	value, ok := loadDashboardCache[int](key)
	require.True(t, ok, "entry B must survive a stale entry's compare-and-delete")
	require.Equal(t, 2, value)

	// A concurrent refresh that was made after the stale entry must also be
	// served directly, with no recompute: the exact-observed-pointer CAS is
	// the only delete in this sequence.
}

func TestCachedDashboardQueryWaiterCancellationKeepsSharedComputation(t *testing.T) {
	resetDashboardQueryCache()
	t.Cleanup(resetDashboardQueryCache)

	ctx1, cancel1 := context.WithCancel(authz.WithTestBypass(context.Background()))
	ctx2 := authz.WithTestBypass(context.Background())
	key := "cancel-test-key"

	started := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	compute := func(context.Context) (int, error) {
		calls++
		close(started)
		<-release
		return 42, nil
	}

	firstResult := make(chan error, 1)
	go func() {
		_, err := cachedDashboardQuery(ctx1, key, time.Minute, compute)
		firstResult <- err
	}()

	<-started

	secondResult := make(chan int, 1)
	go func() {
		value, err := cachedDashboardQuery(ctx2, key, time.Minute, compute)
		require.NoError(t, err)
		secondResult <- value
	}()

	// Let the second caller join the in-flight computation, then cancel the
	// first caller. The shared computation must continue for the second.
	time.Sleep(20 * time.Millisecond)
	cancel1()

	require.ErrorIs(t, <-firstResult, context.Canceled)

	close(release)
	require.Equal(t, 42, <-secondResult)
	require.Equal(t, 1, calls, "the shared computation should run once")
}

func TestNormalizeFastestChannelsInput(t *testing.T) {
	tests := []struct {
		name       string
		input      FastestChannelsInput
		wantWindow string
		wantLimit  int
	}{
		{name: "defaults", input: FastestChannelsInput{}, wantWindow: "day", wantLimit: 5},
		{name: "unknown window and negative limit", input: FastestChannelsInput{TimeWindow: "nope", Limit: new(-1)}, wantWindow: "day", wantLimit: 5},
		{name: "valid window", input: FastestChannelsInput{TimeWindow: "month", Limit: new(20)}, wantWindow: "month", wantLimit: 20},
		{name: "clamps limit", input: FastestChannelsInput{TimeWindow: "week", Limit: new(500)}, wantWindow: "week", wantLimit: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeFastestChannelsInput(tt.input)
			require.Equal(t, tt.wantWindow, got.TimeWindow)
			require.NotNil(t, got.Limit)
			require.Equal(t, tt.wantLimit, *got.Limit)
		})
	}
}

func TestNormalizeChannelSuccessRatesParams(t *testing.T) {
	tests := []struct {
		name       string
		timeWindow *string
		limit      *int
		wantWindow string
		wantLimit  int
	}{
		{name: "defaults", timeWindow: nil, limit: nil, wantWindow: "day", wantLimit: 0},
		{name: "empty window and negative limit", timeWindow: new(""), limit: new(-1), wantWindow: "day", wantLimit: 0},
		{name: "explicit values", timeWindow: new("week"), limit: new(10), wantWindow: "week", wantLimit: 10},
		{name: "unknown window canonicalizes to allTime", timeWindow: new("custom:2026-01-01,2026-01-02"), limit: new(0), wantWindow: "allTime", wantLimit: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWindow, gotLimit := normalizeChannelSuccessRatesParams(tt.timeWindow, tt.limit)
			require.Equal(t, tt.wantWindow, *gotWindow)
			require.Equal(t, tt.wantLimit, *gotLimit)
		})
	}
}
