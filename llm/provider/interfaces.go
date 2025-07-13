package provider

import (
	"context"

	"github.com/looplj/axonhub/llm/types"
)

// Provider represents a unified interface that combines HTTP client and transformation logic
type Provider interface {
	// Name returns the provider name
	Name() string

	// ChatCompletion sends a chat completion request and returns the response
	ChatCompletion(ctx context.Context, request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error)

	// ChatCompletionStream sends a streaming chat completion request
	ChatCompletionStream(ctx context.Context, request *types.ChatCompletionRequest) (Stream[*types.ChatCompletionResponse], error)

	// SupportsModel checks if the provider supports a specific model
	SupportsModel(model string) bool

	// GetConfig returns the provider configuration
	GetConfig() *types.ProviderConfig

	// SetConfig updates the provider configuration
	SetConfig(config *types.ProviderConfig)
}

// ProviderRegistry manages multiple providers
type ProviderRegistry interface {
	// RegisterProvider registers a provider
	RegisterProvider(name string, provider Provider)

	// GetProvider retrieves a provider by name
	GetProvider(name string) (Provider, error)

	// ListProviders returns all registered provider names
	ListProviders() []string

	// UnregisterProvider removes a provider
	UnregisterProvider(name string)

	// GetProviderForModel returns the appropriate provider for a model
	GetProviderForModel(model string) (Provider, error)

	RegisterModelMapping(model string, providerName string)
}
