package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// OutboundTransformer implements transformer.Outbound for OpenAI format.
type OutboundTransformer struct {
	baseURL string
	apiKey  string
}

// NewOutboundTransformer creates a new OpenAI OutboundTransformer.
func NewOutboundTransformer(baseURL, apiKey string) transformer.Outbound {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OutboundTransformer{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
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

	// Marshal the request body
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Prepare authentication
	var auth *httpclient.AuthConfig
	if t.apiKey != "" {
		auth = &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: t.apiKey,
		}
	}

	// Determine endpoint based on streaming
	endpoint := "/chat/completions"
	url := strings.TrimSuffix(t.baseURL, "/") + endpoint

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

	var chatResp llm.Response

	err := json.Unmarshal(httpResp.Body, &chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %w", err)
	}

	return &chatResp, nil
}

func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	}), nil
}

func (t *OutboundTransformer) TransformStreamChunk(
	ctx context.Context,
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
		return &llm.Response{
			Object: "[DONE]",
		}, nil
	}

	// Create a synthetic HTTP response for compatibility with existing logic
	httpResp := &httpclient.Response{
		Body: event.Data,
	}

	return t.TransformResponse(ctx, httpResp)
}

// SetAPIKey updates the API key.
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.apiKey = apiKey
}

// SetBaseURL updates the base URL.
func (t *OutboundTransformer) SetBaseURL(baseURL string) {
	t.baseURL = baseURL
}

func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, error) {
	return AggregateStreamChunks(ctx, chunks)
}
