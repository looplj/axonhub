package biz

import (
	"sync"
	"testing"

	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/stretchr/testify/assert"
)

func TestProviderQuotaService_GetQuotaStatus_ReturnsCorrectData(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	svc.quotaCache.Store(1, &QuotaChannelStatus{Status: providerquotastatus.StatusAvailable, Ready: true})
	svc.quotaCache.Store(2, &QuotaChannelStatus{Status: providerquotastatus.StatusExhausted, Ready: false})
	svc.quotaCache.Store(3, &QuotaChannelStatus{Status: providerquotastatus.StatusWarning, Ready: true})

	status1 := svc.GetQuotaStatus(1)
	assert.NotNil(t, status1)
	assert.Equal(t, providerquotastatus.StatusAvailable, status1.Status)
	assert.True(t, status1.Ready)

	status2 := svc.GetQuotaStatus(2)
	assert.NotNil(t, status2)
	assert.Equal(t, providerquotastatus.StatusExhausted, status2.Status)
	assert.False(t, status2.Ready)

	status3 := svc.GetQuotaStatus(3)
	assert.NotNil(t, status3)
	assert.Equal(t, providerquotastatus.StatusWarning, status3.Status)
	assert.True(t, status3.Ready)
}

func TestProviderQuotaService_GetQuotaStatus_UnknownChannel(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	status := svc.GetQuotaStatus(999)
	assert.Nil(t, status)
}

func TestProviderQuotaService_UpdateQuotaCache(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	svc.updateQuotaCache(1, providerquotastatus.StatusAvailable, true, nil)
	svc.updateQuotaCache(2, providerquotastatus.StatusExhausted, false, nil)

	status1 := svc.GetQuotaStatus(1)
	assert.NotNil(t, status1)
	assert.Equal(t, providerquotastatus.StatusAvailable, status1.Status)
	assert.True(t, status1.Ready)

	status2 := svc.GetQuotaStatus(2)
	assert.NotNil(t, status2)
	assert.Equal(t, providerquotastatus.StatusExhausted, status2.Status)
	assert.False(t, status2.Ready)
}

func TestProviderQuotaService_UpdateQuotaCache_Overwrite(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	svc.updateQuotaCache(1, providerquotastatus.StatusAvailable, true, nil)
	svc.updateQuotaCache(1, providerquotastatus.StatusExhausted, false, nil)

	status := svc.GetQuotaStatus(1)
	assert.NotNil(t, status)
	assert.Equal(t, providerquotastatus.StatusExhausted, status.Status)
	assert.False(t, status.Ready)
}

func TestProviderQuotaService_ConcurrentAccess(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	var wg sync.WaitGroup
	const goroutines = 50

	wg.Add(goroutines)
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			svc.updateQuotaCache(id, providerquotastatus.StatusAvailable, true, nil)
		}(i)
	}

	wg.Add(goroutines)
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			_ = svc.GetQuotaStatus(id)
		}(i)
	}

	wg.Wait()

	for i := range goroutines {
		status := svc.GetQuotaStatus(i)
		assert.NotNil(t, status, "channel %d should have quota status", i)
		assert.Equal(t, providerquotastatus.StatusAvailable, status.Status)
		assert.True(t, status.Ready)
	}
}

func TestProviderQuotaService_ConcurrentReadWrite(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	svc.updateQuotaCache(1, providerquotastatus.StatusAvailable, true, nil)

	var wg sync.WaitGroup
	const iterations = 100

	wg.Add(iterations)
	for range iterations {
		go func() {
			defer wg.Done()
			svc.updateQuotaCache(1, providerquotastatus.StatusExhausted, false, nil)
		}()
	}

	wg.Add(iterations)
	for range iterations {
		go func() {
			defer wg.Done()
			_ = svc.GetQuotaStatus(1)
		}()
	}

	wg.Wait()

	status := svc.GetQuotaStatus(1)
	assert.NotNil(t, status)
	assert.Equal(t, providerquotastatus.StatusExhausted, status.Status)
	assert.False(t, status.Ready)
}

func TestProviderQuotaService_UpdateQuotaCache_WithLimits(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	limits := []provider_quota.QuotaLimitStatus{
		{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.3, Ready: true},
		{Type: provider_quota.QuotaLimitTypeImage, Status: "exhausted", UsageRatio: 1.0, Ready: false},
	}

	svc.updateQuotaCache(1, providerquotastatus.StatusWarning, true, limits)

	status := svc.GetQuotaStatus(1)
	assert.NotNil(t, status)
	assert.Equal(t, providerquotastatus.StatusWarning, status.Status)
	assert.True(t, status.Ready)
	assert.Len(t, status.Limits, 2)

	assert.Equal(t, provider_quota.QuotaLimitTypeToken, status.Limits[0].Type)
	assert.Equal(t, "available", status.Limits[0].Status)
	assert.InDelta(t, 0.3, status.Limits[0].UsageRatio, 0.001)
	assert.True(t, status.Limits[0].Ready)

	assert.Equal(t, provider_quota.QuotaLimitTypeImage, status.Limits[1].Type)
	assert.Equal(t, "exhausted", status.Limits[1].Status)
	assert.InDelta(t, 1.0, status.Limits[1].UsageRatio, 0.001)
	assert.False(t, status.Limits[1].Ready)

	effectiveStatus, ready := status.EffectiveStatus(provider_quota.QuotaLimitTypeImage)
	assert.Equal(t, providerquotastatus.StatusExhausted, effectiveStatus)
	assert.False(t, ready)

	effectiveStatus, ready = status.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusAvailable, effectiveStatus)
	assert.True(t, ready)
}
