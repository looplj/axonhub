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

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// PlatformType represents the platform type for OpenAI API.
type PlatformType string

const (
	PlatformOpenAI PlatformType = "openai"
	PlatformAzure  PlatformType = "azure"
)

const DefaultAzureAPIVersion = "2025-04-01-preview"

// Config holds all configuration for the OpenAI outbound transformer.
type Config struct {
	// Platform configuration
	PlatformType PlatformType `json:"type"`

	// BaseURL is the base URL for the OpenAI API, required.
	BaseURL string `json:"base_url,omitempty"`

	// RawURL is whether to use raw URL for requests, default is false.
	// If true, the request URL will be used as is, without appending the chat completions endpoint.
	RawURL bool `json:"raw_url,omitempty"`

	// APIKeyProvider provides API keys for authentication, required.
	APIKeyProvider auth.APIKeyProvider `json:"-"`

	// APIVersion is the API version for Azure platform, required for Azure.
	APIVersion string `json:"api_version,omitempty"`
}

// OutboundTransformer implements transformer.Outbound for OpenAI format.
type OutboundTransformer struct {
	config *Config
}

// NewOutboundTransformer creates a new OpenAI OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		PlatformType:   PlatformOpenAI,
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
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

	if strings.HasSuffix(config.BaseURL, "##") {
		config.RawURL = true
		config.BaseURL = strings.TrimSuffix(config.BaseURL, "##")
	} else if !config.RawURL {
		// For Azure, don't normalize with version - it has special URL format
		if config.PlatformType == PlatformAzure {
			config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "")
		} else {
			config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "v1")
		}
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
	if config.APIKeyProvider == nil {
		return errors.New("API key provider is required")
	}

	if config.BaseURL == "" {
		return errors.New("base URL is required")
	}

	switch config.PlatformType {
	case PlatformOpenAI:
		return nil
	case PlatformAzure:
		if config.APIVersion == "" {
			return fmt.Errorf("API version is required for Azure platform")
		}
	default:
		return fmt.Errorf("unsupported platform type: %v", config.PlatformType)
	}

	return nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIChatCompletion
}

// TransformRequest transforms ChatCompletionRequest to Request.
func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Validate required fields for chat requests
	if llmReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeEmbedding:
		return t.transformEmbeddingRequest(ctx, llmReq)
	case llm.RequestTypeImage:
		//nolint:exhaustive // Checked.
		switch t.config.PlatformType {
		case PlatformAzure:
			return nil, fmt.Errorf("image generation via Image Generation API is not yet supported for Azure platform")
		default:
			// ok
		}

		return t.buildImageGenerationAPIRequest(ctx, llmReq)
	case llm.RequestTypeRerank:
		return nil, fmt.Errorf("%w: rerank is not supported", transformer.ErrInvalidRequest)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Convert to OpenAI Request format (this strips helper fields)
	oaiReq := RequestFromLLM(llmReq)

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Get API key from provider
	apiKey := t.config.APIKeyProvider.Get(ctx)

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	var authConfig *httpclient.AuthConfig

	//nolint:exhaustive // Chcked.
	switch t.config.PlatformType {
	case PlatformAzure:
		authConfig = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    apiKey,
			HeaderKey: "Api-Key",
		}
	default:
		authConfig = &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: apiKey,
		}
	}

	// Build platform-specific URL
	url, err := t.buildFullRequestURL(llmReq)
	if err != nil {
		return nil, fmt.Errorf("failed to build platform URL: %w", err)
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    authConfig,
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

	// Route to specialized transformers based on request APIFormat
	if httpResp.Request != nil && httpResp.Request.APIFormat != "" {
		switch httpResp.Request.APIFormat {
		case string(llm.APIFormatOpenAIImageGeneration),
			string(llm.APIFormatOpenAIImageEdit),
			string(llm.APIFormatOpenAIImageVariation):
			return transformImageGenerationResponse(httpResp)
		case string(llm.APIFormatOpenAIEmbedding):
			return t.transformEmbeddingResponse(ctx, httpResp)
		}
	}

	// Parse into OpenAI Response type
	var oaiResp Response

	err := json.Unmarshal(httpResp.Body, &oaiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %w", err)
	}

	// Convert to unified llm.Response
	return oaiResp.ToLLMResponse(), nil
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

// buildFullRequestURL constructs the appropriate URL based on the platform.
func (t *OutboundTransformer) buildFullRequestURL(_ *llm.Request) (string, error) {
	//nolint:exhaustive // Checked.
	switch t.config.PlatformType {
	case PlatformAzure:
		if strings.HasSuffix(t.config.BaseURL, "/openai/v1") {
			// Azure URL already includes /openai/v1
			return fmt.Sprintf("%s/chat/completions?api-version=%s",
				t.config.BaseURL, t.config.APIVersion), nil
		}

		if strings.HasSuffix(t.config.BaseURL, "/openai") {
			// Azure URL includes /openai but not /v1
			return fmt.Sprintf("%s/v1/chat/completions?api-version=%s",
				t.config.BaseURL, t.config.APIVersion), nil
		}
		// Default case for other Azure URLs
		return fmt.Sprintf("%s/openai/v1/chat/completions?api-version=%s",
			t.config.BaseURL, t.config.APIVersion), nil
	default:
		if t.config.RawURL {
			return t.config.BaseURL, nil
		}
		return t.config.BaseURL + "/chat/completions", nil
	}
}

// SetAPIKey updates the API key.
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.config.APIKeyProvider = auth.NewStaticKeyProvider(apiKey)
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
		Error  llm.ErrorDetail `json:"error"`
		Errors llm.ErrorDetail `json:"errors"`
	}

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil && (openaiError.Error.Message != "" || openaiError.Errors.Message != "") {
		errDetail := openaiError.Error
		if errDetail.Message == "" {
			errDetail = openaiError.Errors
		}

		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail:     errDetail,
		}
	}

	// If JSON parsing fails, use the upstream status text
	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: http.StatusText(rawErr.StatusCode),
			Type:    "api_error",
		},
	}
}
