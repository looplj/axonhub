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
	oairesp "github.com/looplj/axonhub/internal/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
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
	rt     *oairesp.OutboundTransformer
}

// NewOutboundTransformer creates a new OpenAI OutboundTransformer with legacy parameters.
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

	config.BaseURL = strings.TrimSuffix(config.BaseURL, "/")

	rt, err := oairesp.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI outbound transformer: %w", err)
	}

	return &OutboundTransformer{
		config: config,
		rt:     rt,
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

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIChatCompletion
}

// TransformRequest transforms ChatCompletionRequest to Request.
func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeEmbedding:
		return t.transformEmbeddingRequest(ctx, llmReq)
	case llm.RequestTypeRerank:
		return nil, fmt.Errorf("%w: rerank is not supported", transformer.ErrInvalidRequest)
	}

	// Validate required fields for chat requests
	if llmReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// If this is an image generation request, use the Image Generation API.
	if llmReq.IsImageGenerationRequest() {
		// Platform routing: For now, only standard OpenAI Image Generation API is supported.
		//nolint:exhaustive // Chcked.
		switch t.config.Type {
		case PlatformAzure:
			return nil, fmt.Errorf("image generation via Image Generation API is not yet supported for Azure platform")
		default:
			// ok
		}

		return t.buildImageGenerationAPIRequest(ctx, llmReq)
	}

	// Convert to OpenAI Request format (this strips helper fields)
	oaiReq := RequestFromLLM(llmReq)

	body, err := json.Marshal(oaiReq)
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
	url, err := t.buildPlatformURL(llmReq)
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

	// Route to specialized transformers based on request metadata
	if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if fmtType, ok := httpResp.Request.TransformerMetadata["outbound_format_type"].(string); ok {
			switch fmtType {
			case llm.APIFormatOpenAIResponse.String():
				return t.rt.TransformResponse(ctx, httpResp)
			case llm.APIFormatOpenAIImageGeneration.String():
				return transformImageGenerationResponse(httpResp)
			case llm.APIFormatOpenAIEmbedding.String():
				return t.transformEmbeddingResponse(ctx, httpResp)
			}
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

// buildPlatformURL constructs the appropriate URL based on the platform.
func (t *OutboundTransformer) buildPlatformURL(_ *llm.Request) (string, error) {
	//nolint:exhaustive // Chcked.
	switch t.config.Type {
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
		// Standard OpenAI API
		if strings.HasSuffix(t.config.BaseURL, "/v1") {
			return t.config.BaseURL + "/chat/completions", nil
		}

		return t.config.BaseURL + "/v1/chat/completions", nil
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

// transformEmbeddingRequest transforms unified llm.Request to HTTP embedding request.
func (t *OutboundTransformer) transformEmbeddingRequest(
	_ context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("llm request is nil")
	}

	if llmReq.Embedding == nil {
		return nil, fmt.Errorf("embedding request is nil in llm.Request")
	}

	embReq := EmbeddingRequest{
		Input:          llmReq.Embedding.Input,
		Model:          llmReq.Model,
		EncodingFormat: llmReq.Embedding.EncodingFormat,
		Dimensions:     llmReq.Embedding.Dimensions,
		User:           llmReq.Embedding.User,
	}

	// Re-marshal to JSON (ensure clean output)
	body, err := json.Marshal(embReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Build URL, reuse same logic as chat
	url := t.buildEmbeddingURL()

	// Build auth config
	var auth *httpclient.AuthConfig

	//nolint:exhaustive // Checked.
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

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}

	// Set metadata for response routing
	if httpReq.TransformerMetadata == nil {
		httpReq.TransformerMetadata = make(map[string]any)
	}

	httpReq.TransformerMetadata["outbound_format_type"] = llm.APIFormatOpenAIEmbedding.String()

	return httpReq, nil
}

// buildEmbeddingURL constructs the embedding API URL.
func (t *OutboundTransformer) buildEmbeddingURL() string {
	//nolint:exhaustive // Checked.
	switch t.config.Type {
	case PlatformAzure:
		if strings.HasSuffix(t.config.BaseURL, "/openai/v1") {
			// Azure URL already includes /openai/v1
			return fmt.Sprintf("%s/embeddings?api-version=%s",
				t.config.BaseURL, t.config.APIVersion)
		}

		if strings.HasSuffix(t.config.BaseURL, "/openai") {
			// Azure URL includes /openai but not /v1
			return fmt.Sprintf("%s/v1/embeddings?api-version=%s",
				t.config.BaseURL, t.config.APIVersion)
		}
		// Default case for other Azure URLs
		return fmt.Sprintf("%s/openai/v1/embeddings?api-version=%s",
			t.config.BaseURL, t.config.APIVersion)
	default:
		// Standard OpenAI API
		if strings.HasSuffix(t.config.BaseURL, "/v1") {
			return t.config.BaseURL + "/embeddings"
		}

		return t.config.BaseURL + "/v1/embeddings"
	}
}

// transformEmbeddingResponse transforms HTTP embedding response to unified llm.Response.
func (t *OutboundTransformer) transformEmbeddingResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check HTTP status codes, 4xx/5xx should return standard format error
	// Note: httpclient usually already returns *httpclient.Error for 4xx/5xx,
	// this is defensive code to ensure error format conforms to OpenAI spec
	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
		})
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	// Parse OpenAI embedding response
	var embResp EmbeddingResponse
	if err := json.Unmarshal(httpResp.Body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedding response: %w", err)
	}

	// Convert OpenAI EmbeddingData to llm.EmbeddingData
	llmEmbeddingData := make([]llm.EmbeddingData, len(embResp.Data))
	for i, data := range embResp.Data {
		llmEmbeddingData[i] = llm.EmbeddingData{
			Object:    data.Object,
			Embedding: data.Embedding,
			Index:     data.Index,
		}
	}

	// Build unified embedding response
	var usage *llm.EmbeddingUsage
	if embResp.Usage.PromptTokens > 0 || embResp.Usage.TotalTokens > 0 {
		usage = &llm.EmbeddingUsage{
			PromptTokens: embResp.Usage.PromptTokens,
			TotalTokens:  embResp.Usage.TotalTokens,
		}
	}

	llmEmbeddingResp := &llm.EmbeddingResponse{
		Object: embResp.Object,
		Data:   llmEmbeddingData,
		Usage:  usage,
	}

	llmResp := &llm.Response{
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatOpenAIEmbedding,
		Embedding:   llmEmbeddingResp,
		Model:       embResp.Model,
	}

	return llmResp, nil
}
