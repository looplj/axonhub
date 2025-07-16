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

var (
	dataPrefix = []byte("data: ")
)

// Transformer implements transformer.Transformer for Doubao
type Transformer struct {
	name string
}

// NewTransformer creates a new Doubao InboundTransformer
func NewTransformer() transformer.Transformer {
	return &Transformer{
		name: "openai",
	}
}

// SupportsContentType returns true if the transformer supports the given content type
func (t *Transformer) SupportsContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/json")
}

// TransformRequest converts HTTP request to ChatCompletionRequest
func (t *Transformer) TransformRequest(ctx context.Context, httpReq *http.Request) (*llm.ChatCompletionRequest, error) {
	var chatReq llm.ChatCompletionRequest
	if err := json.NewDecoder(httpReq.Body).Decode(&chatReq); err != nil {
		return nil, fmt.Errorf("failed to decode openai request: %w", err)
	}
	return &chatReq, nil
}

// Name returns the name of the transformer
func (t *Transformer) Name() string {
	return t.name
}

// Priority returns the priority of the transformer
func (t *Transformer) Priority() int {
	return 100 // Default priority
}

// TransformResponse converts ChatCompletionResponse to GenericHttpResponse
func (t *Transformer) TransformResponse(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericHttpResponse, error) {
	body, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	// Create generic response
	return &llm.GenericHttpResponse{
		StatusCode: http.StatusOK,
		Body:       body,
	}, nil
}

// TransformStreamResponse converts ChatCompletionResponse to GenericHttpResponse
func (t *Transformer) TransformStreamResponse(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericHttpResponse, error) {
	body, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	// Create generic response
	return &llm.GenericHttpResponse{
		StatusCode: http.StatusOK,
		Body:       append(dataPrefix, body...),
	}, nil
}
