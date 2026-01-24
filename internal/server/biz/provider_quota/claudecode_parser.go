package provider_quota

import (
	"net/http"
	"strconv"
	"time"
)

type ClaudeCodeQuotaParser struct{}

func (p *ClaudeCodeQuotaParser) ParseResponse(headers http.Header, body []byte) (QuotaData, error) {
	// Guard clause - early return if no quota headers
	if headers.Get("anthropic-ratelimit-unified-status") == "" {
		return QuotaData{}, nil
	}

	unifiedStatus := headers.Get("anthropic-ratelimit-unified-status")
	representativeClaim := headers.Get("anthropic-ratelimit-unified-representative-claim")

	// Parse window data
	windows := map[string]interface{}{
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
	}

	rawData := map[string]interface{}{
		"unified_status":       unifiedStatus,
		"windows":              windows,
		"representative_claim": representativeClaim,
		"fallback":             headers.Get("anthropic-ratelimit-unified-fallback"),
		"fallback_percentage":  parseFloat(headers.Get("anthropic-ratelimit-unified-fallback-percentage")),
		"reset":                parseUnixTimestamp(headers.Get("anthropic-ratelimit-unified-reset")),
	}

	// Normalize status: allowed -> available, throttled/rejected -> exhausted
	normalizedStatus := "unknown"
	switch unifiedStatus {
	case "allowed":
		normalizedStatus = "available"
	case "throttled", "rejected":
		normalizedStatus = "exhausted"
	default:
		normalizedStatus = "unknown"
	}

	// Check for warning state (utilization >= 80% on any window)
	if normalizedStatus == "available" {
		fiveHourUtilization := parseFloat(headers.Get("anthropic-ratelimit-unified-5h-utilization"))
		sevenDayUtilization := parseFloat(headers.Get("anthropic-ratelimit-unified-7d-utilization"))
		if fiveHourUtilization >= 0.8 || sevenDayUtilization >= 0.8 {
			normalizedStatus = "warning"
		}
	}

	// Extract next reset time based on representative claim
	// Map representative claim to window key: "five_hour" -> "5h", "seven_day" -> "7d"
	windowKey := representativeClaim
	switch representativeClaim {
	case "five_hour":
		windowKey = "5h"
	case "seven_day":
		windowKey = "7d"
	}

	var nextResetAt *time.Time
	if resetWindow, ok := windows[windowKey].(map[string]interface{}); ok {
		if resetTs, exists := resetWindow["reset"].(int64); exists && resetTs > 0 {
			t := time.Unix(resetTs, 0)
			nextResetAt = &t
		}
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: "claudecode",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        normalizedStatus == "available" || normalizedStatus == "warning",
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
