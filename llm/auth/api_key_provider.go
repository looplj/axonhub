package auth

import (
	"context"
	"math/rand/v2"
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

// RandomKeyProvider is an APIKeyProvider that randomly selects from multiple API keys.
// This is useful for load balancing across multiple API keys.
type RandomKeyProvider struct {
	apiKeys []string
}

// NewRandomKeyProvider creates a new RandomKeyProvider with the given API keys.
// Panics if no keys are provided.
func NewRandomKeyProvider(apiKeys []string) *RandomKeyProvider {
	if len(apiKeys) == 0 {
		panic("no API keys provided")
	}

	return &RandomKeyProvider{
		apiKeys: apiKeys,
	}
}

// Get returns a randomly selected API key.
func (p *RandomKeyProvider) Get(_ context.Context) string {
	if len(p.apiKeys) == 1 {
		return p.apiKeys[0]
	}

	//nolint:gosec // not a security issue, just a random selection.
	return p.apiKeys[rand.IntN(len(p.apiKeys))]
}
