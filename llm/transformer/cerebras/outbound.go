package cerebras

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openrouter"
)

// DefaultBaseURL is the default Cerebras API base URL.
const DefaultBaseURL = "https://api.cerebras.ai/v1"

// Config holds all configuration for the Cerebras outbound transformer.
type Config struct {
	// BaseURL is the base URL for the Cerebras API.
	BaseURL string `json:"base_url,omitempty"`
	// APIKeyProvider provides API keys for authentication.
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

var _ transformer.Outbound = (*OutboundTransformer)(nil)

// OutboundTransformer implements transformer.Outbound for Cerebras.
// It wraps the OpenRouter transformer and strips OpenAI-specific fields
// that Cerebras does not support (e.g. `store`).
type OutboundTransformer struct {
	transformer.Outbound
}

// NewOutboundTransformerWithConfig creates a new Cerebras OutboundTransformer.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config == nil {
		return nil, fmt.Errorf("invalid Cerebras transformer configuration: config is nil")
	}

	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("invalid Cerebras transformer configuration: API key provider is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	t, err := openrouter.NewOutboundTransformerWithConfig(&openrouter.Config{
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid Cerebras transformer configuration: %w", err)
	}

	return &OutboundTransformer{
		Outbound: t,
	}, nil
}

// TransformRequest strips OpenAI-specific fields unsupported by Cerebras
// and delegates to the underlying OpenRouter transformer.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Create a shallow copy to avoid modifying the original request.
	reqCopy := *llmReq
	reqCopy.Store = nil // Cerebras does not support the `store` parameter.

	return t.Outbound.TransformRequest(ctx, &reqCopy)
}
