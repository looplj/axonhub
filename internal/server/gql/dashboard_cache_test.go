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
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, err := cachedDashboardQuery(ctx, key, time.Minute, compute)
			results <- value
			errors <- err
		}()
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
		{name: "preserves unknown window", timeWindow: new("custom:2026-01-01,2026-01-02"), limit: new(0), wantWindow: "custom:2026-01-01,2026-01-02", wantLimit: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWindow, gotLimit := normalizeChannelSuccessRatesParams(tt.timeWindow, tt.limit)
			require.Equal(t, tt.wantWindow, *gotWindow)
			require.Equal(t, tt.wantLimit, *gotLimit)
		})
	}
}
