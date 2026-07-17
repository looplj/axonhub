package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	// Import bedrock package to register its decoder.
	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	_ "github.com/looplj/axonhub/llm/bedrock"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/shared"
	"github.com/looplj/axonhub/llm/vertex"
)

func init() {
	httpclient.RegisterMergeWithAppendHeaders("Anthropic-Beta")
}

// PlatformType represents the platform type for Anthropic API.
type PlatformType string

const (
	PlatformDirect     PlatformType = "direct"     // Direct Anthropic API
	PlatformBedrock    PlatformType = "bedrock"    // AWS Bedrock
	PlatformVertex     PlatformType = "vertex"     // Google Vertex AI
	PlatformDeepSeek   PlatformType = "deepseek"   // DeepSeek with Anthropic format
	PlatformDoubao     PlatformType = "doubao"     // Doubao with Anthropic format
	PlatformMoonshot   PlatformType = "moonshot"   // Moonshot with Anthropic format
	PlatformZhipu      PlatformType = "zhipu"      // Zhipu with Anthropic format
	PlatformZai        PlatformType = "zai"        // Zai with Anthropic format
	PlatformLongCat    PlatformType = "longcat"    // LongCat with Anthropic format (Bearer auth)
	PlatformClaudeCode PlatformType = "claudecode" // Claude Code CLI
)

// Config holds all configuration for the Anthropic outbound transformer.
type Config struct {
	// Platform configuration
	Type PlatformType `json:"type"`

	Region string `json:"region,omitempty"` // For Vertex

	ProjectID string `json:"project_id,omitempty"` // For Vertex

	JSONData string `json:"json_data,omitempty"` // For Vertex

	// BaseURL is the base URL for the Anthropic API, required.
	BaseURL string `json:"base_url,omitempty"`

	// APIKeyProvider provides API keys for authentication, required.
	APIKeyProvider auth.APIKeyProvider `json:"-"`

	// EndpointPath is an optional custom path override for this endpoint.
	// When set, it replaces the default API path (e.g., "/messages").
	// Must start with "/". Skips default version normalization when set.
	EndpointPath string `json:"endpoint_path,omitempty"`

	// ThinkingCapabilityOverride declares the actual thinking wire capability of
	// an Anthropic-compatible upstream. It overrides the official Claude model
	// policy for this channel, except for DeepSeek which has its own adapter
	// policy and does not support Anthropic adaptive-thinking wire semantics.
	ThinkingCapabilityOverride ThinkingCapability `json:"thinking_capability_override,omitempty"`
}

// OutboundTransformer implements transformer.Outbound for Anthropic format.
type OutboundTransformer struct {
	config *Config
}

// NewOutboundTransformer creates a new Anthropic OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		Type:           PlatformDirect,
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
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

	// For Vertex/Bedrock, don't normalize with version - they have special URL formats
	//nolint:exhaustive // Checked.
	switch config.Type {
	case PlatformVertex, PlatformBedrock:
		config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "")
	default:
		if config.EndpointPath != "" {
			config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "")
		} else {
			config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "v1")
		}
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

	// Get API key from provider (Vertex/ClaudeCode use OAuth, not API keys)
	var apiKey string
	if t.config.APIKeyProvider != nil {
		apiKey = t.config.APIKeyProvider.Get(ctx)
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeChat, "":
		// continue
	case llm.RequestTypeCompact:
		return nil, fmt.Errorf("%w: compact is only supported by OpenAI Responses API", transformer.ErrInvalidRequest)
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

	thinkingPlan := resolveThinkingRequestPlan(llmReq, t.config)
	if thinkingPlan.validationErr != nil {
		return nil, fmt.Errorf("%w: %w", transformer.ErrInvalidRequest, thinkingPlan.validationErr)
	}

	chatResponseFormat := chatJSONSchemaResponseFormatForAnthropic(llmReq, t.config)
	recordAnthropicThinkingLossyDowngrade(llmReq, thinkingPlan)
	recordAnthropicChatNativeLossyDowngrades(llmReq, chatResponseFormat.isRepresented())
	recordAnthropicResponsesNativeLossyDowngrades(llmReq)
	recordAnthropicUnsupportedNativeToolLossyDowngrades(llmReq, t.config)

	// Convert to Anthropic request format
	anthropicReq, err := convertToAnthropicRequestWithThinkingPlan(llmReq, t.config, thinkingPlan)
	if err != nil {
		return nil, err
	}
	applyChatJSONSchemaResponseFormatForAnthropic(anthropicReq, llmReq, chatResponseFormat)

	// Anthropic supports two prompt-caching modes (see
	// https://docs.claude.com/en/docs/build-with-claude/prompt-caching):
	//   1. Automatic caching: a single top-level cache_control field. Anthropic
	//      itself manages the breakpoint placement, so we forward it untouched
	//      and intentionally skip our own breakpoint optimization pipeline.
	//   2. Explicit cache breakpoints: per-block cache_control fields. We run
	//      our optimization pipeline only in this mode.
	if anthropicReq.CacheControl == nil && countCacheControls(anthropicReq) > 0 {
		optimizeCacheControl(anthropicReq)
	}

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
	var authConfig *httpclient.AuthConfig

	if apiKey != "" {
		// LongCat uses Bearer token authentication instead of X-API-Key
		if t.config.Type == PlatformLongCat || t.config.Type == PlatformBedrock {
			authConfig = &httpclient.AuthConfig{
				Type:   httpclient.AuthTypeBearer,
				APIKey: apiKey,
			}
		} else {
			authConfig = &httpclient.AuthConfig{
				Type:      httpclient.AuthTypeAPIKey,
				APIKey:    apiKey,
				HeaderKey: "X-API-Key",
			}
		}
	}

	httpReq := &httpclient.Request{
		Method:    http.MethodPost,
		URL:       url,
		Headers:   headers,
		Body:      body,
		Auth:      authConfig,
		APIFormat: string(llm.APIFormatAnthropicMessage),
		Metadata:  nil,
	}
	shared.RecordResponsesLossyDowngradeDiagnosticsForTarget(llmReq, llm.APIFormatAnthropicMessage)
	shared.PropagateRequestMetadata(httpReq, llmReq)
	return httpReq, nil
}

