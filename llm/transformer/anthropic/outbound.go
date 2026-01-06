package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	// Import bedrock package to register its decoder.
	"github.com/looplj/axonhub/internal/pkg/vertex"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm"
	_ "github.com/looplj/axonhub/llm/bedrock"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// PlatformType represents the platform type for Anthropic API.
type PlatformType string

const (
	PlatformDirect   PlatformType = "direct"   // Direct Anthropic API
	PlatformBedrock  PlatformType = "bedrock"  // AWS Bedrock
	PlatformVertex   PlatformType = "vertex"   // Google Vertex AI
	PlatformDeepSeek PlatformType = "deepseek" // DeepSeek with Anthropic format
	PlatformDoubao   PlatformType = "doubao"   // Doubao with Anthropic format
	PlatformMoonshot PlatformType = "moonshot" // Moonshot with Anthropic format
	PlatformZhipu    PlatformType = "zhipu"    // Zhipu with Anthropic format
	PlatformZai      PlatformType = "zai"      // Zai with Anthropic format
	PlatformLongCat  PlatformType = "longcat"  // LongCat with Anthropic format (Bearer auth)
)

// Config holds all configuration for the Anthropic outbound transformer.
type Config struct {
	// Platform configuration
	Type PlatformType `json:"type"`

	Region string `json:"region,omitempty"` // For Vertex

	ProjectID string `json:"project_id,omitempty"` // For Vertex
	JSONData  string `json:"json_data,omitempty"`  // For Vertex

	// BaseURL is the base URL for the Anthropic API, required.
	BaseURL string `json:"base_url,omitempty"`

	// RawURL is whether to use raw URL for requests, default is false.
	// If true, the base URL will be used as is, without appending the version.
	RawURL bool `json:"raw_url,omitempty"`

	// APIKey is the API key for authentication, required.
	APIKey string `json:"api_key,omitempty"`

	// Thinking configuration
	// Maps ReasoningEffort values to Anthropic thinking budget tokens
	ReasoningEffortToBudget map[string]int64 `json:"reasoning_effort_to_budget,omitempty"`
}

// OutboundTransformer implements transformer.Outbound for Anthropic format.
type OutboundTransformer struct {
	config *Config
}

// NewOutboundTransformer creates a new Anthropic OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		Type:    PlatformDirect,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Anthropic OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	var t transformer.Outbound = &OutboundTransformer{
		config: config,
	}

	if config.Type == PlatformVertex {
		executor, err := vertex.NewExecutorFromJSON(config.Region, config.ProjectID, config.JSONData)
		if err != nil {
			return nil, fmt.Errorf("failed to create vertex transformer: %w", err)
		}

		t = &VertexTransformer{
			Outbound: t,
			executor: executor,
		}
	}

	if before, ok := strings.CutSuffix(config.BaseURL, "#"); ok {
		config.BaseURL = before
		config.RawURL = true
	}

	return t, nil
}

// APIFormat returns the API format of the transformer.
func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatAnthropicMessage
}

