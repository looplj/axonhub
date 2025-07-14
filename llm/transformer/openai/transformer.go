package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/types"
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
func (t *Transformer) TransformRequest(ctx context.Context, httpReq *http.Request) (*types.ChatCompletionRequest, error) {
	var chatReq types.ChatCompletionRequest
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
func (t *Transformer) TransformResponse(ctx context.Context, chatResp *types.ChatCompletionResponse) (*types.GenericHttpResponse, error) {
	body, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	// Create generic response
	return &types.GenericHttpResponse{
		StatusCode: http.StatusOK,
		Body:       body,
		Provider:   "openai",
	}, nil
}

// TransformStreamResponse converts ChatCompletionResponse to GenericHttpResponse
func (t *Transformer) TransformStreamResponse(ctx context.Context, chatResp *types.ChatCompletionResponse) (*types.GenericHttpResponse, error) {
	body, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	// Create generic response
	return &types.GenericHttpResponse{
		StatusCode: http.StatusOK,
		Body:       append(dataPrefix, body...),
		Provider:   "openai",
	}, nil
}
