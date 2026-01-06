package bailian

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// Config holds all configuration for the Bailian outbound transformer.
type Config struct {
	BaseURL string `json:"base_url,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
}

// OutboundTransformer implements transformer.Outbound for Bailian (OpenAI-compatible) format.
type OutboundTransformer struct {
	transformer.Outbound
}

// NewOutboundTransformer creates a new Bailian OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Bailian OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config == nil {
		return nil, fmt.Errorf("invalid Bailian transformer configuration: config is nil")
	}

	base, err := openai.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("invalid Bailian transformer configuration: %w", err)
	}

	return &OutboundTransformer{Outbound: base}, nil
}

// TransformStream applies Bailian-specific streaming normalization on top of OpenAI-compatible stream.
func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	baseStream, err := t.Outbound.TransformStream(ctx, stream)
	if err != nil {
		return nil, err
	}

	return newBailianStreamFilter(baseStream), nil
}
