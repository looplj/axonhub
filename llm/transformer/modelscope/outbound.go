package modelscope

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config holds all configuration for the ModelScope outbound transformer.
type Config struct {
	BaseURL        string                 `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKeyProvider auth.APIKeyProvider    `json:"-"`                  // API key provider
	HTTPClient     *httpclient.HttpClient `json:"-"`                  // Optional proxy-aware HTTP client
}

// OutboundTransformer implements transformer.Outbound for ModelScope format.
type OutboundTransformer struct {
	transformer.Outbound

	baseURL        string
	apiKeyProvider auth.APIKeyProvider
	httpClient     *httpclient.HttpClient
	pollInterval   time.Duration
	taskTimeout    time.Duration
}

// NewOutboundTransformer creates a new ModelScope OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new ModelScope OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
		ReasoningField: openai.ReasoningFieldContent,
	}

	t, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid ModelScope transformer configuration: %w", err)
	}

	return &OutboundTransformer{
		Outbound:       t,
		baseURL:        config.BaseURL,
		apiKeyProvider: config.APIKeyProvider,
		httpClient:     config.HTTPClient,
		pollInterval:   defaultImageTaskPollInterval,
		taskTimeout:    defaultImageTaskTimeout,
	}, nil
}

// TransformRequest transforms ChatCompletionRequest to Request.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	chatReq *llm.Request,
) (*httpclient.Request, error) {
	if chatReq == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Image generation and editing use the ModelScope async image task API.
	if chatReq.RequestType == llm.RequestTypeImage {
		return t.buildImageRequest(ctx, chatReq)
	}

	// Create a shallow copy to avoid modifying the original request.
	reqCopy := *chatReq
	reqCopy.Metadata = nil // model scope does not support metadata.

	return t.Outbound.TransformRequest(ctx, &reqCopy)
}
