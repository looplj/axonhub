package provider_quota

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuotaCheckers_NormalizeRatioAndStatusBoundaries(t *testing.T) {
	// Given an overage ratio, an unknown type, and an expired top-level reset.
	expiredReset := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	quota := NormalizeQuotaData(QuotaData{
		Status:      "unknown",
		NextResetAt: &expiredReset,
		Limits: []QuotaLimitStatus{
			{Type: QuotaLimitTypeToken, Window: QuotaWindow5h, UsageRatio: 1.5, Status: "available"},
			{Window: QuotaWindowWeekly, UsageRatio: 0.2, Status: "available"},
		},
	})

	// Then overage is exhausted and malformed identities do not enter Limits.
	require.Len(t, quota.Limits, 1)
	require.Equal(t, float64(1), quota.Limits[0].UsageRatio)
	require.Equal(t, "exhausted", quota.Limits[0].Status)
	require.False(t, quota.Limits[0].Ready)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	require.Nil(t, quota.NextResetAt)
	assertNormalizedLimitContract(t, quota)
}

func TestQuotaCheckers_ExpiryAndStatusPrecedence(t *testing.T) {
	// Given an expired limit reset and a future exhausted limit with a warning channel status.
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	expiredReset := now.Add(-time.Minute)
	expiredPeriodStart := expiredReset.Add(-5 * time.Hour)
	futureReset := now.Add(time.Hour)
	quota := normalizeQuotaDataAt(QuotaData{
		Status: "warning",
		Limits: []QuotaLimitStatus{
			{
				Type:        QuotaLimitTypeToken,
				Window:      QuotaWindowWeekly,
				UsageRatio:  0.5,
				Status:      "warning",
				NextResetAt: &expiredReset,
				PeriodStart: &expiredPeriodStart,
			},
			{
				Type:        QuotaLimitTypeToken,
				Window:      QuotaWindow5h,
				UsageRatio:  1.2,
				Status:      "available",
				NextResetAt: &futureReset,
			},
		},
	}, now)

	// Then expired timing data is cleared and the ready warning status outranks exhaustion.
	require.Len(t, quota.Limits, 2)
	require.Nil(t, quota.Limits[0].NextResetAt)
	require.Nil(t, quota.Limits[0].PeriodStart)
	require.Equal(t, "exhausted", quota.Limits[1].Status)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, futureReset, *quota.NextResetAt)
	assertNormalizedLimitContract(t, quota)
}

func TestQuotaCheckers_MalformedRatios(t *testing.T) {
	// Given non-finite and negative ratios alongside an invalid period boundary.
	resetAt := time.Date(2099, 9, 11, 10, 0, 0, 0, time.UTC)
	invalidPeriodStart := resetAt.Add(time.Hour)
	normalized := normalizeQuotaDataAt(QuotaData{
		Status: "unknown",
		Limits: []QuotaLimitStatus{
			{Type: QuotaLimitTypeToken, Window: "empty-window", UsageRatio: math.NaN(), Status: "available"},
			{Type: QuotaLimitTypeToken, Window: "infinite-ratio", UsageRatio: math.Inf(1), Status: "available"},
			{Type: QuotaLimitTypeToken, Window: "negative-ratio", UsageRatio: -0.1, Status: "available"},
			{Type: QuotaLimitTypeToken, Window: "bad-period", UsageRatio: 0.2, Status: "available", NextResetAt: &resetAt, PeriodStart: &invalidPeriodStart},
		},
	}, time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC))

	// Then malformed ratios are omitted and an invalid period start is cleared.
	require.Len(t, normalized.Limits, 1)
	require.Equal(t, "bad-period", normalized.Limits[0].Window)
	require.Nil(t, normalized.Limits[0].PeriodStart)
	assertNormalizedLimitContract(t, normalized)
}

func TestNormalizeQuotaData_OverallStatusUsesReadyLimitOR(t *testing.T) {
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		limits     []QuotaLimitStatus
		wantStatus string
		wantReady  bool
	}{
		{
			name: "exhausted and available",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeSubscriptionCycle, Window: QuotaWindowCycle, UsageRatio: 1, Status: "exhausted"},
				{Type: QuotaLimitTypeToken, Window: QuotaWindowPayg, UsageRatio: 0.2, Status: "available"},
			},
			wantStatus: "available",
			wantReady:  true,
		},
		{
			name: "exhausted and warning",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeSubscriptionCycle, Window: QuotaWindowCycle, UsageRatio: 1, Status: "exhausted"},
				{Type: QuotaLimitTypeToken, Window: QuotaWindowPayg, UsageRatio: 0.9, Status: "warning"},
			},
			wantStatus: "warning",
			wantReady:  true,
		},
		{
			name: "all exhausted",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeSubscriptionCycle, Window: QuotaWindowCycle, UsageRatio: 1, Status: "exhausted"},
				{Type: QuotaLimitTypeToken, Window: QuotaWindowPayg, UsageRatio: 1, Status: "exhausted"},
			},
			wantStatus: "exhausted",
			wantReady:  false,
		},
		{
			name: "unknown only",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeToken, Window: QuotaWindowPayg, UsageRatio: 0, Status: "unknown"},
			},
			wantStatus: "unknown",
			wantReady:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When the common normalizer derives the overall status from limits.
			quota := normalizeQuotaDataAt(QuotaData{Status: "unknown", Limits: tt.limits}, now)

			// Then ready limits win over exhausted limits, and unknown remains unknown.
			require.Equal(t, tt.wantStatus, quota.Status)
			require.Equal(t, tt.wantReady, quota.Ready)
		})
	}
}

func TestNormalizeQuotaData_ExplicitStatusContributesReadyState(t *testing.T) {
	// Given an explicit warning status and an exhausted limit.
	quota := normalizeQuotaDataAt(QuotaData{
		Status: "warning",
		Limits: []QuotaLimitStatus{{
			Type:       QuotaLimitTypeToken,
			Window:     QuotaWindow5h,
			UsageRatio: 1,
			Status:     "exhausted",
		}},
	}, time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC))

	// Then the explicit ready status is not downgraded by an exhausted limit.
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
}
