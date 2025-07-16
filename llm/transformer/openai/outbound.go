package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

// OutboundTransformer implements transformer.Outbound for OpenAI format
type OutboundTransformer struct {
	name string
	baseURL string
	apiKey string
}

// NewOutboundTransformer creates a new OpenAI OutboundTransformer
func NewOutboundTransformer(baseURL, apiKey string) transformer.Outbound {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	
	return &OutboundTransformer{
		name:    "openai-outbound",
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// TransformRequest transforms ChatCompletionRequest to GenericHttpRequest
func (t *OutboundTransformer) TransformRequest(ctx context.Context, chatReq *llm.ChatCompletionRequest) (*llm.GenericHttpRequest, error) {
	if chatReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Validate required fields
	if chatReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if len(chatReq.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	// Marshal the request body
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	
	// Add User-Agent
	headers.Set("User-Agent", "axonhub/1.0")

	// Prepare authentication
	var auth *llm.AuthConfig
	if t.apiKey != "" {
		auth = &llm.AuthConfig{
			Type:   "bearer",
			APIKey: t.apiKey,
		}
	}

	// Determine endpoint based on streaming
	endpoint := "/chat/completions"
	url := strings.TrimSuffix(t.baseURL, "/") + endpoint

	return &llm.GenericHttpRequest{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

// TransformResponse transforms GenericHttpResponse to ChatCompletionResponse
func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *llm.GenericHttpResponse) (*llm.ChatCompletionResponse, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check for HTTP errors
	if httpResp.StatusCode >= 400 {
		if httpResp.Error != nil {
			return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, httpResp.Error.Message)
		}
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var chatResp llm.ChatCompletionResponse
	if err := json.Unmarshal(httpResp.Body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %w", err)
	}

	return &chatResp, nil
}

// Name returns the transformer name
func (t *OutboundTransformer) Name() string {
	return t.name
}

// SupportsModel checks if the transformer supports a specific model
func (t *OutboundTransformer) SupportsModel(model string) bool {
	// OpenAI transformer supports OpenAI models
	openaiModels := []string{
		"gpt-4", "gpt-4-turbo", "gpt-4o", "gpt-4o-mini",
		"gpt-3.5-turbo", "gpt-3.5-turbo-16k",
		"text-davinci-003", "text-davinci-002",
	}
	
	for _, supportedModel := range openaiModels {
		if strings.HasPrefix(model, supportedModel) {
			return true
		}
	}
	
	return false
}

// SetAPIKey updates the API key
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.apiKey = apiKey
}

// SetBaseURL updates the base URL
func (t *OutboundTransformer) SetBaseURL(baseURL string) {
	t.baseURL = baseURL
}