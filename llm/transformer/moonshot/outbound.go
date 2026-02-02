package moonshot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config holds all configuration for the Moonshot outbound transformer.
type Config struct {
	// API configuration
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKey  string `json:"api_key,omitempty"`  // API key
}

// OutboundTransformer implements transformer.Outbound for Moonshot format.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL string
	APIKey  string
}

// NewOutboundTransformer creates a new Moonshot OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Moonshot OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	t, err := openai.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("invalid Moonshot transformer configuration: %w", err)
	}

	baseURL := transformer.NormalizeBaseURL(config.BaseURL, "v1")

	return &OutboundTransformer{
		BaseURL:  baseURL,
		APIKey:   config.APIKey,
		Outbound: t,
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
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Convert llm.Request to openai.Request first
	oaiReq := openai.RequestFromLLM(llmReq)

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

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: t.APIKey,
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
