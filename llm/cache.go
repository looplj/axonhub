package llm

// CacheControl represents cache control configuration.
// This field is used internally for provider-specific cache control
// and should not be serialized in the standard llm JSON format.
type CacheControl struct {
	Type string `json:"type,omitempty"`
	TTL  string `json:"ttl,omitempty"`
}
