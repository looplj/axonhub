package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

func TestQuotaChannelStatus_EffectiveStatus_NoLimits(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusWarning,
		Ready:  true,
		Limits: nil,
	}

	status, ready := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusWarning, status)
	assert.True(t, ready)
}

func TestQuotaChannelStatus_EffectiveStatus_ImageExhausted_TokenAvailable(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusWarning,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeImage, Status: "exhausted", UsageRatio: 1.0, Ready: false},
			{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.3, Ready: true},
		},
	}

	imgStatus, imgReady := s.EffectiveStatus(provider_quota.QuotaLimitTypeImage)
	assert.Equal(t, providerquotastatus.StatusExhausted, imgStatus)
	assert.False(t, imgReady)

	tknStatus, tknReady := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusAvailable, tknStatus)
	assert.True(t, tknReady)
}

func TestQuotaChannelStatus_EffectiveStatus_ImageWarning_DoesNotAffectTokens(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusWarning,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeImage, Status: "warning", UsageRatio: 0.9, Ready: true},
			{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.3, Ready: true},
		},
	}

	imgStatus, _ := s.EffectiveStatus(provider_quota.QuotaLimitTypeImage)
	assert.Equal(t, providerquotastatus.StatusWarning, imgStatus)

	tknStatus, _ := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusAvailable, tknStatus)
}

func TestQuotaChannelStatus_EffectiveStatus_MultipleTokenLimits_WorstWins(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusWarning,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.3, Ready: true},
			{Type: provider_quota.QuotaLimitTypeToken, Status: "warning", UsageRatio: 0.85, Ready: true},
		},
	}

	status, ready := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusWarning, status)
	assert.True(t, ready)
}

func TestQuotaChannelStatus_EffectiveStatus_NoMatchingLimit_Fallback(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusAvailable,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeImage, Status: "exhausted", UsageRatio: 1.0, Ready: false},
		},
	}

	status, ready := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusUnknown, status)
	assert.True(t, ready)
}

func TestQuotaChannelStatus_EffectiveStatus_BothExhausted(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusExhausted,
		Ready:  false,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeImage, Status: "exhausted", UsageRatio: 1.0, Ready: false},
			{Type: provider_quota.QuotaLimitTypeToken, Status: "exhausted", UsageRatio: 1.0, Ready: false},
		},
	}

	imgStatus, imgReady := s.EffectiveStatus(provider_quota.QuotaLimitTypeImage)
	assert.Equal(t, providerquotastatus.StatusExhausted, imgStatus)
	assert.False(t, imgReady)

	tknStatus, tknReady := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusExhausted, tknStatus)
	assert.False(t, tknReady)
}

func TestQuotaChannelStatus_EffectiveStatus_AllLimitsUnknown(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusAvailable,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, Status: "unknown", UsageRatio: 0, Ready: false},
			{Type: provider_quota.QuotaLimitTypeImage, Status: "unknown", UsageRatio: 0, Ready: false},
		},
	}

	tknStatus, tknReady := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusUnknown, tknStatus, "all-unknown limits should return unknown status")
	assert.False(t, tknReady, "all-unknown limits should not be ready")

	imgStatus, imgReady := s.EffectiveStatus(provider_quota.QuotaLimitTypeImage)
	assert.Equal(t, providerquotastatus.StatusUnknown, imgStatus, "all-unknown limits should return unknown status")
	assert.False(t, imgReady, "all-unknown limits should not be ready")
}

func TestEffectiveStatus_ChannelExhaustedOverridesPerLimitAvailable(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusExhausted,
		Ready:  false,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.3, Ready: true},
		},
	}

	status, ready := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusExhausted, status)
	assert.False(t, ready)
}

func TestEffectiveStatus_UnknownFallbackWhenNoMatchingLimitType(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusWarning,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeImage, Status: "exhausted", UsageRatio: 1.0, Ready: false},
		},
	}

	status, ready := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusUnknown, status)
	assert.True(t, ready)
}

func TestEffectiveStatus_EqualRankReadyAggregation(t *testing.T) {
	s := &QuotaChannelStatus{
		Status: providerquotastatus.StatusWarning,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, Status: "warning", UsageRatio: 0.85, Ready: true},
			{Type: provider_quota.QuotaLimitTypeToken, Status: "warning", UsageRatio: 0.90, Ready: false},
		},
	}

	status, ready := s.EffectiveStatus(provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, providerquotastatus.StatusWarning, status)
	assert.False(t, ready)
}

func TestProviderQuotaService_NextCheckIntervalForStatus(t *testing.T) {
	svc := &ProviderQuotaService{
		checkInterval: 5 * time.Minute,
	}

	assert.Equal(t, 5*time.Minute, svc.nextCheckIntervalForStatus("available"), "available should use normal interval")
	assert.Equal(t, 20*time.Minute, svc.nextCheckIntervalForStatus("warning"), "warning should use multiplied interval")
	assert.Equal(t, 5*time.Minute, svc.nextCheckIntervalForStatus("exhausted"), "exhausted should use normal interval")
	assert.Equal(t, 5*time.Minute, svc.nextCheckIntervalForStatus("unknown"), "unknown should use normal interval")
}
