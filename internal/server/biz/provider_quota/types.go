package provider_quota

import (
	"context"
	"net/http"
	"time"

	"github.com/looplj/axonhub/internal/ent"
)

// QuotaParser extracts quota data from provider responses
type QuotaParser interface {
	// ParseResponse extracts quota data from HTTP response
	ParseResponse(headers http.Header, body []byte) (QuotaData, error)

	// GetProviderType returns the provider type this parser handles
	GetProviderType() string
}

// QuotaChecker makes API requests to check quota status
type QuotaChecker interface {
	// CheckQuota makes a minimal API request to get quota information
	CheckQuota(ctx context.Context, channel *ent.Channel) (http.Header, []byte, error)

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
