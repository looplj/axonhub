package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

const (
	// DefaultBaseURL is the default base URL for Gemini API.
	DefaultBaseURL = "https://generativelanguage.googleapis.com"

	// DefaultAPIVersion is the default API version.
	DefaultAPIVersion = "v1beta"

	// PlatformVertex indicates Vertex AI platform.
	PlatformVertex = "vertex"
)

// Config holds all configuration for the Gemini outbound transformer.
type Config struct {
	// BaseURL is the base URL for the Gemini API.
	BaseURL string `json:"base_url,omitempty"`

	// APIKey is the API key for authentication.
	APIKey string `json:"api_key,omitempty"`

	// APIVersion is the API version to use.
	APIVersion string `json:"api_version,omitempty"`

	// PlatformType distinguishes different platform configurations (e.g., "vertex").
	PlatformType string `json:"platform_type,omitempty"`

	// ReasoningEffortToBudget maps reasoning effort levels to thinking budget tokens.
	ReasoningEffortToBudget map[string]int64 `json:"reasoning_effort_to_budget,omitempty"`
}

// OutboundTransformer implements transformer.Outbound for Gemini format.
type OutboundTransformer struct {
	config Config
}

// NewOutboundTransformer creates a new Gemini OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

func clenupConfig(config Config) Config {
	if config.BaseURL == "" {
		config.BaseURL = strings.TrimSuffix(DefaultBaseURL, "/")
	}

	if config.APIVersion == "" {
		config.APIVersion = DefaultAPIVersion
		config.BaseURL = strings.TrimSuffix(config.BaseURL, "/v1beta")
		config.BaseURL = strings.TrimSuffix(config.BaseURL, "/v1")
	} else {
		if strings.HasSuffix(config.BaseURL, "/v1beta") {
			config.APIVersion = "v1beta"
			config.BaseURL = strings.TrimSuffix(config.BaseURL, "/v1beta")
		}

		if strings.HasSuffix(config.BaseURL, "/v1") {
			config.APIVersion = "v1"
			config.BaseURL = strings.TrimSuffix(config.BaseURL, "/v1")
		}
	}

	return config
}

// NewOutboundTransformerWithConfig creates a new Gemini OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config Config) (transformer.Outbound, error) {
	config = clenupConfig(config)

	return &OutboundTransformer{
		config: config,
	}, nil
}

// APIFormat returns the API format of the transformer.
func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatGeminiContents
}

// TransformRequest transforms the unified request to Gemini HTTP request.
func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("request is nil")
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeChat, "":
		// continue
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	if llmReq.Model == "" {
		return nil, fmt.Errorf("model is required,%v", llmReq.Model)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required,%v", transformer.ErrInvalidRequest, llmReq.Messages)
	}

	// Convert to Gemini request format
	geminiReq := convertLLMToGeminiRequest(llmReq)

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Prepare authentication
	var auth *httpclient.AuthConfig
	if t.config.APIKey != "" {
		auth = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    t.config.APIKey,
			HeaderKey: "x-goog-api-key",
		}
	}

	if llmReq.RawRequest != nil {
		// We need to remove the alt query parameter to avoid passing through to the backend.
		llmReq.RawRequest.Query = nil
	}

	// Build URL
	url := t.buildFullRequestURL(llmReq)

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

// buildFullRequestURL constructs the appropriate URL for the Gemini API.
func (t *OutboundTransformer) buildFullRequestURL(llmReq *llm.Request) string {
	// Determine endpoint based on streaming
	var action string
	if llmReq.Stream != nil && *llmReq.Stream {
		// Use SSE for streaming.
		action = "streamGenerateContent?alt=sse"
	} else {
		action = "generateContent"
	}

	version := t.config.APIVersion
	if version == "" {
		version = DefaultAPIVersion

		if llmReq.RawRequest != nil && llmReq.RawRequest.RawRequest != nil {
			requestVersion := llmReq.RawRequest.RawRequest.PathValue("gemini-api-version")
			if requestVersion != "" {
				version = requestVersion
			}
		}
	}

	// For Vertex AI platform, use different URL format:
	// https://${API_ENDPOINT}/v1/publishers/google/models/${MODEL_ID}:${ACTION}?key=${API_KEY}
	// If base URL starts with Cloudflare gateway, don't add /v1 prefix
	if t.config.PlatformType == PlatformVertex {
		baseURL := strings.TrimSuffix(t.config.BaseURL, "/")
		if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
			return fmt.Sprintf("%s/publishers/google/models/%s:%s", baseURL, llmReq.Model, action)
		}
		return fmt.Sprintf("%s/v1/publishers/google/models/%s:%s", baseURL, llmReq.Model, action)
	}

	// Format: /base_url/{version}/models/{model}:generateContent
	return fmt.Sprintf("%s/%s/models/%s:%s", t.config.BaseURL, version, llmReq.Model, action)
}

// TransformResponse transforms the Gemini HTTP response to unified response format.
func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check for HTTP error status
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var geminiResp GenerateContentResponse
	if err := json.Unmarshal(httpResp.Body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gemini response: %w", err)
	}

	// Convert to unified response (non-streaming)
	return convertGeminiToLLMResponse(&geminiResp, false), nil
}

// TransformError transforms HTTP error response to unified error response for Gemini.
func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	if rawErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: "Request failed.",
				Type:    "api_error",
			},
		}
	}

	var geminiErr GeminiError
	if err := json.Unmarshal(rawErr.Body, &geminiErr); err == nil && geminiErr.Error.Message != "" {
		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail: llm.ErrorDetail{
				Type:    geminiErr.Error.Status,
				Message: geminiErr.Error.Message,
				Code:    fmt.Sprintf("%d", geminiErr.Error.Code),
			},
		}
	}

	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: string(rawErr.Body),
			Type:    "api_error",
		},
	}
}

// SetAPIKey updates the API key.
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.config.APIKey = apiKey
}

// SetBaseURL updates the base URL.
func (t *OutboundTransformer) SetBaseURL(baseURL string) {
	t.config.BaseURL = baseURL
}
