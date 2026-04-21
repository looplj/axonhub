package fireworks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// DefaultBaseURL is the default Fireworks API base URL.
const DefaultBaseURL = "https://api.fireworks.ai/inference/v1"

// Config holds all configuration for the Fireworks outbound transformer.
type Config struct {
	// BaseURL is the base URL for the Fireworks API.
	BaseURL string `json:"base_url,omitempty"`
	// APIKeyProvider provides API keys for authentication.
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

// OutboundTransformer implements transformer.Outbound for Fireworks format.
// Fireworks API is OpenAI-compatible but does not support reasoning content fields.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL        string
	APIKeyProvider auth.APIKeyProvider
}

// NewOutboundTransformer creates a new Fireworks OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Fireworks OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config == nil {
		return nil, fmt.Errorf("invalid Fireworks transformer configuration: config is nil")
	}

	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("invalid Fireworks transformer configuration: API key provider is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
	}

	baseTransformer, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid Fireworks transformer configuration: %w", err)
	}

	return &OutboundTransformer{
		Outbound:       baseTransformer,
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
	}, nil
}

// TransformRequest transforms ChatCompletionRequest to Fireworks-compatible Request.
// It strips reasoning-related fields that Fireworks API does not support.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Validate required fields
	if llmReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Convert to OpenAI Request format
	oaiReq := openai.RequestFromLLM(llmReq)

	// Strip reasoning fields that Fireworks API does not support
	stripReasoningFromMessages(oaiReq)

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to transform request: %w", transformer.ErrInvalidRequest, err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Get API key from provider
	apiKey := t.APIKeyProvider.Get(ctx)

	auth := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	url := t.BaseURL + "/chat/completions"

	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		Headers:     headers,
		Body:        body,
		Auth:        auth,
		ContentType: "application/json",
		APIFormat:   string(llm.APIFormatOpenAIChatCompletion),
	}, nil
}

func stripReasoningFromMessages(req *openai.Request) {
	if req == nil {
		return
	}

	for i := range req.Messages {
		msg := &req.Messages[i]
		msg.Reasoning = nil
		msg.ReasoningContent = nil
	}
}

// TransformResponse transforms Response to ChatCompletionResponse.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	return t.Outbound.TransformResponse(ctx, httpResp)
}

// TransformStream transforms the streaming response.
func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return t.Outbound.TransformStream(ctx, stream)
}

// AggregateStreamChunks aggregates streaming chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return t.Outbound.AggregateStreamChunks(ctx, chunks)
}

// TransformError transforms HTTP error response to unified error response.
func (t *OutboundTransformer) TransformError(
	ctx context.Context,
	rawErr *httpclient.Error,
) *llm.ResponseError {
	return t.Outbound.TransformError(ctx, rawErr)
}
