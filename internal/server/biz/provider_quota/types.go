package provider_quota

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/ent"
)

// QuotaChecker checks quota status for a provider
type QuotaChecker interface {
	// CheckQuota makes a minimal API request to get quota information and returns parsed quota data
	CheckQuota(ctx context.Context, channel *ent.Channel) (QuotaData, error)

	// SupportsChannel returns true if this checker supports the channel
	SupportsChannel(channel *ent.Channel) bool
}

// QuotaData is the unified quota data structure
type QuotaData struct {
	Status       string                 `json:"status"` // available, warning, exhausted, unknown
	ProviderType string                 `json:"provider_type"`
	RawData      map[string]interface{} `json:"raw_data"`
	NextResetAt  *time.Time             `json:"next_reset_at"` // Next quota reset timestamp
	Ready        bool                   `json:"ready"`         // True if status is available or warning
}
