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

// InboundTransformer implements transformer.InboundTransformer for Doubao
type InboundTransformer struct {
	name string
}

// NewInboundTransformer creates a new Doubao InboundTransformer
func NewInboundTransformer() transformer.InboundTransformer {
	return &InboundTransformer{
		name: "openai-inbound",
	}
}

// SupportsContentType returns true if the transformer supports the given content type
func (t *InboundTransformer) SupportsContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/json")
}

// Transform converts HTTP request to ChatCompletionRequest
func (t *InboundTransformer) Transform(ctx context.Context, httpReq *http.Request) (*types.ChatCompletionRequest, error) {
	var chatReq types.ChatCompletionRequest
	if err := json.NewDecoder(httpReq.Body).Decode(&chatReq); err != nil {
		return nil, fmt.Errorf("failed to decode openai request: %w", err)
	}
	return &chatReq, nil
}

// Name returns the name of the transformer
func (t *InboundTransformer) Name() string {
	return t.name
}

// Priority returns the priority of the transformer
func (t *InboundTransformer) Priority() int {
	return 100 // Default priority
}

// TransformResponse converts ChatCompletionResponse to GenericHttpResponse
func (t *InboundTransformer) TransformResponse(ctx context.Context, chatResp *types.ChatCompletionResponse, httpResp *http.Response) (*types.GenericHttpResponse, error) {
	// Marshal the ChatCompletionResponse to JSON
	body, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	// Convert headers from HTTP response
	headers := make(map[string]string)
	for key, values := range httpResp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Set content type for JSON response
	headers["Content-Type"] = "application/json"

	// Create generic response
	genericResp := &types.GenericHttpResponse{
		StatusCode: httpResp.StatusCode,
		Headers:    headers,
		Body:       body,
		Provider:   "openai",
	}

	// Handle error responses based on status code
	if httpResp.StatusCode >= 400 {
		genericResp.Error = &types.ResponseError{
			Code:    fmt.Sprintf("%d", httpResp.StatusCode),
			Message: "HTTP error",
			Type:    "http_error",
		}
	}

	return genericResp, nil
}
