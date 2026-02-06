package deepseek

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

// Config holds all configuration for the DeepSeek outbound transformer.
type Config struct {
	// API configuration
	BaseURL        string              `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKeyProvider auth.APIKeyProvider `json:"-"`                  // API key provider
}

// OutboundTransformer implements transformer.Outbound for DeepSeek format.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL        string
	APIKeyProvider auth.APIKeyProvider
}

// NewOutboundTransformer creates a new DeepSeek OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new DeepSeek OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
	}

	t, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid DeepSeek transformer configuration: %w", err)
	}

	baseURL := transformer.NormalizeBaseURL(config.BaseURL, "v1")

	return &OutboundTransformer{
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
		Outbound:       t,
	}, nil
}

type Request struct {
	openai.Request

	Thinking *Thinking `json:"thinking,omitempty"`
}

type Thinking struct {
	// Enable or disable thinking.
	// enabled | disabled.
	Type string `json:"type"`
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
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	oaiReq := openai.RequestFromLLM(llmReq)

	// DeepSeek doesn't support json_schema, convert to json_object
	if oaiReq.ResponseFormat != nil && oaiReq.ResponseFormat.Type == "json_schema" {
		oaiReq.ResponseFormat.Type = "json_object"
		oaiReq.ResponseFormat.JSONSchema = nil
	}

	dsReq := Request{
		Request: *oaiReq,
	}

	// Convert ReasoningEffort to Thinking if present
	if llmReq.ReasoningEffort != "" && llmReq.ReasoningEffort != "none" {
		dsReq.Thinking = &Thinking{
			Type: "enabled",
		}
	}

	body, err := json.Marshal(dsReq)
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
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}
