package zai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/tracing"
)

// Config holds all configuration for the Zai outbound transformer.
type Config struct {
	// API configuration
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKey  string `json:"api_key,omitempty"`  // API key
}

// OutboundTransformer implements transformer.Outbound for Zai format.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL string
	APIKey  string
}

// NewOutboundTransformer creates a new Zai OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Zai OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	t, err := openai.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("invalid Zai transformer configuration: %w", err)
	}

	baseURL := strings.TrimSuffix(config.BaseURL, "/")

	return &OutboundTransformer{
		BaseURL:  baseURL,
		APIKey:   config.APIKey,
		Outbound: t,
	}, nil
}

type Request struct {
	llm.Request

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

	// If this is an image generation request, use the Image Generation API.
	if chatReq.IsImageGenerationRequest() {
		return t.buildImageGenerationAPIRequest(chatReq)
	}

	// Create Zai-specific request by removing Metadata and adding request_id/user_id
	zaiReq := Request{
		Request:   *chatReq,
		UserID:    "",
		RequestID: "",
	}

	if chatReq.Metadata != nil {
		zaiReq.UserID = chatReq.Metadata["user_id"]
		zaiReq.RequestID = chatReq.Metadata["request_id"]
	}

	if zaiReq.RequestID == "" {
		traceID, _ := tracing.GetTraceID(ctx)
		zaiReq.RequestID = traceID
	}

	// zai only support auto tool choice.
	if zaiReq.ToolChoice != nil {
		zaiReq.ToolChoice = &llm.ToolChoice{
			ToolChoice: lo.ToPtr("auto"),
		}
	}

	// zai request does not support metadata.
	zaiReq.Metadata = nil

	// Convert ReasoningEffort to Thinking if present
	if chatReq.ReasoningEffort != "" {
		zaiReq.Thinking = &Thinking{
			Type: "enabled",
		}
	}

	body, err := json.Marshal(zaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: t.APIKey,
	}

	var url string
	if strings.HasSuffix(t.BaseURL, "/v1") {
		url = t.BaseURL + "/chat/completions"
	} else {
		url = t.BaseURL + "/v1/chat/completions"
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
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
	if httpResp.Request != nil && httpResp.Request.Metadata != nil && httpResp.Request.Metadata["outbound_format_type"] == string(llm.APIFormatOpenAIImageGeneration) {
		return transformImageGenerationResponse(ctx, httpResp)
	}

	// For regular chat completions, delegate to the wrapped OpenAI transformer
	return t.Outbound.TransformResponse(ctx, httpResp)
}
