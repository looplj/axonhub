package geminioai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/log"
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

// Request extends openai.Request with Gemini-specific fields.
type Request struct {
	openai.Request

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

// thinkingConfigToReasoningEffort converts Gemini's thinking_config to OpenAI's reasoning_effort.
// According to Gemini OpenAI documentation, reasoning_effort is automatically converted.
// Mapping (priority: ThinkingLevel > ThinkingBudget):
// ThinkingLevel:
//   - "minimal" -> "minimal"
//   - "low" -> "low"
//   - "medium" -> "medium"
//   - "high" -> "high"
//
// ThinkingBudget (Gemini 2.5):
//   - 1024 -> "low"
//   - 8192 -> "medium"
//   - 24576 -> "high"
func thinkingConfigToReasoningEffort(config *ThinkingConfig) string {
	if config == nil {
		return ""
	}

	// Priority 1: Use ThinkingLevel if present
	if config.ThinkingLevel != "" {
		return config.ThinkingLevel
	}

	// Priority 2: Convert ThinkingBudget to reasoning_effort
	if config.ThinkingBudget != nil {
		if config.ThinkingBudget.IntValue != nil {
			switch *config.ThinkingBudget.IntValue {
			case 1024:
				return "low"
			case 8192:
				return "medium"
			case 24576:
				return "high"
			case 0:
				return "none"
			}
		} else if config.ThinkingBudget.StringValue != nil {
			// String values like "low", "high" map directly
			return *config.ThinkingBudget.StringValue
		}
	}

	return ""
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

func isGemini25Model(model string) bool {
	return strings.Contains(strings.ToLower(model), "gemini-2.5")
}

func reasoningEffortToThinkingBudget(effort string) *ThinkingBudget {
	switch strings.ToLower(effort) {
	case "minimal", "low":
		return NewThinkingBudgetInt(1024)
	case "medium":
		return NewThinkingBudgetInt(8192)
	case "high":
		return NewThinkingBudgetInt(24576)
	case "none":
		return NewThinkingBudgetInt(0)
	default:
		return nil
	}
}

func fillThinkingConfigFromReasoningEffort(tc *ThinkingConfig, reasoningEffort string, model string) {
	if tc == nil {
		return
	}

	if tc.ThinkingLevel != "" {
		tc.ThinkingBudget = nil
		return
	}

	if tc.ThinkingBudget != nil {
		return
	}

	if reasoningEffort == "" {
		return
	}

	if isGemini25Model(model) {
		if budget := reasoningEffortToThinkingBudget(reasoningEffort); budget != nil {
			tc.ThinkingBudget = budget
			return
		}
	}

	tc.ThinkingLevel = strings.ToLower(reasoningEffort)
}

// TransformRequest transforms ChatCompletionRequest to Request with Gemini-specific handling.
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
		return nil, fmt.Errorf("model is required")
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Make a copy to avoid modifying the original request
	req := *llmReq

	// Fallback: Filter out Google native tools (not supported by OpenAI-compatible endpoint)
	// This is a graceful degradation when no native Gemini channels are available.
	if llm.ContainsGoogleNativeTools(req.Tools) {
		log.Warn(ctx, "Google native tools detected but gemini_openai channel does not support them, filtering out",
			log.Int("original_tools_count", len(req.Tools)))

		req.Tools = llm.FilterGoogleNativeTools(req.Tools)

		// 如果过滤后为空，置为 nil 以避免某些 OpenAI 兼容实现对空数组的校验问题
		if len(req.Tools) == 0 {
			req.Tools = nil
			// 同时重置 ToolChoice，因为没有工具可选
			req.ToolChoice = nil
		}

		log.Debug(ctx, "Filtered Google native tools",
			log.Int("remaining_tools_count", len(req.Tools)))
	}

	var extraBody *ExtraBody
	if len(req.ExtraBody) > 0 {
		extraBody = ParseExtraBody(req.ExtraBody)
		if extraBody != nil && extraBody.Google != nil && extraBody.Google.ThinkingConfig != nil {
			fillThinkingConfigFromReasoningEffort(extraBody.Google.ThinkingConfig, req.ReasoningEffort, req.Model)
			req.ReasoningEffort = ""
		}
	}

	// Convert llm.Request to openai.Request
	oaiReq := openai.RequestFromLLM(&req)

	geminiReq := Request{Request: *oaiReq}
	if extraBody != nil {
		geminiReq.ExtraBody = extraBody
	}

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

	type geminiOpenAIErrorDetail struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	}

	type geminiOpenAIErrorEnvelope struct {
		Error geminiOpenAIErrorDetail `json:"error"`
	}

	var arr []geminiOpenAIErrorEnvelope
	if err := json.Unmarshal(rawErr.Body, &arr); err == nil && len(arr) > 0 && arr[0].Error.Message != "" {
		detailType := arr[0].Error.Status
		if detailType == "" {
			detailType = "api_error"
		}

		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail: llm.ErrorDetail{
				Code:    strconv.Itoa(arr[0].Error.Code),
				Message: arr[0].Error.Message,
				Type:    detailType,
			},
		}
	}

	var obj geminiOpenAIErrorEnvelope
	if err := json.Unmarshal(rawErr.Body, &obj); err == nil && obj.Error.Message != "" {
		detailType := obj.Error.Status
		if detailType == "" {
			detailType = "api_error"
		}

		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail: llm.ErrorDetail{
				Code:    strconv.Itoa(obj.Error.Code),
				Message: obj.Error.Message,
				Type:    detailType,
			},
		}
	}

	if len(rawErr.Body) > 0 {
		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail: llm.ErrorDetail{
				Message: string(rawErr.Body),
				Type:    "api_error",
			},
		}
	}

	return t.Outbound.TransformError(ctx, rawErr)
}