// buildFullRequestURL constructs the appropriate URL based on the platform.
func (t *OutboundTransformer) buildFullRequestURL(chatReq *llm.Request) (string, error) {
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

		return t.config.BaseURL + endpoint, nil

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

		return t.config.BaseURL + endpoint, nil

	default:
		// BaseURL is already normalized with version in NewOutboundTransformerWithConfig
		if t.config.EndpointPath != "" {
			return t.config.BaseURL + t.config.EndpointPath, nil
		}

		return t.config.BaseURL + "/messages", nil
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
	shared.MergeResponseMetadata(chatResp, httpResp)

	return chatResp, nil
}

// AggregateStreamChunks aggregates Anthropic streaming response chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context, _ *httpclient.Request,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks, t.config.Type)
}

// SetAPIKey updates the API key.
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.config.APIKeyProvider = auth.NewStaticKeyProvider(apiKey)
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
			Message:   lo.Ternary(string(rawErr.Body) != "", strings.TrimSpace(string(rawErr.Body)), http.StatusText(rawErr.StatusCode)),
			Type:      "api_error",
			Code:      http.StatusText(rawErr.StatusCode),
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

func recordAnthropicChatNativeLossyDowngrades(llmReq *llm.Request, responseFormatBridged bool) {
	if llmReq == nil {
		return
	}

	// Typed OpenAI common fields with no Anthropic equivalent. Explicit allowlist
	// only — do not reflect over the full request model.
	switch llmReq.APIFormat {
	case llm.APIFormatOpenAIChatCompletion, llm.APIFormatOpenAIResponse:
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llmReq.APIFormat,
			"frequency_penalty",
			llm.APIFormatAnthropicMessage,
			llmReq.FrequencyPenalty != nil,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llmReq.APIFormat,
			"presence_penalty",
			llm.APIFormatAnthropicMessage,
			llmReq.PresencePenalty != nil,
		)
		// These OpenAI fields are deliberately not bridged to Anthropic's
		// metadata.user_id or cache_control: their semantics are different.
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llmReq.APIFormat,
			"safety_identifier",
			llm.APIFormatAnthropicMessage,
			llmReq.SafetyIdentifier != nil,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llmReq.APIFormat,
			"prompt_cache_key",
			llm.APIFormatAnthropicMessage,
			llmReq.PromptCacheKey != nil,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llmReq.APIFormat,
			"metadata",
			llm.APIFormatAnthropicMessage,
			hasOpenAIMetadataRemainder(llmReq.Metadata),
		)
	}
	// seed is Chat-native; Responses has no seed wire field to diagnose from.
	if llmReq.APIFormat == llm.APIFormatOpenAIChatCompletion {
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"seed",
			llm.APIFormatAnthropicMessage,
			llmReq.Seed != nil,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"tool_choice.type=allowed_tools",
			llm.APIFormatAnthropicMessage,
			llmReq.ToolChoice != nil && llmReq.ToolChoice.OpenAIChatAllowedTools != nil,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"logprobs",
			llm.APIFormatAnthropicMessage,
			llmReq.Logprobs != nil && *llmReq.Logprobs,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"top_logprobs",
			llm.APIFormatAnthropicMessage,
			llmReq.TopLogprobs != nil,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"logit_bias",
			llm.APIFormatAnthropicMessage,
			len(llmReq.LogitBias) > 0,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"store",
			llm.APIFormatAnthropicMessage,
			llmReq.Store != nil,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"stream_options.include_usage",
			llm.APIFormatAnthropicMessage,
			llmReq.StreamOptions != nil && llmReq.StreamOptions.IncludeUsage,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"response_format",
			llm.APIFormatAnthropicMessage,
			llmReq.ResponseFormat != nil && llmReq.ResponseFormat.Type != "text" && !responseFormatBridged,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"verbosity",
			llm.APIFormatAnthropicMessage,
			llmReq.Verbosity != nil,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"modalities",
			llm.APIFormatAnthropicMessage,
			hasNonTextModalities(llmReq.Modalities),
		)
	}

	recordAnthropicOpenAICustomToolLossyDowngrades(llmReq)

	shared.RecordOpenAIChatRawRequestLossyDowngrades(llmReq, llm.APIFormatAnthropicMessage, nil)
}

