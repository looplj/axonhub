package biz

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestMetricClaims(t *testing.T) {
	claims := newRequestMetricClaims()

	// The first claim for a request wins.
	require.True(t, claims.claim(1))
	// Repeat claims for the same request lose, so terminal setters that race
	// or run sequentially record the request at most once.
	require.False(t, claims.claim(1))
	require.False(t, claims.claim(1))

	// Other requests are unaffected.
	require.True(t, claims.claim(2))
	require.False(t, claims.claim(2))
}

func TestRequestMetricClaimsConcurrent(t *testing.T) {
	claims := newRequestMetricClaims()

	const goroutines = 32
	var mu sync.Mutex
	winners := 0

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			won := claims.claim(42)
			mu.Lock()
			defer mu.Unlock()
			if won {
				winners++
			}
		})
	}
	wg.Wait()

	require.Equal(t, 1, winners)
}
