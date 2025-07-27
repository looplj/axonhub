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

// InboundTransformer implements transformer.Inbound for OpenAI format
type InboundTransformer struct {
	name string
}

// NewInboundTransformer creates a new OpenAI InboundTransformer
func NewInboundTransformer() transformer.Inbound {
	return &InboundTransformer{
		name: "openai-inbound",
	}
}

// TransformRequest transforms HTTP request to ChatCompletionRequest
func (t *InboundTransformer) TransformRequest(ctx context.Context, httpReq *llm.GenericHttpRequest) (*llm.ChatCompletionRequest, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("http request is nil")
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}

	// Check content type
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = httpReq.Headers.Get("content-type")
	}

	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	var chatReq llm.ChatCompletionRequest
	if err := json.Unmarshal(httpReq.Body, &chatReq); err != nil {
		return nil, fmt.Errorf("failed to decode openai request: %w", err)
	}

	// Validate required fields
	if chatReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if len(chatReq.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	return &chatReq, nil
}

// TransformResponse transforms ChatCompletionResponse to GenericHttpResponse
func (t *InboundTransformer) TransformResponse(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericHttpResponse, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	body, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	// Create generic response
	return &llm.GenericHttpResponse{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

// TransformStreamChunk transforms ChatCompletionResponse to GenericStreamEvent
func (t *InboundTransformer) TransformStreamChunk(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericStreamEvent, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	if chatResp.Object == "[DONE]" {
		return &llm.GenericStreamEvent{
			Data: []byte("[DONE]"),
		}, nil
	}

	// For OpenAI, we keep the original response format as the event data
	eventData, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}
	return &llm.GenericStreamEvent{
		Type: "",
		Data: eventData,
	}, nil
}

// Name returns the transformer name
func (t *InboundTransformer) Name() string {
	return t.name
}

// SupportsContentType returns true if the transformer supports the given content type
func (t *InboundTransformer) SupportsContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/json")
}

// Priority returns the priority of the transformer
func (t *InboundTransformer) Priority() int {
	return 100 // Default priority
}
