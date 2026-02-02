package auth

import (
	"context"
)

// APIKeyProvider provides API keys for authentication.
// Implementations can support single or multiple API keys with various selection strategies.
type APIKeyProvider interface {
	// Get returns an API key for the given context.
	// The context may contain hints (e.g., trace ID, session ID) that implementations
	// can use to ensure consistent key selection for related requests.
	Get(ctx context.Context) string
}

// StaticKeyProvider is a simple APIKeyProvider that always returns the same API key.
type StaticKeyProvider struct {
	apiKey string
}

// NewStaticKeyProvider creates a new StaticKeyProvider with the given API key.
func NewStaticKeyProvider(apiKey string) *StaticKeyProvider {
	return &StaticKeyProvider{apiKey: apiKey}
}

// Get returns the static API key.
func (p *StaticKeyProvider) Get(_ context.Context) string {
	return p.apiKey
}
