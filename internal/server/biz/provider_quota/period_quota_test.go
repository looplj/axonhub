package provider_quota

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestEstimatePeriodQuota(t *testing.T) {
	t.Parallel()

	total, ok := EstimatePeriodQuota(45, 0.5)
	require.True(t, ok)
	require.InDelta(t, 90.0, total, 1e-9)

	total, ok = EstimatePeriodQuota(80, 1)
	require.True(t, ok)
	require.InDelta(t, 80.0, total, 1e-9)

	// An over-consumed period (ratio > 1) still yields the period's worth.
	total, ok = EstimatePeriodQuota(120, 1.2)
	require.True(t, ok)
	require.InDelta(t, 100.0, total, 1e-9)

	// No cost recorded in the period: nothing to extrapolate from.
	_, ok = EstimatePeriodQuota(0, 0.5)
	require.False(t, ok)
	_, ok = EstimatePeriodQuota(-1, 0.5)
	require.False(t, ok)

	// Ratios below the threshold would amplify noise into an absurd estimate.
	_, ok = EstimatePeriodQuota(10, 0)
	require.False(t, ok)
	_, ok = EstimatePeriodQuota(10, -0.1)
	require.False(t, ok)
	_, ok = EstimatePeriodQuota(10, MinPeriodQuotaUsageRatio/2)
	require.False(t, ok)

	total, ok = EstimatePeriodQuota(10, MinPeriodQuotaUsageRatio)
	require.True(t, ok)
	require.InDelta(t, 10/MinPeriodQuotaUsageRatio, total, 1e-9)
}

func TestQuotaLimitStatusFillPeriodQuota(t *testing.T) {
	t.Parallel()

	limit := QuotaLimitStatus{UsageRatio: 0.5, PeriodCost: lo.ToPtr(45.0)}
	limit.FillPeriodQuota()
	require.NotNil(t, limit.PeriodQuota)
	require.InDelta(t, 90.0, *limit.PeriodQuota, 1e-9)

	// A refreshed limit that no longer supports an estimate must not keep the
	// stale value from the previous check.
	limit.UsageRatio = 0
	limit.FillPeriodQuota()
	require.Nil(t, limit.PeriodQuota)

	limit.UsageRatio = 0.5
	limit.PeriodCost = nil
	limit.FillPeriodQuota()
	require.Nil(t, limit.PeriodQuota)
}

func TestQuotaDataFillPeriodQuotas(t *testing.T) {
	t.Parallel()

	quota := QuotaData{
		Limits: []QuotaLimitStatus{
			{UsageRatio: 0.25, PeriodCost: lo.ToPtr(20.0)},
			{UsageRatio: 0.8},
		},
	}
	quota.FillPeriodQuotas()

	require.NotNil(t, quota.Limits[0].PeriodQuota)
	require.InDelta(t, 80.0, *quota.Limits[0].PeriodQuota, 1e-9)
	require.Nil(t, quota.Limits[1].PeriodQuota)
}

func TestPeriodStartFromReset(t *testing.T) {
	t.Parallel()

	reset := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	start := PeriodStartFromReset(&reset, 5*time.Hour)
	require.NotNil(t, start)
	require.Equal(t, reset.Add(-5*time.Hour), *start)

	require.Nil(t, PeriodStartFromReset(nil, 5*time.Hour))
	require.Nil(t, PeriodStartFromReset(&reset, 0))
}

func TestPeriodStartFromMonthlyResetClampsToPreviousMonth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		reset time.Time
		start time.Time
	}{
		{
			name:  "common year February",
			reset: time.Date(2026, time.March, 31, 12, 34, 56, 789, time.UTC),
			start: time.Date(2026, time.February, 28, 12, 34, 56, 789, time.UTC),
		},
		{
			name:  "leap year February",
			reset: time.Date(2024, time.March, 31, 12, 34, 56, 789, time.UTC),
			start: time.Date(2024, time.February, 29, 12, 34, 56, 789, time.UTC),
		},
		{
			name:  "same day in previous month",
			reset: time.Date(2026, time.March, 15, 12, 34, 56, 789, time.UTC),
			start: time.Date(2026, time.February, 15, 12, 34, 56, 789, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			start := PeriodStartFromMonthlyReset(&tt.reset)
			require.NotNil(t, start)
			require.Equal(t, tt.start, *start)
		})
	}

	require.Nil(t, PeriodStartFromMonthlyReset(nil))
}

func TestQuotaLimitStatusWithWindow(t *testing.T) {
	t.Parallel()

	reset := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	limit := NewTokenLimitStatus("available", 0.4, &reset).WithWindow(QuotaWindow7d, 7*24*time.Hour)
	require.Equal(t, QuotaWindow7d, limit.Window)
	require.NotNil(t, limit.PeriodStart)
	require.Equal(t, reset.Add(-7*24*time.Hour), *limit.PeriodStart)

	// Unknown window length: labeled, but no period to aggregate over.
	unknown := NewTokenLimitStatus("available", 0.4, &reset).WithWindow(QuotaWindowMonthly, 0)
	require.Equal(t, QuotaWindowMonthly, unknown.Window)
	require.Nil(t, unknown.PeriodStart)
}
