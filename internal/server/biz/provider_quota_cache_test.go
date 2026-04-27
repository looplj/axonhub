package biz

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderQuotaService_GetQuotaStatus_ReturnsCorrectData(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	svc.quotaCache.Store(1, &QuotaChannelStatus{Status: "available", Ready: true})
	svc.quotaCache.Store(2, &QuotaChannelStatus{Status: "exhausted", Ready: false})
	svc.quotaCache.Store(3, &QuotaChannelStatus{Status: "warning", Ready: true})

	status1 := svc.GetQuotaStatus(1)
	assert.NotNil(t, status1)
	assert.Equal(t, "available", status1.Status)
	assert.True(t, status1.Ready)

	status2 := svc.GetQuotaStatus(2)
	assert.NotNil(t, status2)
	assert.Equal(t, "exhausted", status2.Status)
	assert.False(t, status2.Ready)

	status3 := svc.GetQuotaStatus(3)
	assert.NotNil(t, status3)
	assert.Equal(t, "warning", status3.Status)
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

	svc.updateQuotaCache(1, "available", true)
	svc.updateQuotaCache(2, "exhausted", false)

	status1 := svc.GetQuotaStatus(1)
	assert.NotNil(t, status1)
	assert.Equal(t, "available", status1.Status)
	assert.True(t, status1.Ready)

	status2 := svc.GetQuotaStatus(2)
	assert.NotNil(t, status2)
	assert.Equal(t, "exhausted", status2.Status)
	assert.False(t, status2.Ready)
}

func TestProviderQuotaService_UpdateQuotaCache_Overwrite(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	svc.updateQuotaCache(1, "available", true)
	svc.updateQuotaCache(1, "exhausted", false)

	status := svc.GetQuotaStatus(1)
	assert.NotNil(t, status)
	assert.Equal(t, "exhausted", status.Status)
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
			svc.updateQuotaCache(id, "available", true)
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
		assert.Equal(t, "available", status.Status)
		assert.True(t, status.Ready)
	}
}

func TestProviderQuotaService_ConcurrentReadWrite(t *testing.T) {
	svc := &ProviderQuotaService{
		quotaCache: sync.Map{},
	}

	svc.updateQuotaCache(1, "available", true)

	var wg sync.WaitGroup
	const iterations = 100

	wg.Add(iterations)
	for range iterations {
		go func() {
			defer wg.Done()
			svc.updateQuotaCache(1, "exhausted", false)
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
	assert.Equal(t, "exhausted", status.Status)
	assert.False(t, status.Ready)
}
