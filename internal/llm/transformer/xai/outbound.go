package xai

import (
	"context"
	"errors"
	"fmt"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

const (
	// DefaultBaseURL is the default base URL for xAI API.
	DefaultBaseURL = "https://api.x.ai/v1"
)

// Config holds all configuration for the xAI outbound transformer.
type Config struct {
	// API configuration
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional, defaults to DefaultBaseURL)
	APIKey  string `json:"api_key"`            // API key (required)
}

// OutboundTransformer implements transformer.Outbound for xAI format.
type OutboundTransformer struct {
	transformer.Outbound

	config *Config
}

// NewOutboundTransformer creates a new xAI OutboundTransformer with legacy parameters
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new xAI OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	err := validateConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid xAI transformer configuration: %w", err)
	}

	outbound, err := openai.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
	}

	return &OutboundTransformer{
		Outbound: outbound,
		config:   config,
	}, nil
}

// validateConfig validates the configuration.
func validateConfig(config *Config) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}

	if config.APIKey == "" {
		return errors.New("API key is required")
	}

	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}

	return nil
}

// TransformRequest transforms the unified request to xAI HTTP request.
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

	switch chatReq.Model {
	case "grok-4":
		chatReq.ReasoningEffort = ""
		chatReq.PresencePenalty = nil
		chatReq.FrequencyPenalty = nil
		chatReq.Stop = nil
	case "grok-3", "grok-3-mini":
		chatReq.PresencePenalty = nil
		chatReq.FrequencyPenalty = nil
		chatReq.Stop = nil
	default:
		// Do nothing
	}

	return t.Outbound.TransformRequest(ctx, chatReq)
}

func IsValidResponse(event *llm.Response) bool {
	// Always allow the done response
	if event.Object == llm.DoneResponse.Object {
		return true
	}

	// Filter out events with no choices
	if len(event.Choices) == 0 {
		return false
	}

	choice := event.Choices[0]

	// Filter out events with no delta
	if choice.Delta == nil {
		return false
	}

	delta := choice.Delta

	// Check if delta has meaningful content
	hasContent := delta.Content.Content != nil && *delta.Content.Content != ""

	// Check for text content

	// Check for multiple content parts
	if len(delta.Content.MultipleContent) > 0 {
		hasContent = true
	}

	// Check for tool calls
	if len(delta.ToolCalls) > 0 {
		hasContent = true
	}

	// Check for role (important for the first message)
	if delta.Role != "" {
		hasContent = true
	}

	// Check for finish reason
	if choice.FinishReason != nil {
		hasContent = true
	}

	// Check for refusal
	if delta.Refusal != "" {
		hasContent = true
	}

	// Check for reasoning content (for models that support it)
	if delta.ReasoningContent != nil && *delta.ReasoningContent != "" {
		hasContent = true
	}

	return hasContent
}

func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	originStream, err := t.Outbound.TransformStream(ctx, stream)
	if err != nil {
		return nil, err
	}

	llmStream := streams.Filter(originStream, func(event *llm.Response) bool {
		return IsValidResponse(event)
	})

	return llmStream, nil
}