type chatJSONSchemaResponseFormatBridge struct {
	format         json.RawMessage
	residualFields []string
}

func (bridge chatJSONSchemaResponseFormatBridge) isRepresented() bool {
	return len(bridge.format) > 0
}

func chatJSONSchemaResponseFormatForAnthropic(llmReq *llm.Request, config *Config) chatJSONSchemaResponseFormatBridge {
	if llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIChatCompletion ||
		llmReq.ResponseFormat == nil || llmReq.ResponseFormat.Type != "json_schema" ||
		!supportsOutputConfig(config) {
		return chatJSONSchemaResponseFormatBridge{}
	}

	var source map[string]json.RawMessage
	if json.Unmarshal(llmReq.ResponseFormat.JSONSchema, &source) != nil || source == nil {
		return chatJSONSchemaResponseFormatBridge{}
	}
	schema, ok := source["schema"]
	if !ok {
		return chatJSONSchemaResponseFormatBridge{}
	}
	var schemaObject map[string]json.RawMessage
	if json.Unmarshal(schema, &schemaObject) != nil || schemaObject == nil {
		return chatJSONSchemaResponseFormatBridge{}
	}

	format, err := json.Marshal(struct {
		Type   string          `json:"type"`
		Schema json.RawMessage `json:"schema"`
	}{
		Type:   "json_schema",
		Schema: schema,
	})
	if err != nil {
		return chatJSONSchemaResponseFormatBridge{}
	}

	residualFields := make([]string, 0, len(source)-1)
	for field := range source {
		if field != "schema" {
			residualFields = append(residualFields, "response_format.json_schema."+field)
		}
	}
	sort.Strings(residualFields)
	return chatJSONSchemaResponseFormatBridge{
		format:         format,
		residualFields: residualFields,
	}
}

func applyChatJSONSchemaResponseFormatForAnthropic(
	request *MessageRequest,
	llmReq *llm.Request,
	bridge chatJSONSchemaResponseFormatBridge,
) {
	if request == nil || !bridge.isRepresented() {
		return
	}
	if request.OutputConfig == nil {
		request.OutputConfig = &OutputConfig{}
	}
	request.OutputConfig.Format = append(json.RawMessage(nil), bridge.format...)
	for _, field := range bridge.residualFields {
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			field,
			llm.APIFormatAnthropicMessage,
			true,
		)
	}
}

func recordAnthropicResponsesNativeLossyDowngrades(llmReq *llm.Request) {
	if llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIResponse {
		return
	}

	llm.AddLossyDowngradeIfPresent(
		llmReq,
		llm.APIFormatOpenAIResponse,
		"previous_response_id",
		llm.APIFormatAnthropicMessage,
		llmReq.PreviousResponseID != nil,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		llm.APIFormatOpenAIResponse,
		"input[].type=input_file",
		llm.APIFormatAnthropicMessage,
		hasResponsesInputFileParts(llmReq),
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		llm.APIFormatOpenAIResponse,
		"input[].type=input_audio",
		llm.APIFormatAnthropicMessage,
		hasResponsesInputAudioParts(llmReq),
	)

	if llmReq.ProviderExtensions == nil || llmReq.ProviderExtensions.OpenAIResponses == nil || llmReq.ProviderExtensions.OpenAIResponses.Request == nil {
		return
	}

	requestExt := llmReq.ProviderExtensions.OpenAIResponses.Request
	for _, field := range []struct {
		name    string
		present bool
	}{
		{name: "include", present: len(requestExt.Include) > 0},
		{name: "max_tool_calls", present: requestExt.MaxToolCalls != nil},
		{name: "prompt_cache_retention", present: requestExt.PromptCacheRetention != nil},
		{name: "truncation", present: requestExt.Truncation != nil},
		{name: "background", present: requestExt.Background != nil},
		{name: "prompt", present: len(requestExt.RawPrompt) > 0},
		{name: "stream_options", present: len(requestExt.RawStreamOptions) > 0},
		{name: "tool_choice", present: len(requestExt.RawToolChoice) > 0 && llmReq.ToolChoice == nil},
	} {
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIResponse,
			field.name,
			llm.APIFormatAnthropicMessage,
			field.present,
		)
	}
}

