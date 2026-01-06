package modelscope

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config holds all configuration for the ModelScope outbound transformer.
type Config struct {
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKey  string `json:"api_key,omitempty"`  // API key
}

// OutboundTransformer implements transformer.Outbound for ModelScope format.
type OutboundTransformer struct {
	transformer.Outbound
}

// NewOutboundTransformer creates a new ModelScope OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new ModelScope OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	t, err := openai.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("invalid ModelScope transformer configuration: %w", err)
	}

	return &OutboundTransformer{
		Outbound: t,
	}, nil
}

// TransformRequest transforms ChatCompletionRequest to Request.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	chatReq *llm.Request,
) (*httpclient.Request, error) {
	// Create a shallow copy to avoid modifying the original request.
	reqCopy := *chatReq
	reqCopy.Metadata = nil // model scope does not support metadata.

	return t.Outbound.TransformRequest(ctx, &reqCopy)
}
