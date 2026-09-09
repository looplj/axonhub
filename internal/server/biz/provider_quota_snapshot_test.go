package biz

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

func TestQuotaCacheSnapshot_ConsumerMutationDoesNotChangeCache(t *testing.T) {
	// Given
	service := &ProviderQuotaService{}
	service.updateQuotaCache(1, "", providerquotastatus.StatusAvailable, true, snapshotTestLimits())

	// When
	returned := service.GetQuotaStatus(t.Context(), 1)
	require.NotNil(t, returned)
	returned.ProviderType = "changed"
	returned.Status = providerquotastatus.StatusExhausted
	returned.Ready = false
	returned.Limits = append(returned.Limits, provider_quota.QuotaLimitStatus{})
	returned.Limits[0].Status = "changed"
	*returned.Limits[0].NextResetAt = time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	*returned.Limits[0].PeriodStart = time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC)
	*returned.Limits[0].PeriodCost = 99.0
	*returned.Limits[0].PeriodQuota = 100.0

	// Then
	again := service.GetQuotaStatus(t.Context(), 1)
	require.NotNil(t, again)
	require.Equal(t, "", again.ProviderType)
	require.Equal(t, providerquotastatus.StatusAvailable, again.Status)
	require.True(t, again.Ready)
	require.Len(t, again.Limits, 1)
	require.Equal(t, "available", again.Limits[0].Status)
	require.Equal(t, time.Date(2026, 9, 8, 13, 0, 0, 0, time.UTC), *again.Limits[0].NextResetAt)
	require.Equal(t, time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC), *again.Limits[0].PeriodStart)
	require.Equal(t, 12.5, *again.Limits[0].PeriodCost)
	require.Equal(t, 25.0, *again.Limits[0].PeriodQuota)
}

func TestQuotaCacheSnapshot_ProducerMutationDoesNotChangeCache(t *testing.T) {
	// Given
	service := &ProviderQuotaService{}
	limits := snapshotTestLimits()

	// When
	service.updateQuotaCache(1, "", providerquotastatus.StatusAvailable, true, limits)
	limits[0].Status = "changed"
	*limits[0].NextResetAt = time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	*limits[0].PeriodStart = time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC)
	*limits[0].PeriodCost = 99.0
	*limits[0].PeriodQuota = 100.0

	// Then
	returned := service.GetQuotaStatus(t.Context(), 1)
	require.NotNil(t, returned)
	require.Len(t, returned.Limits, 1)
	require.Equal(t, "available", returned.Limits[0].Status)
	require.Equal(t, time.Date(2026, 9, 8, 13, 0, 0, 0, time.UTC), *returned.Limits[0].NextResetAt)
	require.Equal(t, time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC), *returned.Limits[0].PeriodStart)
	require.Equal(t, 12.5, *returned.Limits[0].PeriodCost)
	require.Equal(t, 25.0, *returned.Limits[0].PeriodQuota)
}

func TestQuotaCacheSnapshot_RaceUpdateAndRead(t *testing.T) {
	// Given
	service := &ProviderQuotaService{}
	service.updateQuotaCache(1, "", providerquotastatus.StatusAvailable, true, snapshotTestLimits())
	const iterations = 1000
	const readers = 4
	var group sync.WaitGroup

	// When
	group.Go(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("cache writer panicked: %v", recovered)
			}
		}()
		for i := 0; i < iterations; i++ {
			limits := snapshotTestLimits()
			limits[0].UsageRatio = float64(i) / iterations
			service.updateQuotaCache(1, "", providerquotastatus.StatusAvailable, true, limits)
		}
	})

	for range readers {
		group.Go(func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("cache reader panicked: %v", recovered)
				}
			}()
			for j := 0; j < iterations; j++ {
				status := service.GetQuotaStatus(t.Context(), 1)
				if status == nil {
					t.Error("cache read returned nil")
					return
				}
				provider_quota.EffectiveStatus(status.Limits, status.Status, status.Ready, provider_quota.QuotaLimitTypeToken)
			}
		})
	}
	group.Wait()
}

func snapshotTestLimits() []provider_quota.QuotaLimitStatus {
	nextResetAt := time.Date(2026, 9, 8, 13, 0, 0, 0, time.UTC)
	periodStart := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	periodCost := 12.5
	periodQuota := 25.0
	return []provider_quota.QuotaLimitStatus{{
		Type:        provider_quota.QuotaLimitTypeToken,
		Status:      "available",
		UsageRatio:  0.5,
		Ready:       true,
		NextResetAt: &nextResetAt,
		PeriodStart: &periodStart,
		PeriodCost:  &periodCost,
		PeriodQuota: &periodQuota,
		Window:      provider_quota.QuotaWindow5h,
	}}
}
