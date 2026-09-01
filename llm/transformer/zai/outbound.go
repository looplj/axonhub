package zai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// Config holds all configuration for the Zai outbound transformer.
type Config struct {
	// API configuration
	BaseURL        string              `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKeyProvider auth.APIKeyProvider `json:"-"`                  // API key provider
	Version        string              `json:"version,omitempty"`  // API version (default: "v4")
	// EndpointPath replaces the default "/chat/completions" path. When set, the
	// base URL is kept as-is without version normalization (same convention as
	// the OpenAI transformer).
	EndpointPath string `json:"endpoint_path,omitempty"`
}

// OutboundTransformer implements transformer.Outbound for Zai format.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL        string
	APIKeyProvider auth.APIKeyProvider
	endpointPath   string
	rawURL         bool
}

// GLM/z.ai accept user_id values of 6-128 characters (error 1214); the field is
// omitted when the client-supplied value is shorter.
const (
	minUserIDLength = 6
	maxUserIDLength = 128
)

// NewOutboundTransformer creates a new Zai OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Zai OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
		ReasoningField: openai.ReasoningFieldContent,
	}

	t, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid Zai transformer configuration: %w", err)
	}

	version := config.Version
	if version == "" {
		version = "v4"
	}

	// "##" suffix marks a raw full request URL (same convention as the OpenAI
	// transformer); strip it here so it never leaks into the request URL.
	rawURL := strings.HasSuffix(config.BaseURL, "##")
	if rawURL {
		config.BaseURL = strings.TrimSuffix(config.BaseURL, "##")
	}

	var baseURL string
	switch {
	case rawURL:
		baseURL = strings.TrimRight(config.BaseURL, "/")
	case config.EndpointPath != "":
		baseURL = transformer.NormalizeBaseURL(config.BaseURL, "")
	default:
		baseURL = transformer.NormalizeBaseURL(config.BaseURL, version)
	}

	return &OutboundTransformer{
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
		endpointPath:   config.EndpointPath,
		rawURL:         rawURL,
		Outbound:       t,
	}, nil
}

type Request struct {
	openai.Request

	UserID    string    `json:"user_id,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Thinking  *Thinking `json:"thinking,omitempty"`
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
	if llmReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Validate required fields
	if llmReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeChat, "":
		// continue
	case llm.RequestTypeImage:
		return t.buildImageGenerationAPIRequest(ctx, llmReq)
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

	// Zai doesn't support json_schema, convert to json_object
	if oaiReq.ResponseFormat != nil && oaiReq.ResponseFormat.Type == "json_schema" {
		oaiReq.ResponseFormat.Type = "json_object"
		oaiReq.ResponseFormat.JSONSchema = nil
	}

	// Create Zai-specific request by adding request_id/user_id
	zaiReq := Request{
		Request:   *oaiReq,
		UserID:    "",
		RequestID: "",
	}

	if llmReq.Metadata != nil {
		zaiReq.UserID = llmReq.Metadata["user_id"]
		zaiReq.RequestID = llmReq.Metadata["request_id"]
	}

	// GLM/z.ai validate user_id as 6-128 characters (error 1214). Clients like
	// Claude Code send a long JSON blob (device_id/session_id) here; send only
	// values the upstream accepts — truncate long ones, drop short ones. An
	// empty value omits the field entirely (json omitempty).
	if runes := []rune(zaiReq.UserID); len(runes) > maxUserIDLength {
		zaiReq.UserID = string(runes[:maxUserIDLength])
	} else if len(runes) < minUserIDLength {
		zaiReq.UserID = ""
	}

	if zaiReq.RequestID == "" {
		sessionID, _ := shared.GetSessionID(ctx)
		zaiReq.RequestID = sessionID
	}

	// zai only support auto tool choice.
	if zaiReq.ToolChoice != nil {
		zaiReq.ToolChoice = &openai.ToolChoice{
			ToolChoice: lo.ToPtr("auto"),
		}
	}

	// zai request does not support metadata (extracted to user_id/request_id)
	zaiReq.Metadata = nil

	// Convert ReasoningEffort to Thinking if present
	if llmReq.ReasoningEffort != "" {
		var thinkingType string
		switch llmReq.ReasoningEffort {
		case "none":
			thinkingType = "disabled"
		default:
			thinkingType = "enabled"
		}
		zaiReq.Thinking = &Thinking{
			Type: thinkingType,
		}
	}

	body, err := json.Marshal(zaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Get API key from provider
	apiKey := t.APIKeyProvider.Get(ctx)

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: apiKey,
	}

	url := t.BaseURL
	switch {
	case t.rawURL:
		// Base URL is already the full request URL.
	case t.endpointPath != "":
		url += t.endpointPath
	default:
		url += "/chat/completions"
	}

	return &httpclient.Request{
		Method:    http.MethodPost,
		URL:       url,
		Headers:   headers,
		Body:      body,
		Auth:      auth,
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
	}, nil
}

// TransformResponse transforms the HTTP response to llm.Response.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	// Check for HTTP error status codes
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	// If this looks like Image Generation API, use image generation response transformer
	if httpResp.Request != nil && httpResp.Request.APIFormat == string(llm.APIFormatOpenAIImageGeneration) {
		return transformImageGenerationResponse(ctx, httpResp)
	}

	// For regular chat completions, delegate to the wrapped OpenAI transformer
	return t.Outbound.TransformResponse(ctx, httpResp)
}
