package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cast"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// PlatformType represents the platform type for OpenAI API.
type PlatformType string

const (
	PlatformOpenAI PlatformType = "openai"
	PlatformGoogle PlatformType = "google"
)

// Config holds all configuration for the OpenAI outbound transformer.
type Config struct {
	// Platform configuration
	PlatformType PlatformType `json:"type"`

	// BaseURL is the base URL for the OpenAI API, required.
	BaseURL string `json:"base_url,omitempty"`

	AccountIdentity string `json:"account_identity,omitempty"`

	// RawURL is whether to use raw URL for requests, default is false.
	// If true, the request URL will be used as is, without appending the chat completions endpoint.
	RawURL bool `json:"raw_url,omitempty"`

	// APIKeyProvider provides API keys for authentication, required.
	APIKeyProvider auth.APIKeyProvider `json:"-"`
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
		config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "v1")
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
	case PlatformOpenAI, PlatformGoogle:
		return nil
	default:
		return fmt.Errorf("unsupported platform type: %v", config.PlatformType)
	}
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
		return t.buildImageGenerationAPIRequest(ctx, llmReq)
	case llm.RequestTypeVideo:
		return t.buildVideoGenerationAPIRequest(ctx, llmReq)
	case llm.RequestTypeCompact:
		return nil, fmt.Errorf("%w: compact is only supported by OpenAI Responses API", transformer.ErrInvalidRequest)
	case llm.RequestTypeRerank:
		return nil, fmt.Errorf("%w: rerank is not supported", transformer.ErrInvalidRequest)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Convert to OpenAI Request format (this strips helper fields)
	oaiReq := RequestFromLLM(llmReq)
	//nolint:exhaustive // Checked.
	switch t.config.PlatformType {
	case PlatformOpenAI:
		stripUnsupportedToolCallExtraContent(oaiReq)
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Get API key from provider
	apiKey := t.config.APIKeyProvider.Get(ctx)
	scope := shared.TransportScope{
		BaseURL:         t.config.BaseURL,
		AccountIdentity: t.config.AccountIdentity,
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	authConfig := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: apiKey,
	}

	// Build platform-specific URL
	url, err := t.buildFullRequestURL(llmReq)
	if err != nil {
		return nil, fmt.Errorf("failed to build platform URL: %w", err)
	}

	return &httpclient.Request{
		Method:   http.MethodPost,
		URL:      url,
		Headers:  headers,
		Body:     body,
		Auth:     authConfig,
		Metadata: scope.Metadata(),
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
		case string(llm.APIFormatOpenAIVideo):
			return transformVideoResponse(httpResp)
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

	// Some providers emit structured error events in-stream (e.g. SSE `event: error`,
	// or JSON payloads like {"event":"error","data":{...}}). Treat them as stream errors
	// so the caller can surface them and persistence can mark the request as failed/canceled.
	if streamErr := parseStreamErrorEvent(event); streamErr != nil {
		return nil, streamErr
	}

	// Create a synthetic HTTP response for compatibility with existing logic
	httpResp := &httpclient.Response{
		Body: event.Data,
	}

	return t.TransformResponse(ctx, httpResp)
}

func parseStreamErrorEvent(event *httpclient.StreamEvent) *llm.ResponseError {
	if event == nil {
		return nil
	}

	// A provider may emit `event: error` with empty payload. Treat it as an error anyway.
	if event.Type == "error" && len(event.Data) == 0 {
		return &llm.ResponseError{
			Detail: llm.ErrorDetail{
				Message: "stream error",
				Type:    "stream_error",
			},
		}
	}

	if len(event.Data) == 0 {
		return nil
	}

	root := gjson.ParseBytes(event.Data)

	// Prefer explicit SSE event type when present.
	if event.Type == "error" || root.Get("event").String() == "error" {
		// Zai-style (SSE `event: error`): {"error":{"code":"...","message":"..."},"request_id":"..."}
		// Also tolerate wrapped form: {"event":"error","data":{"error":{...},"request_id":"..."}}
		errObj := root.Get("error")
		if !errObj.Exists() {
			errObj = root.Get("data.error")
		}

		detail := llm.ErrorDetail{
			Message: errObj.Get("message").String(),
			Type:    errObj.Get("type").String(),
			Code:    errObj.Get("code").String(),
			Param:   errObj.Get("param").String(),
		}

		if detail.Message == "" && errObj.Exists() {
			detail.Message = errObj.String()
		}
		if detail.Message == "" {
			detail.Message = "stream error"
		}

		if rid := root.Get("request_id").String(); rid != "" {
			detail.RequestID = rid
		} else if rid := root.Get("data.request_id").String(); rid != "" {
			detail.RequestID = rid
		} else if rid := errObj.Get("request_id").String(); rid != "" {
			detail.RequestID = rid
		}

		return &llm.ResponseError{Detail: detail}
	}

	// OpenAI-style: {"error":{...}} or {"error":"..."}
	ep := root.Get("error")
	if !ep.Exists() {
		return nil
	}

	detail := llm.ErrorDetail{
		Message: ep.Get("message").String(),
		Type:    ep.Get("type").String(),
		Code:    ep.Get("code").String(),
		Param:   ep.Get("param").String(),
	}
	if detail.Message == "" {
		detail.Message = ep.String()
	}

	// Best-effort request_id extraction (provider-specific).
	if rid := root.Get("request_id").String(); rid != "" {
		detail.RequestID = rid
	} else if rid := ep.Get("request_id").String(); rid != "" {
		detail.RequestID = rid
	}

	return &llm.ResponseError{Detail: detail}
}

// buildFullRequestURL constructs the appropriate URL based on the platform.
func (t *OutboundTransformer) buildFullRequestURL(_ *llm.Request) (string, error) {
	if t.config.RawURL {
		return t.config.BaseURL, nil
	}

	return t.config.BaseURL + "/chat/completions", nil
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
	// Use flexible types for code field to handle both string and number formats
	// (e.g., NVIDIA returns {"error":{"code":400}} while OpenAI returns {"error":{"code":"invalid_model"}})
	var openaiError struct {
		Error struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Param     string `json:"param,omitempty"`
			Code      any    `json:"code"` // Accept both string and number
			RequestID string `json:"request_id,omitempty"`
		} `json:"error"`
		Errors struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Param     string `json:"param,omitempty"`
			Code      any    `json:"code"` // Accept both string and number
			RequestID string `json:"request_id,omitempty"`
		} `json:"errors"`
	}

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil && (openaiError.Error.Message != "" || openaiError.Errors.Message != "") {
		errDetail := openaiError.Error
		if errDetail.Message == "" {
			errDetail = openaiError.Errors
		}

		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail: llm.ErrorDetail{
				Message:   errDetail.Message,
				Type:      errDetail.Type,
				Param:     errDetail.Param,
				Code:      cast.ToString(errDetail.Code),
				RequestID: errDetail.RequestID,
			},
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