// TransformRequest transforms ChatCompletionRequest to Anthropic HTTP request.
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

	// Validate max_tokens
	if llmReq.MaxTokens != nil && *llmReq.MaxTokens <= 0 {
		return nil, fmt.Errorf("%w: max_tokens must be positive", transformer.ErrInvalidRequest)
	}

	// Convert to Anthropic request format
	anthropicReq := convertToAnthropicRequestWithConfig(llmReq, t.config)

	// Determine endpoint based on platform
	url, err := t.buildFullRequestURL(llmReq)
	if err != nil {
		return nil, fmt.Errorf("failed to build platform URL: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	//nolint:exhaustive // Checked.
	switch t.config.Type {
	case PlatformBedrock:
		headers.Set("Anthropic-Version", "bedrock-2023-05-31")

		anthropicReq.AnthropicVersion = "bedrock-2023-05-31"
		// Clear the model as it's not used with Bedrock
		anthropicReq.Model = ""
		// Clear stream as it's not used with Bedrock
		anthropicReq.Stream = nil
	case PlatformVertex:
		headers.Set("Anthropic-Version", "vertex-2023-10-16")
	default:
		headers.Set("Anthropic-Version", "2023-06-01")
	}

	// Apply platform-specific transformations
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic request: %w", err)
	}

	// Add beta header for web search feature only when:
	// 1. Native web search tool is present, AND
	// 2. Platform is direct Anthropic API or Bedrock (not Vertex which may not support this beta)
	if containsNativeWebSearchTool(anthropicReq.Tools) {
		//nolint:exhaustive // Checked.
		switch t.config.Type {
		case PlatformDirect:
			headers.Add("Anthropic-Beta", "web-search-2025-03-05")
		case PlatformBedrock:
			anthropicReq.AnthropicBeta = append(anthropicReq.AnthropicBeta, "web-search-2025-03-05")
		}
	}

	// Prepare authentication
	var auth *httpclient.AuthConfig

	if t.config.APIKey != "" && t.config.Type != PlatformVertex {
		// LongCat uses Bearer token authentication instead of X-API-Key
		if t.config.Type == PlatformLongCat || t.config.Type == PlatformBedrock {
			auth = &httpclient.AuthConfig{
				Type:   httpclient.AuthTypeBearer,
				APIKey: t.config.APIKey,
			}
		} else {
			auth = &httpclient.AuthConfig{
				Type:      httpclient.AuthTypeAPIKey,
				APIKey:    t.config.APIKey,
				HeaderKey: "X-API-Key",
			}
		}
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

// buildFullRequestURL constructs the appropriate URL based on the platform.
func (t *OutboundTransformer) buildFullRequestURL(chatReq *llm.Request) (string, error) {
	baseURL := strings.TrimSuffix(t.config.BaseURL, "/")

	//nolint:exhaustive // Checked.
	switch t.config.Type {
	case PlatformBedrock:
		// Bedrock URL format: /model/{model}/invoke or /model/{model}/invoke-with-response-stream
		var endpoint string
		if chatReq.Stream != nil && *chatReq.Stream {
			endpoint = fmt.Sprintf("/model/%s/invoke-with-response-stream", chatReq.Model)
		} else {
			endpoint = fmt.Sprintf("/model/%s/invoke", chatReq.Model)
		}

		return baseURL + endpoint, nil

	case PlatformVertex:
		// Vertex AI URL format: /v1/projects/{project}/locations/{region}/publishers/anthropic/models/{model}:rawPredict
		if t.config.ProjectID == "" {
			return "", fmt.Errorf("project ID is required for Vertex AI")
		}

		if t.config.Region == "" {
			return "", fmt.Errorf("region is required for Vertex AI")
		}

		var specifier string
		if chatReq.Stream != nil && *chatReq.Stream {
			specifier = "streamRawPredict"
		} else {
			specifier = "rawPredict"
		}

		endpoint := fmt.Sprintf("/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:%s",
			t.config.ProjectID, t.config.Region, chatReq.Model, specifier)

		return baseURL + endpoint, nil

	default:
		// RawURL is true, use the base URL as is
		if t.config.RawURL {
			return baseURL + "/messages", nil
		}

		// Direct Anthropic API
		if strings.HasSuffix(baseURL, "/v1") {
			return baseURL + "/messages", nil
		}

		return baseURL + "/v1/messages", nil
	}
}

// TransformResponse transforms Anthropic HTTP response to ChatCompletionResponse.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check for HTTP error status
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var anthropicResp Message

	err := json.Unmarshal(httpResp.Body, &anthropicResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal anthropic response: %w", err)
	}

	// Convert to ChatCompletionResponse
	chatResp := convertToLlmResponse(&anthropicResp, t.config.Type)

	return chatResp, nil
}

// AggregateStreamChunks aggregates Anthropic streaming response chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks, t.config.Type)
}

// SetAPIKey updates the API key.
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.config.APIKey = apiKey
}

// SetBaseURL updates the base URL.
func (t *OutboundTransformer) SetBaseURL(baseURL string) {
	t.config.BaseURL = baseURL
}

// SetConfig updates the entire configuration.
func (t *OutboundTransformer) SetConfig(config *Config) {
	if config == nil {
		config = &Config{Type: PlatformDirect}
	}

	t.config = config
}

// GetConfig returns the current configuration.
func (t *OutboundTransformer) GetConfig() *Config {
	return t.config
}

// GetPlatformConfig returns the current platform configuration (for backward compatibility).
// Deprecated: Use GetConfig instead.
func (t *OutboundTransformer) GetPlatformConfig() *Config {
	return t.config
}

// TransformError transforms HTTP error response to unified error response for Anthropic.
func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	if rawErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: "Request failed.",
				Type:    "api_error",
			},
		}
	}

	aErr, err := xjson.To[AnthropicError](rawErr.Body)
	if err == nil && aErr.Error.Message != "" {
		// Successfully parsed as Anthropic error format
		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail: llm.ErrorDetail{
				Type:      "api_error",
				Message:   aErr.Error.Message,
				RequestID: aErr.RequestID,
			},
		}
	}

	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message:   string(rawErr.Body),
			Type:      "api_error",
			Code:      "",
			Param:     "",
			RequestID: "",
		},
	}
}

// containsNativeWebSearchTool checks if the Anthropic tools slice contains the native web search tool.
func containsNativeWebSearchTool(tools []Tool) bool {
	for _, tool := range tools {
		if tool.Type == ToolTypeWebSearch20250305 {
			return true
		}
	}

	return false
}
