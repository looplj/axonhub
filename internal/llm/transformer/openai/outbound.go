package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// PlatformType represents the platform type for OpenAI API.
type PlatformType string

const (
	PlatformOpenAI PlatformType = "openai" // Standard OpenAI API
	PlatformAzure  PlatformType = "azure"  // Azure OpenAI
)

const DefaultAzureAPIVersion = "2025-04-01-preview"

// Config holds all configuration for the OpenAI outbound transformer.
type Config struct {
	// Platform configuration
	Type PlatformType `json:"type"`

	// API configuration
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKey  string `json:"api_key,omitempty"`  // API key

	// Azure-specific configuration
	APIVersion string `json:"api_version,omitempty"` // Azure API version (required for Azure)
}

// OutboundTransformer implements transformer.Outbound for OpenAI format.
type OutboundTransformer struct {
	config *Config
}

// NewOutboundTransformer creates a new OpenAI OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		Type:    PlatformOpenAI,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	err := validateConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenAI transformer configuration: %w", err)
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new OpenAI OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	err := validateConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenAI transformer configuration: %w", err)
	}

	return &OutboundTransformer{
		config: config,
	}, nil
}

// validateConfig validates the configuration for the given platform.
func validateConfig(config *Config) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}

	// Standard OpenAI validation
	if config.APIKey == "" {
		return errors.New("API key is required")
	}

	if config.BaseURL == "" {
		return errors.New("base URL is required")
	}

	switch config.Type {
	case PlatformOpenAI:
		return nil
	case PlatformAzure:
		if config.APIVersion == "" {
			return fmt.Errorf("API version is required for Azure platform")
		}
	default:
		return fmt.Errorf("unsupported platform type: %v", config.Type)
	}

	return nil
}

// APIFormat returns the API format of the transformer.
func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIChatCompletion
}

// TransformRequest transforms ChatCompletionRequest to Request.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	chatReq *llm.Request,
) (*httpclient.Request, error) {
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

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	var auth *httpclient.AuthConfig

	//nolint:exhaustive // Chcked.
	switch t.config.Type {
	case PlatformAzure:
		auth = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    t.config.APIKey,
			HeaderKey: "Api-Key",
		}
	default:
		auth = &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: t.config.APIKey,
		}
	}

	// Build platform-specific URL
	url, err := t.buildPlatformURL(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to build platform URL: %w", err)
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

// TransformResponse transforms Response to ChatCompletionResponse.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check for HTTP error status codes
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var chatResp Response

	err := json.Unmarshal(httpResp.Body, &chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %w", err)
	}

	return chatResp.ToLLMResponse(), nil
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	}), nil
}

func (t *OutboundTransformer) TransformStreamChunk(
	ctx context.Context,
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
		return llm.DoneResponse, nil
	}

	ep := gjson.GetBytes(event.Data, "error")
	if ep.Exists() {
		return nil, &llm.ResponseError{
			Detail: llm.ErrorDetail{
				Message: ep.String(),
			},
		}
	}

	// Create a synthetic HTTP response for compatibility with existing logic
	httpResp := &httpclient.Response{
		Body: event.Data,
	}

	return t.TransformResponse(ctx, httpResp)
}

// buildPlatformURL constructs the appropriate URL based on the platform.
func (t *OutboundTransformer) buildPlatformURL(chatReq *llm.Request) (string, error) {
	baseURL := strings.TrimSuffix(t.config.BaseURL, "/")

	//nolint:exhaustive // Chcked.
	switch t.config.Type {
	case PlatformAzure:
		// Build the Azure OpenAI URL
		azureURL := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
			baseURL, chatReq.Model, t.config.APIVersion)

		return azureURL, nil
	default:
		// Standard OpenAI API
		return baseURL + "/chat/completions", nil
	}
}

// SetAPIKey updates the API key.
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.config.APIKey = apiKey

	// Validate configuration after updating API key
	err := validateConfig(t.config)
	if err != nil {
		panic(fmt.Sprintf("invalid OpenAI transformer configuration after setting API key: %v", err))
	}
}

// SetBaseURL updates the base URL.
func (t *OutboundTransformer) SetBaseURL(baseURL string) {
	t.config.BaseURL = baseURL

	// Validate configuration after updating base URL
	err := validateConfig(t.config)
	if err != nil {
		panic(fmt.Sprintf("invalid OpenAI transformer configuration after setting base URL: %v", err))
	}
}

// SetConfig updates the entire configuration.
func (t *OutboundTransformer) SetConfig(config *Config) {
	// Validate configuration before setting
	err := validateConfig(config)
	if err != nil {
		panic(fmt.Sprintf("invalid OpenAI transformer configuration: %v", err))
	}

	t.config = config
}

// ConfigureForAzure configures the transformer for Azure OpenAI.
func (t *OutboundTransformer) ConfigureForAzure(resourceName, apiVersion, apiKey string) error {
	// Create new Azure configuration
	newConfig := &Config{
		Type:       PlatformAzure,
		APIVersion: apiVersion,
		APIKey:     apiKey,
	}

	// Set base URL only if resource name is provided
	if resourceName != "" {
		newConfig.BaseURL = fmt.Sprintf("https://%s.openai.azure.com", resourceName)
	}

	// Validate the new configuration
	err := validateConfig(newConfig)
	if err != nil {
		return fmt.Errorf("invalid Azure configuration: %w", err)
	}

	// Apply the validated configuration
	t.config = newConfig

	return nil
}

// GetConfig returns the current configuration.
func (t *OutboundTransformer) GetConfig() *Config {
	return t.config
}

func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks, DefaultTransformChunk)
}

// TransformError transforms HTTP error response to unified error response.
func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	if rawErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
				Type:    "api_error",
			},
		}
	}

	// Try to parse as OpenAI error format first
	var openaiError struct {
		Error llm.ErrorDetail `json:"error"`
	}

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil && openaiError.Error.Message != "" {
		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail:     openaiError.Error,
		}
	}

	// If JSON parsing fails, return the JSON error message
	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: http.StatusText(http.StatusInternalServerError),
			Type:    "api_error",
		},
	}
}
