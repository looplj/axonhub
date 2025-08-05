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
	if err := json.Unmarshal(httpResp.Body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %w", err)
	}

	return &chatResp, nil
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

// SupportsModel checks if the transformer supports a specific model.
func (t *OutboundTransformer) SupportsModel(model string) bool {
	// OpenAI transformer supports OpenAI models
	openaiModels := []string{
		"gpt-4", "gpt-4-turbo", "gpt-4o", "gpt-4o-mini",
		"gpt-3.5-turbo", "gpt-3.5-turbo-16k",
		"text-davinci-003", "text-davinci-002",
	}

	for _, supportedModel := range openaiModels {
		if strings.HasPrefix(model, supportedModel) {
			return true
		}
	}

	return false
}

// SetAPIKey updates the API key.
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.apiKey = apiKey
}

// SetBaseURL updates the base URL.
func (t *OutboundTransformer) SetBaseURL(baseURL string) {
	t.baseURL = baseURL
}

// AggregateStreamChunks aggregates OpenAI streaming response chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, error) {
	if len(chunks) == 0 {
		emptyResp := &llm.Response{}
		return json.Marshal(emptyResp)
	}

	// For OpenAI-style streaming, we need to aggregate the delta content from chunks
	// into a complete ChatCompletionResponse
	var (
		aggregatedContent strings.Builder
		lastChunk         map[string]any
	)

	for _, chunk := range chunks {
		// Skip [DONE] events
		if bytes.HasPrefix(chunk.Data, []byte("[DONE]")) {
			continue
		}

		var chunkData map[string]any
		if err := json.Unmarshal(chunk.Data, &chunkData); err != nil {
			continue // Skip invalid chunks
		}

		// Extract content from choices[0].delta.content if it exists
		if choices, ok := chunkData["choices"].([]any); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]any); ok {
				if delta, ok := choice["delta"].(map[string]any); ok {
					if content, ok := delta["content"].(string); ok {
						aggregatedContent.WriteString(content)
					}
				}
			}
		}

		// Keep the last chunk for metadata
		lastChunk = chunkData
	}

	// Create a complete ChatCompletionResponse based on the last chunk structure
	if lastChunk == nil {
		emptyResp := &llm.Response{}
		return json.Marshal(emptyResp)
	}

	// Build the final response
	finalResponse := map[string]interface{}{
		"object": "chat.completion", // Change from "chat.completion.chunk" to "chat.completion"
	}

	// Copy metadata from the last chunk
	for key, value := range lastChunk {
		if key != "choices" && key != "object" {
			finalResponse[key] = value
		}
	}

	// Create the final choices with aggregated content
	finalResponse["choices"] = []map[string]interface{}{
		{
			"index": 0,
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": aggregatedContent.String(),
			},
			"finish_reason": "stop",
		},
	}

	// Marshal the final response directly
	finalJSON, err := json.Marshal(finalResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal final response: %w", err)
	}

	return finalJSON, nil
}
