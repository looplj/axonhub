package moonshot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config holds all configuration for the Moonshot outbound transformer.
type Config struct {
	// API configuration
	BaseURL        string              `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKeyProvider auth.APIKeyProvider `json:"-"`                  // API key provider
}

// OutboundTransformer implements transformer.Outbound for Moonshot format.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL        string
	APIKeyProvider auth.APIKeyProvider
}

// NewOutboundTransformer creates a new Moonshot OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Moonshot OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
		ReasoningField: openai.ReasoningFieldContent,
	}

	t, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid Moonshot transformer configuration: %w", err)
	}

	baseURL := transformer.NormalizeBaseURL(config.BaseURL, "v1")

	return &OutboundTransformer{
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
		Outbound:       t,
	}, nil
}

// TransformRequest transforms ChatCompletionRequest to Request.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeChat, "":
		// continue
	case llm.RequestTypeCompact:
		return nil, fmt.Errorf("%w: compact is only supported by OpenAI Responses API", transformer.ErrInvalidRequest)
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Convert llm.Request to openai.Request first
	oaiReq := openai.RequestFromLLM(ctx, llmReq, openai.ReasoningFieldContent)

	// Let Moonshot apply the fixed sampling parameters for these models.
	if hasFixedSamplingParams(llmReq.Model) {
		oaiReq.Temperature = nil
		oaiReq.TopP = nil
		oaiReq.PresencePenalty = nil
		oaiReq.FrequencyPenalty = nil
	}

	// Moonshot doesn't support json_schema, convert to json_object
	if oaiReq.ResponseFormat != nil && oaiReq.ResponseFormat.Type == "json_schema" {
		oaiReq.ResponseFormat.Type = "json_object"
		oaiReq.ResponseFormat.JSONSchema = nil
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Get API key from provider
	apiKey := t.APIKeyProvider.Get(ctx)

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: apiKey,
	}

	url := t.BaseURL + "/chat/completions"

	return &httpclient.Request{
		Method:    http.MethodPost,
		URL:       url,
		Headers:   headers,
		Body:      body,
		Auth:      auth,
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
	}, nil
}

// hasFixedSamplingParams reports whether the model rejects client-provided
// temperature, top_p, presence_penalty and frequency_penalty values.
func hasFixedSamplingParams(model string) bool {
	switch model {
	case "kimi-k2.6", "kimi-k2.7-code", "kimi-k2.7-code-highspeed",
		"kimi-k3", "kimi-k3.1-pro", "kimi-k3.1-pro-highspeed":
		return true
	default:
		return false
	}
}