func hasResponsesInputFileParts(llmReq *llm.Request) bool {
	if llmReq == nil {
		return false
	}
	for _, message := range llmReq.Messages {
		for _, part := range message.Content.MultipleContent {
			if part.Type == "file" && part.OpenAIChatFile != nil {
				return true
			}
		}
	}
	return false
}

func hasResponsesInputAudioParts(llmReq *llm.Request) bool {
	if llmReq == nil {
		return false
	}
	for _, message := range llmReq.Messages {
		for _, part := range message.Content.MultipleContent {
			if part.Type == "input_audio" && part.InputAudio != nil {
				return true
			}
		}
	}
	return false
}

func hasNonTextModalities(modalities []string) bool {
	for _, modality := range modalities {
		if modality != "text" {
			return true
		}
	}
	return false
}

// recordAnthropicOpenAICustomToolLossyDowngrades records explicit loss for OpenAI
// freeform custom tool declarations/calls that have no Anthropic JSON input_schema
// equivalent. It does not invent Anthropic tools or tool_use blocks.
func recordAnthropicOpenAICustomToolLossyDowngrades(llmReq *llm.Request) {
	if llmReq == nil {
		return
	}

	hasCustomDecl := false
	for _, tool := range llmReq.Tools {
		if isOpenAICustomToolDecl(tool) {
			hasCustomDecl = true
			break
		}
	}

	hasCustomCall := false
	for _, msg := range llmReq.Messages {
		for _, tc := range msg.ToolCalls {
			if isOpenAICustomToolCall(tc) {
				hasCustomCall = true
				break
			}
		}
		if hasCustomCall {
			break
		}
	}

	if !hasCustomDecl && !hasCustomCall {
		return
	}

	sourceProtocol := llmReq.APIFormat
	if sourceProtocol == "" {
		sourceProtocol = llm.APIFormatOpenAIChatCompletion
	}

	switch sourceProtocol {
	case llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseCompact:
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			sourceProtocol,
			"tools[].type=custom",
			llm.APIFormatAnthropicMessage,
			hasCustomDecl,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			sourceProtocol,
			"input[].type=custom_tool_call",
			llm.APIFormatAnthropicMessage,
			hasCustomCall,
		)
	default:
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			sourceProtocol,
			"tools[].type=custom",
			llm.APIFormatAnthropicMessage,
			hasCustomDecl,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			sourceProtocol,
			"messages[].tool_calls[].type=custom",
			llm.APIFormatAnthropicMessage,
			hasCustomCall,
		)
	}
}

// recordAnthropicUnsupportedNativeToolLossyDowngrades records non-Anthropic native
// tool declarations that convertToolsAnthropic intentionally omits. This keeps the
// no-fake-bridge behavior while making the loss observable.
func recordAnthropicUnsupportedNativeToolLossyDowngrades(llmReq *llm.Request, config *Config) {
	if llmReq == nil || len(llmReq.Tools) == 0 {
		return
	}

	sourceProtocol := llmReq.APIFormat
	if sourceProtocol == "" {
		sourceProtocol = llm.APIFormatOpenAIChatCompletion
	}

	hasImageGeneration := false
	hasGoogleSearch := false
	hasGoogleCodeExecution := false
	hasGoogleURLContext := false
	hasUnsupportedWebSearch := false
	supportsNativeTools := supportsAnthropicNativeTools(config)

	for _, tool := range llmReq.Tools {
		switch tool.Type {
		case llm.ToolTypeImageGeneration:
			hasImageGeneration = true
		case llm.ToolTypeGoogleSearch:
			hasGoogleSearch = true
		case llm.ToolTypeGoogleCodeExecution:
			hasGoogleCodeExecution = true
		case llm.ToolTypeGoogleUrlContext:
			hasGoogleURLContext = true
		case llm.ToolTypeWebSearch:
			if !supportsNativeTools {
				hasUnsupportedWebSearch = true
			}
		}
	}

	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=image_generation",
		llm.APIFormatAnthropicMessage,
		hasImageGeneration,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=google_search",
		llm.APIFormatAnthropicMessage,
		hasGoogleSearch,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=google_code_execution",
		llm.APIFormatAnthropicMessage,
		hasGoogleCodeExecution,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=google_url_context",
		llm.APIFormatAnthropicMessage,
		hasGoogleURLContext,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=web_search",
		llm.APIFormatAnthropicMessage,
		hasUnsupportedWebSearch,
	)
}

func hasOpenAIMetadataRemainder(metadata map[string]string) bool {
	for key := range metadata {
		if key != "user_id" {
			return true
		}
	}
	return false
}
