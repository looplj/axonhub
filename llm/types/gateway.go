package types

import "time"

// GatewayMetadata contains internal metadata for the gateway
type GatewayMetadata struct {
	RequestID    string            `json:"request_id"`
	UserID       string            `json:"user_id"`
	APIKeyID     string            `json:"api_key_id"`
	Provider     string            `json:"provider"`
	Timestamp    time.Time         `json:"timestamp"`
	Headers      map[string]string `json:"headers"`
	RateLimiting *RateLimitInfo    `json:"rate_limiting,omitempty"`
}

// RateLimitInfo contains rate limiting information
type RateLimitInfo struct {
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
	ResetTime int `json:"reset_time"`
}

// GatewayResponseInfo contains gateway-specific response information
type GatewayResponseInfo struct {
	Provider   string        `json:"provider"`
	Latency    time.Duration `json:"latency"`
	CacheHit   bool          `json:"cache_hit"`
	RetryCount int           `json:"retry_count"`
	CostInfo   *CostInfo     `json:"cost_info,omitempty"`
}

// CostInfo represents cost calculation information
type CostInfo struct {
	InputCost  float64 `json:"input_cost"`
	OutputCost float64 `json:"output_cost"`
	TotalCost  float64 `json:"total_cost"`
	Currency   string  `json:"currency"`
}
