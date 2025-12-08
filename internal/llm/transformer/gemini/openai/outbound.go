package geminioai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xjson"
)

// Config holds all configuration for the Gemini OpenAI outbound transformer.
type Config struct {
	// API configuration
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKey  string `json:"api_key,omitempty"`  // API key
}

// OutboundTransformer implements transformer.Outbound for Gemini OpenAI format.
// It wraps the OpenAI transformer and adds support for Gemini-specific features
// like thinking configuration via extra_body.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL string
	APIKey  string
}

// ThinkingBudget represents a thinking budget that can be either an int or a string.
// For Gemini 2.5 models: 1024 (low), 8192 (medium), 24576 (high)
// For Gemini 3 models: "low", "high".
type ThinkingBudget struct {
	IntValue    *int
	StringValue *string
}

// MarshalJSON implements json.Marshaler for ThinkingBudget.
func (tb ThinkingBudget) MarshalJSON() ([]byte, error) {
	if tb.StringValue != nil {
		return json.Marshal(*tb.StringValue)
	}

	if tb.IntValue != nil {
		return json.Marshal(*tb.IntValue)
	}

	return []byte("null"), nil
}

// UnmarshalJSON implements json.Unmarshaler for ThinkingBudget.
func (tb *ThinkingBudget) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as int first
	var intVal int
	if err := json.Unmarshal(data, &intVal); err == nil {
		tb.IntValue = &intVal
		return nil
	}

	// Try to unmarshal as string
	var strVal string
	if err := json.Unmarshal(data, &strVal); err == nil {
		tb.StringValue = &strVal
		return nil
	}

	return fmt.Errorf("thinking_budget must be an int or string")
}

// ThinkingConfig represents Gemini's thinking configuration.
type ThinkingConfig struct {
	// ThinkingBudget is the token budget for thinking.
	// For Gemini 2.5 models: 1024 (low), 8192 (medium), 24576 (high)
	// For Gemini 3 models: can also be "low", "high"
	ThinkingBudget *ThinkingBudget `json:"thinking_budget,omitempty"`
	// ThinkingLevel is the thinking level for Gemini 3 models.
	// Values: "low", "high"
	ThinkingLevel string `json:"thinking_level,omitempty"`
	// IncludeThoughts indicates whether to include thought summaries in the response.
	IncludeThoughts bool `json:"include_thoughts,omitempty"`
}

// GoogleExtraBody represents the Google-specific extra body structure.
type GoogleExtraBody struct {
	ThinkingConfig *ThinkingConfig `json:"thinking_config,omitempty"`
}

// ExtraBody represents the extra_body structure for Gemini OpenAI requests.
type ExtraBody struct {
	Google *GoogleExtraBody `json:"google,omitempty"`
}

// Request extends llm.Request with Gemini-specific fields.
type Request struct {
	llm.Request

	// ExtraBody contains Gemini-specific configuration like thinking_config.
	ExtraBody *ExtraBody `json:"extra_body,omitempty"`
}

// NewOutboundTransformer creates a new Gemini OpenAI OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Gemini OpenAI OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required for Gemini OpenAI transformer")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for Gemini OpenAI transformer")
	}

	baseURL := strings.TrimSuffix(config.BaseURL, "/")

	outbound, err := openai.NewOutboundTransformer(baseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini OpenAI outbound transformer: %w", err)
	}

	return &OutboundTransformer{
		Outbound: outbound,
		BaseURL:  baseURL,
		APIKey:   config.APIKey,
	}, nil
}

// NewThinkingBudgetInt creates a ThinkingBudget with an integer value.
func NewThinkingBudgetInt(val int) *ThinkingBudget {
	return &ThinkingBudget{IntValue: &val}
}

// NewThinkingBudgetString creates a ThinkingBudget with a string value.
func NewThinkingBudgetString(val string) *ThinkingBudget {
	return &ThinkingBudget{StringValue: &val}
}

// reasoningEffortToThinkingConfig converts OpenAI's reasoning_effort to Gemini's thinking_config.
// Mapping:
// - "none" -> disable thinking (only for 2.5 models, not 2.5 Pro or 3 models)
// - "minimal" or "low" -> thinking_level: "low" / thinking_budget: 1024
// - "medium" -> thinking_level: "high" / thinking_budget: 8192
// - "high" -> thinking_level: "high" / thinking_budget: 24576.
func reasoningEffortToThinkingConfig(effort string) *ThinkingConfig {
	switch strings.ToLower(effort) {
	case "none":
		// Disable thinking - use minimal budget
		// Note: Reasoning cannot be turned off for Gemini 2.5 Pro or 3 models
		return &ThinkingConfig{
			ThinkingBudget: NewThinkingBudgetInt(0),
		}
	case "minimal", "low":
		return &ThinkingConfig{
			ThinkingLevel:   "low",
			ThinkingBudget:  NewThinkingBudgetInt(1024),
			IncludeThoughts: true,
		}
	case "medium":
		return &ThinkingConfig{
			ThinkingLevel:   "high",
			ThinkingBudget:  NewThinkingBudgetInt(8192),
			IncludeThoughts: true,
		}
	case "high":
		return &ThinkingConfig{
			ThinkingLevel:   "high",
			ThinkingBudget:  NewThinkingBudgetInt(24576),
			IncludeThoughts: true,
		}
	default:
		// No mapping needed, let Gemini use default
		return nil
	}
}

// ParseExtraBody parses the extra_body from llm.Request and returns the ExtraBody struct.
func ParseExtraBody(rawExtraBody json.RawMessage) *ExtraBody {
	if xjson.IsNull(rawExtraBody) {
		return nil
	}

	var extraBody ExtraBody
	if err := json.Unmarshal(rawExtraBody, &extraBody); err != nil {
		return nil
	}

	return &extraBody
}

// TransformRequest transforms ChatCompletionRequest to Request with Gemini-specific handling.
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
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Create Gemini-specific request
	geminiReq := Request{
		Request: *chatReq,
	}

	// Priority 1: Parse ExtraBody from llm.Request (higher priority)
	if len(chatReq.ExtraBody) > 0 {
		extraBody := ParseExtraBody(chatReq.ExtraBody)
		if extraBody != nil && extraBody.Google != nil && extraBody.Google.ThinkingConfig != nil {
			geminiReq.ExtraBody = extraBody
			// Clear reasoning_effort as we're using thinking_config from extra_body
			geminiReq.ReasoningEffort = ""
		}
	}

	// Priority 2: Convert reasoning_effort to thinking_config if ExtraBody not set
	if geminiReq.ExtraBody == nil && chatReq.ReasoningEffort != "" {
		thinkingConfig := reasoningEffortToThinkingConfig(chatReq.ReasoningEffort)
		if thinkingConfig != nil {
			geminiReq.ExtraBody = &ExtraBody{
				Google: &GoogleExtraBody{
					ThinkingConfig: thinkingConfig,
				},
			}
			// Clear reasoning_effort as we're using thinking_config instead
			// Note: reasoning_effort and thinking_level/thinking_budget overlap functionality
			geminiReq.ReasoningEffort = ""
		}
	}

	// Clear help fields
	geminiReq.Metadata = nil
	geminiReq.Request.ExtraBody = nil // Clear the raw extra body from llm.Request

	body, err := json.Marshal(geminiReq)
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

	baseURL := strings.TrimSuffix(t.BaseURL, "/")

	var url string
	if strings.HasSuffix(baseURL, "/v1beta/openai") {
		url = baseURL + "/chat/completions"
	} else {
		url = baseURL + "/v1beta/openai/chat/completions"
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}
