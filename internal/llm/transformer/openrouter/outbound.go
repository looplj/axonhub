package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// Config holds all configuration for the OpenRouter outbound transformer.
type Config struct {
	// API configuration
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKey  string `json:"api_key,omitempty"`  // API key
}

// OutboundTransformer implements transformer.Outbound for OpenRouter format.
// OpenRouter is mostly compatible with OpenAI(DeepSeek) API, but there are some differences for the reasoning content.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL string
	APIKey  string
}

// NewOutboundTransformer creates a new OpenRouter OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new OpenRouter OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	t, err := openai.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenRouter transformer configuration: %w", err)
	}

	baseURL := strings.TrimSuffix(config.BaseURL, "/")

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
	if llmReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeChat, "":
		// continue
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	// Validate required fields
	if llmReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	if llmReq.IsImageGenerationRequest() {
		llmReq = removeImageGenerationFromRequest(llmReq)
		llmReq.Stream = lo.ToPtr(false)
	}

	body, err := json.Marshal(openai.RequestFromLLM(llmReq))
	if err != nil {
		return nil, fmt.Errorf("%w: failed to transform request: %w", transformer.ErrInvalidRequest, err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	auth := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: t.APIKey,
	}

	url := t.BaseURL + "/chat/completions"

	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		Headers:     headers,
		Body:        body,
		Auth:        auth,
		ContentType: "application/json",
	}, nil
}

// The image generation tool is AxonHub specific to customize the image, so we need to remove it before sending the request.
func removeImageGenerationFromRequest(req *llm.Request) *llm.Request {
	var tools []llm.Tool

	for _, tool := range req.Tools {
		if tool.Type != "image_generation" {
			tools = append(tools, tool)
		}
	}

	req.Tools = tools

	return req
}

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

	return chatResp.ToOpenAIResponse().ToLLMResponse(), nil
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	}), nil
}

func (t *OutboundTransformer) TransformStreamChunk(ctx context.Context, event *httpclient.StreamEvent) (*llm.Response, error) {
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

type openRouterError struct {
	Error struct {
		Message  string `json:"message"`
		Code     int    `json:"code"`
		Metadata struct {
			Raw any `json:"raw"`
		} `json:"metadata"`
	} `json:"error"`
}

func (e openRouterError) ToLLMError() llm.ErrorDetail {
	message := cast.ToString(e.Error.Metadata.Raw)
	if message == "" {
		message = e.Error.Message
	}

	if message == "" && e.Error.Code != 0 {
		message = http.StatusText(e.Error.Code)
	}

	return llm.ErrorDetail{
		Message: message,
		Code:    fmt.Sprintf("%d", e.Error.Code),
		Type:    "api_error",
	}
}

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

	// Try to parse as OpenRouter error format first
	var openaiError openRouterError

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil {
		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail:     openaiError.ToLLMError(),
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
