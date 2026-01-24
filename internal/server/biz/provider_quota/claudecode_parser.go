package provider_quota

import (
	"net/http"
	"strconv"
)

type ClaudeCodeQuotaParser struct{}

func (p *ClaudeCodeQuotaParser) ParseResponse(headers http.Header, body []byte) (QuotaData, error) {
	// Guard clause - early return if no quota headers
	if headers.Get("anthropic-ratelimit-unified-status") == "" {
		return QuotaData{}, nil
	}

	status := headers.Get("anthropic-ratelimit-unified-status")

	rawData := map[string]interface{}{
		"unified_status": status,
		"windows": map[string]interface{}{
			"5h": map[string]interface{}{
				"status":      headers.Get("anthropic-ratelimit-unified-5h-status"),
				"reset":       parseUnixTimestamp(headers.Get("anthropic-ratelimit-unified-5h-reset")),
				"utilization": parseFloat(headers.Get("anthropic-ratelimit-unified-5h-utilization")),
			},
			"7d": map[string]interface{}{
				"status":      headers.Get("anthropic-ratelimit-unified-7d-status"),
				"reset":       parseUnixTimestamp(headers.Get("anthropic-ratelimit-unified-7d-reset")),
				"utilization": parseFloat(headers.Get("anthropic-ratelimit-unified-7d-utilization")),
			},
			"overage": map[string]interface{}{
				"status":      headers.Get("anthropic-ratelimit-unified-overage-status"),
				"reset":       parseUnixTimestamp(headers.Get("anthropic-ratelimit-unified-overage-reset")),
				"utilization": parseFloat(headers.Get("anthropic-ratelimit-unified-overage-utilization")),
			},
		},
		"representative_claim": headers.Get("anthropic-ratelimit-unified-representative-claim"),
		"fallback":             headers.Get("anthropic-ratelimit-unified-fallback"),
		"fallback_percentage":  parseFloat(headers.Get("anthropic-ratelimit-unified-fallback-percentage")),
		"reset":                parseUnixTimestamp(headers.Get("anthropic-ratelimit-unified-reset")),
	}

	return QuotaData{
		Status:       status,
		ProviderType: "claudecode",
		RawData:      rawData,
	}, nil
}

func (p *ClaudeCodeQuotaParser) GetProviderType() string {
	return "claudecode"
}

// Defensive parsing - silently default to zero on error
func parseUnixTimestamp(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
