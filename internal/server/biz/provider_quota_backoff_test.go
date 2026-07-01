package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuotaErrorBackoff(t *testing.T) {
	base := 5 * time.Minute

	// First failure (and the defensive failures<1 case) uses the base interval.
	require.Equal(t, base, quotaErrorBackoff(base, 0))
	require.Equal(t, base, quotaErrorBackoff(base, 1))

	// Then doubles per consecutive failure.
	require.Equal(t, 2*base, quotaErrorBackoff(base, 2))
	require.Equal(t, 4*base, quotaErrorBackoff(base, 3))
	require.Equal(t, 8*base, quotaErrorBackoff(base, 4))

	// Capped at maxQuotaErrorBackoffMultiplier (8x) regardless of further failures.
	require.Equal(t, 8*base, quotaErrorBackoff(base, 5))
	require.Equal(t, 8*base, quotaErrorBackoff(base, 100))
}

// TestQuotaErrorBackoffSaturation locks the invariant tying the two constants
// together: the multiplier reaches its cap exactly at maxQuotaErrorBackoffSteps,
// and not before. This guards against the constants drifting apart.
func TestQuotaErrorBackoffSaturation(t *testing.T) {
	base := time.Minute
	capped := time.Duration(maxQuotaErrorBackoffMultiplier) * base

	require.Equal(t, capped, quotaErrorBackoff(base, maxQuotaErrorBackoffSteps))
	require.Less(t, quotaErrorBackoff(base, maxQuotaErrorBackoffSteps-1), capped)
}

func TestQuotaErrorCount(t *testing.T) {
	require.Equal(t, 0, quotaErrorCount(nil))
	require.Equal(t, 0, quotaErrorCount(map[string]any{}))
	require.Equal(t, 3, quotaErrorCount(map[string]any{"error_count": 3}))
	require.Equal(t, 5, quotaErrorCount(map[string]any{"error_count": int64(5)}))
	// Reloaded from the DB, JSON numbers come back as float64.
	require.Equal(t, 4, quotaErrorCount(map[string]any{"error_count": float64(4)}))
}

func TestNextQuotaErrorCount(t *testing.T) {
	require.Equal(t, 1, nextQuotaErrorCount(0))
	require.Equal(t, 2, nextQuotaErrorCount(1))
	require.Equal(t, maxQuotaErrorBackoffSteps, nextQuotaErrorCount(maxQuotaErrorBackoffSteps-1))
	// Clamped — never grows past the saturation point, even after many failures.
	require.Equal(t, maxQuotaErrorBackoffSteps, nextQuotaErrorCount(maxQuotaErrorBackoffSteps))
	require.Equal(t, maxQuotaErrorBackoffSteps, nextQuotaErrorCount(100))
}
