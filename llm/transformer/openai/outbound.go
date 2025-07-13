package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/types"
)

// OutboundTransformer converts ChatCompletionRequest to DeepSeek API format
type OutboundTransformer struct {
	name               string
	supportedProviders map[string]*types.ProviderConfig
}

// NewOutboundTransformer creates a new DeepSeek outbound transformer
func NewOutboundTransformer() transformer.OutboundTransformer {
	return &OutboundTransformer{
		name:               "openai-outbound",
		supportedProviders: make(map[string]*types.ProviderConfig),
	}
}

// Transform converts ChatCompletionRequest to DeepSeek API format
func (t *OutboundTransformer) Transform(ctx context.Context, request *types.ChatCompletionRequest) (*types.GenericHttpRequest, error) {
	var config = types.ProviderConfig{
		Name:     "",
		Settings: map[string]interface{}{},
	}

	// Build HTTP request
	builder := types.NewRequestBuilder().
		WithMethod("POST").
		WithURL(config.BaseURL+"/v1/chat/completions").
		WithHeader("Content-Type", "application/json").
		WithBody(request).
		WithTimeout(30 * time.Second)

	// Add authentication
	if config.APIKey != "" {
		builder.WithHeader("Authorization", "Bearer "+config.APIKey)
	}

	// Enable streaming if requested
	if request.Stream {
		builder.WithStreaming(true)
	}
	return builder.Build(), nil
}

// TransformResponse converts DeepSeek response to ChatCompletionResponse
func (t *OutboundTransformer) TransformResponse(ctx context.Context, response *types.GenericHttpResponse, originalRequest *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
	if response.Error != nil {
		return nil, fmt.Errorf("deepseek API error: %s", response.Error.Message)
	}

	var chatResp types.ChatCompletionResponse
	err := json.Unmarshal(response.Body, &chatResp)
	if err != nil {
		return nil, err
	}
	return &chatResp, nil
}

// TransformStreamResponse converts DeepSeek streaming response to ChatCompletionStreamResponse
func (t *OutboundTransformer) TransformStreamResponse(ctx context.Context, response *types.GenericHttpResponse, originalRequest *types.ChatCompletionRequest) (<-chan *types.ChatCompletionStreamResponse, error) {
	if response.Stream == nil {
		return nil, fmt.Errorf("no stream available in response")
	}

	responseChan := make(chan *types.ChatCompletionStreamResponse, 100)

	go func() {
		defer close(responseChan)
		defer response.Stream.Close()

		scanner := bufio.NewScanner(response.Stream)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Parse SSE format: "data: {json}"
			if strings.HasPrefix(line, "data: ") {
				jsonData := strings.TrimPrefix(line, "data: ")
				if jsonData == "[DONE]" {
					break
				}

				var streamResp types.ChatCompletionStreamResponse
				if err := json.Unmarshal([]byte(jsonData), &streamResp); err != nil {
					continue
				}

				select {
				case responseChan <- &streamResp:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return responseChan, nil
}

// SupportsProvider returns true if this transformer supports the given provider
func (t *OutboundTransformer) SupportsProvider(provider string) bool {
	_, exists := t.supportedProviders[provider]
	return exists
}

// GetProviderConfig returns the configuration for the given provider
func (t *OutboundTransformer) GetProviderConfig(provider string) (*types.ProviderConfig, error) {
	config, exists := t.supportedProviders[provider]
	if !exists {
		return nil, fmt.Errorf("provider %s not supported", provider)
	}
	return config, nil
}

// Name returns the name of the transformer
func (t *OutboundTransformer) Name() string {
	return t.name
}

// SupportsContentType checks if the transformer supports the given content type
func (t *OutboundTransformer) SupportsContentType(contentType string) bool {
	return contentType == "application/json"
}

// AddProviderConfig adds a provider configuration
func (t *OutboundTransformer) AddProviderConfig(provider string, config *types.ProviderConfig) {
	t.supportedProviders[provider] = config
}
