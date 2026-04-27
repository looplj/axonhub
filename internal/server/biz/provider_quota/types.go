package provider_quota

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/ent"
)

type QuotaChecker interface {
	CheckQuota(ctx context.Context, channel *ent.Channel) (QuotaData, error)
	SupportsChannel(channel *ent.Channel) bool
}

type QuotaLimitType string

const (
	QuotaLimitTypeImage QuotaLimitType = "image"
	QuotaLimitTypeToken QuotaLimitType = "token"
)

type QuotaLimitStatus struct {
	Type        QuotaLimitType `json:"type"`
	Status      string         `json:"status"`
	UsageRatio  float64       `json:"usage_ratio"`
	Ready       bool           `json:"ready"`
	NextResetAt *time.Time    `json:"next_reset_at"`
}

type QuotaData struct {
	Status       string             `json:"status"`
	ProviderType string             `json:"provider_type"`
	RawData      map[string]any     `json:"raw_data"`
	NextResetAt  *time.Time         `json:"next_reset_at"`
	Ready        bool               `json:"ready"`
	Limits       []QuotaLimitStatus `json:"limits"`
}

func RequestModality(isImageRequest bool) QuotaLimitType {
	if isImageRequest {
		return QuotaLimitTypeImage
	}
	return QuotaLimitTypeToken
}
