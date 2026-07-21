package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xmap"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

var (
	_ transformer.Outbound               = (*OutboundTransformer)(nil)
	_ pipeline.ChannelCustomizedExecutor = (*OutboundTransformer)(nil)
)

// Config holds all configuration for the OpenAI Responses outbound transformer.
const (
	TransportHTTP       = "http"
	TransportWebSocket  = "websocket"
	ResponsesLiteHeader = "X-OpenAI-Internal-Codex-Responses-Lite"
)

type Config struct {
	// BaseURL is the base URL for the OpenAI API, required.
	BaseURL string `json:"base_url,omitempty"`

	// RawURL is whether to use raw URL for requests, default is false.
	// If true, the request URL will be used as is, without appending the response endpoint.
	RawURL bool `json:"raw_url,omitempty"`

	// EndpointPath is an optional custom path override for this endpoint.
	// When set, it replaces the default API path (e.g., "/responses").
	// Must start with "/". Skips default version normalization when set.
	EndpointPath string `json:"endpoint_path,omitempty"`

	// APIKeyProvider provides API keys for authentication, required.
	APIKeyProvider auth.APIKeyProvider `json:"-"`

	// Transport selects the upstream transport for Responses API requests.
	// Empty and "http" use the existing HTTP/SSE transport; "websocket" uses Responses WebSocket mode.
	Transport string `json:"transport,omitempty"`
}

func NewOutboundTransformer(baseURL, apiKey string) (*OutboundTransformer, error) {
	if apiKey == "" || baseURL == "" {
		return nil, fmt.Errorf("apiKey or baseURL is empty")
	}

	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

func NewOutboundTransformerWithConfig(config *Config) (*OutboundTransformer, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("API key provider is required")
	}

	if strings.HasSuffix(config.BaseURL, "##") {
		config.RawURL = true
		config.BaseURL = strings.TrimSuffix(config.BaseURL, "##")
	} else {
		if config.EndpointPath != "" {
			config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "")
		} else {
			config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "v1")
		}
	}

	return &OutboundTransformer{
		config: config,
	}, nil
}

func (t *OutboundTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	if t == nil || t.config == nil || t.config.Transport != TransportWebSocket {
		return executor
	}

	if !ExecutorComparable(executor) {
		return NewWebSocketExecutor(executor)
	}

	t.executorMu.Lock()
	defer t.executorMu.Unlock()

	if t.webSocketExecutors == nil {
		t.webSocketExecutors = make(map[pipeline.Executor]*WebSocketExecutor)
	}
	if cached, ok := t.webSocketExecutors[executor]; ok {
		return cached
	}

	webSocketExecutor := NewWebSocketExecutor(executor)
	t.webSocketExecutors[executor] = webSocketExecutor

	return webSocketExecutor
}

func (t *OutboundTransformer) Stop() {
	if t == nil {
		return
	}

	t.executorMu.Lock()
	executors := make([]*WebSocketExecutor, 0, len(t.webSocketExecutors))
	for _, executor := range t.webSocketExecutors {
		executors = append(executors, executor)
	}
	t.webSocketExecutors = nil
	t.executorMu.Unlock()

	for _, executor := range executors {
		_ = executor.Close()
	}
}

func ExecutorComparable(executor pipeline.Executor) bool {
	if executor == nil {
		return true
	}

	return reflect.TypeOf(executor).Comparable()
}

type OutboundTransformer struct {
	config *Config

	executorMu         sync.Mutex
	webSocketExecutors map[pipeline.Executor]*WebSocketExecutor
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIResponse
}

// TransformError transforms HTTP error response to unified error response.
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

	// Try to parse as OpenAI error format first
	var openaiError struct {
		Error llm.ErrorDetail `json:"error"`
	}

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil && openaiError.Error.Message != "" {
		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail:     openaiError.Error,
		}
	}

	// If JSON parsing fails, use the upstream status text
	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: strings.TrimSpace(string(rawErr.Body)),
			Type:    "api_error",
		},
	}
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("chat request is nil")
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeCompact:
		return t.transformCompactRequest(ctx, llmReq)
	case llm.RequestTypeImage:
		imageReq, err := buildImageToolRequest(llmReq)
		if err != nil {
			return nil, err
		}

		llmReq = imageReq
	case llm.RequestTypeChat, "":
		// continue
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	// Initialize TransformerMetadata if nil
	if llmReq.TransformerMetadata == nil {
		llmReq.TransformerMetadata = map[string]any{}
	}

	apiKey := t.config.APIKeyProvider.Get(ctx)

	var tools []Tool
	hasUnsupportedGoogleCodeExecution := false
	hasUnsupportedGoogleURLContext := false
	hasUnsupportedCustomWithoutShape := false
	hasUnsupportedUnknownTool := false
	// Convert tools to Responses API format
	for _, item := range llmReq.Tools {
		switch item.Type {
		case llm.ToolTypeImageGeneration:
			tool := convertImageGenerationToTool(item)
			if action := xmap.GetStringPtr(llmReq.TransformerMetadata, responsesImageGenActionTransformerMetadataKey); action != nil {
				tool.Action = *action
			}
			tools = append(tools, tool)
			// Store image output format in TransformerMetadata
			llmReq.TransformerMetadata[responsesImageOutputFormatTransformerMetadataKey] = tool.OutputFormat
		case llm.ToolTypeWebSearch, llm.ToolTypeGoogleSearch:
			tool := convertWebSearchToTool(item)
			tools = append(tools, tool)
		case llm.ToolTypeResponsesCustomTool:
			tool := convertCustomToTool(item)
			tools = append(tools, tool)
		case "custom":
			// Explicit Chat→Responses custom tool declaration bridge.
			if item.OpenAIChatCustomTool == nil {
				hasUnsupportedCustomWithoutShape = true
				continue
			}
			tool := convertChatCustomToTool(item)
			tools = append(tools, tool)
		case "function":
			tool := convertFunctionToTool(item)
			tools = append(tools, tool)
		case llm.ToolTypeGoogleCodeExecution:
			hasUnsupportedGoogleCodeExecution = true
		case llm.ToolTypeGoogleUrlContext:
			hasUnsupportedGoogleURLContext = true
		default:
			// Omit tool types with no Responses declaration mapping.
			hasUnsupportedUnknownTool = true
			continue
		}
	}
	recordResponsesUnsupportedToolLossyDowngrades(
		llmReq,
		hasUnsupportedGoogleCodeExecution,
		hasUnsupportedGoogleURLContext,
		hasUnsupportedCustomWithoutShape,
		hasUnsupportedUnknownTool,
	)
	chatRawRepresentedFields := map[string]bool{}
	if webSearchTool, unknownFields := chatWebSearchOptionsForResponses(llmReq); webSearchTool != nil {
		tools = mergeChatWebSearchOptionsIntoResponsesTools(tools, *webSearchTool)
		chatRawRepresentedFields["web_search_options"] = true
		for _, field := range unknownFields {
			llm.AddLossyDowngradeIfPresent(
				llmReq,
				llm.APIFormatOpenAIChatCompletion,
				field,
				llm.APIFormatOpenAIResponse,
				true,
			)
		}
	}
	promptCacheRetention := openAIResponsesRequestPromptCacheRetention(llmReq)
	if promptCacheRetention == nil {
		if bridged, ok := chatPromptCacheRetentionForResponses(llmReq); ok {
			promptCacheRetention = bridged
			chatRawRepresentedFields["prompt_cache_retention"] = true
		}
	}
	payload := Request{
		Model:                llmReq.Model,
		Input:                convertInputFromMessages(llmReq.Messages, llmReq.TransformOptions, llmReq.TransformerMetadata),
		Instructions:         convertInstructionsFromMessages(llmReq.Messages),
		Tools:                tools,
		ParallelToolCalls:    llmReq.ParallelToolCalls,
		Stream:               llmReq.Stream,
		Text:                 convertToTextOptions(llmReq),
		Store:                llmReq.Store,
		ServiceTier:          llmReq.ServiceTier,
		SafetyIdentifier:     llmReq.SafetyIdentifier,
		User:                 llmReq.User,
		Metadata:             llmReq.Metadata,
		MaxOutputTokens:      llmReq.MaxCompletionTokens,
		Temperature:          llmReq.Temperature,
		FrequencyPenalty:     llmReq.FrequencyPenalty,
		PresencePenalty:      llmReq.PresencePenalty,
		TopLogprobs:          llmReq.TopLogprobs,
		TopP:                 llmReq.TopP,
		TopK:                 xmap.GetInt64Ptr(llmReq.TransformerMetadata, shared.TransformerMetadataKeyTopK),
		CacheControl:         restoreCacheControl(llmReq.TransformerMetadata),
		ToolChoice:           convertToolChoice(llmReq.ToolChoice),
		StreamOptions:        convertStreamOptions(openAIResponsesRequestRawStreamOptions(llmReq)),
		Reasoning:            convertReasoning(llmReq),
		PromptCacheKey:       llmReq.PromptCacheKey,
		PreviousResponseID:   llmReq.PreviousResponseID,
		Include:              includeForResponsesOutbound(llmReq),
		MaxToolCalls:         openAIResponsesRequestMaxToolCalls(llmReq),
		PromptCacheRetention: promptCacheRetention,
		Truncation:           openAIResponsesRequestTruncation(llmReq),
		Background:           openAIResponsesRequestBackground(llmReq),
	}

	if lo.FromPtr(payload.PromptCacheKey) == "" {
		if sessionID, ok := shared.GetSessionID(ctx); ok {
			payload.PromptCacheKey = lo.ToPtr(sessionID)
		}
	}

	// Responses Lite requires an explicit false value, even when no top-level tools are sent.
	if llmReq.RawRequest != nil && strings.EqualFold(strings.TrimSpace(llmReq.RawRequest.Headers.Get(ResponsesLiteHeader)), "true") {
		payload.ParallelToolCalls = lo.ToPtr(false)
	} else if len(payload.Tools) == 0 {
		// Other Responses providers may reject parallel_tool_calls when tools are absent.
		payload.ParallelToolCalls = nil
	}

	// Set MaxOutputTokens to MaxTokens if not set
	if payload.MaxOutputTokens == nil {
		payload.MaxOutputTokens = llmReq.MaxTokens
	}

	body, err := marshalRequestPayload(payload, llmReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal responses api request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	fullURL, err := t.buildFullRequestURL(llmReq)
	if err != nil {
		return nil, err
	}

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     fullURL,
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: apiKey,
		},
		APIFormat:             string(llm.APIFormatOpenAIResponse),
		SkipInboundQueryMerge: true,
		Metadata:              nil,
	}
	recordResponsesChatNativeLossyDowngrades(llmReq, chatRawRepresentedFields)
	shared.RecordAnthropicNativeLossyDowngradesForTarget(llmReq, llm.APIFormatOpenAIResponse)
	shared.PropagateRequestMetadata(httpReq, llmReq)
	return httpReq, nil
}

// recordResponsesUnsupportedToolLossyDowngrades records tool declarations that the
// Responses outbound intentionally omits instead of faking an unsupported wire shape.
func recordResponsesUnsupportedToolLossyDowngrades(
	llmReq *llm.Request,
	hasGoogleCodeExecution bool,
	hasGoogleURLContext bool,
	hasCustomWithoutShape bool,
	hasUnknownTool bool,
) {
	if llmReq == nil {
		return
	}
	sourceProtocol := llmReq.APIFormat
	if sourceProtocol == "" {
		sourceProtocol = llm.APIFormatOpenAIChatCompletion
	}
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=google_code_execution",
		llm.APIFormatOpenAIResponse,
		hasGoogleCodeExecution,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=google_url_context",
		llm.APIFormatOpenAIResponse,
		hasGoogleURLContext,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[].type=custom",
		llm.APIFormatOpenAIResponse,
		hasCustomWithoutShape,
	)
	llm.AddLossyDowngradeIfPresent(
		llmReq,
		sourceProtocol,
		"tools[] unsupported type",
		llm.APIFormatOpenAIResponse,
		hasUnknownTool,
	)
}

// recordResponsesChatNativeLossyDowngrades records explicit Chat→Responses field
// losses that the Responses payload cannot represent. Allowlisted only.
func recordResponsesChatNativeLossyDowngrades(llmReq *llm.Request, chatRawRepresentedFields map[string]bool) {
	if llmReq == nil {
		return
	}
	// seed is Chat-native and has no Responses request equivalent.
	if llmReq.APIFormat == llm.APIFormatOpenAIChatCompletion {
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"seed",
			llm.APIFormatOpenAIResponse,
			llmReq.Seed != nil,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"stop",
			llm.APIFormatOpenAIResponse,
			llmReq.Stop != nil && (llmReq.Stop.Stop != nil || len(llmReq.Stop.MultipleStop) > 0),
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"logit_bias",
			llm.APIFormatOpenAIResponse,
			len(llmReq.LogitBias) > 0,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"stream_options.include_usage",
			llm.APIFormatOpenAIResponse,
			llmReq.StreamOptions != nil && llmReq.StreamOptions.IncludeUsage,
		)
		llm.AddLossyDowngradeIfPresent(
			llmReq,
			llm.APIFormatOpenAIChatCompletion,
			"modalities",
			llm.APIFormatOpenAIResponse,
			hasNonTextModalities(llmReq.Modalities),
		)
	}
	shared.RecordOpenAIChatRawRequestLossyDowngrades(llmReq, llm.APIFormatOpenAIResponse, chatRawRepresentedFields)
}

func hasNonTextModalities(modalities []string) bool {
	for _, modality := range modalities {
		if modality != "text" {
			return true
		}
	}
	return false
}

func includeForResponsesOutbound(llmReq *llm.Request) []string {
	include := append([]string(nil), openAIResponsesRequestInclude(llmReq)...)
	if llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIChatCompletion || llmReq.Logprobs == nil || !*llmReq.Logprobs {
		return include
	}
	for _, value := range include {
		if value == "message.output_text.logprobs" {
			return include
		}
	}
	return append(include, "message.output_text.logprobs")
}

func chatPromptCacheRetentionForResponses(llmReq *llm.Request) (*string, bool) {
	if llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIChatCompletion || llmReq.RawRequest == nil {
		return nil, false
	}
	var source struct {
		PromptCacheRetention *string `json:"prompt_cache_retention"`
	}
	if json.Unmarshal(llmReq.RawRequest.Body, &source) != nil || source.PromptCacheRetention == nil {
		return nil, false
	}
	switch *source.PromptCacheRetention {
	case "in_memory", "24h":
		return source.PromptCacheRetention, true
	default:
		return nil, false
	}
}

// chatWebSearchOptionsForResponses adapts the Chat-only web-search wrapper to
// the equivalent Responses web_search tool. It keeps unknown source members
// separate so the target boundary can report only the residual loss.
func chatWebSearchOptionsForResponses(llmReq *llm.Request) (*Tool, []string) {
	if llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIChatCompletion || llmReq.RawRequest == nil {
		return nil, nil
	}

	var source map[string]json.RawMessage
	if json.Unmarshal(llmReq.RawRequest.Body, &source) != nil {
		return nil, nil
	}
	rawOptions, ok := source["web_search_options"]
	if !ok || string(rawOptions) == "null" {
		return nil, nil
	}

	var optionFields map[string]json.RawMessage
	if json.Unmarshal(rawOptions, &optionFields) != nil || optionFields == nil {
		return nil, nil
	}

	tool := &Tool{Type: "web_search"}
	unknownFields := make([]string, 0)
	for field, rawValue := range optionFields {
		switch field {
		case "search_context_size":
			var searchContextSize string
			if json.Unmarshal(rawValue, &searchContextSize) != nil || !isSupportedWebSearchContextSize(searchContextSize) {
				unknownFields = append(unknownFields, "web_search_options.search_context_size")
				continue
			}
			tool.SearchContextSize = searchContextSize
		case "user_location":
			location, locationUnknownFields, ok := chatWebSearchLocationForResponses(rawValue)
			if !ok {
				unknownFields = append(unknownFields, "web_search_options.user_location")
				continue
			}
			tool.UserLocation = location
			unknownFields = append(unknownFields, locationUnknownFields...)
		default:
			unknownFields = append(unknownFields, "web_search_options."+field)
		}
	}
	sort.Strings(unknownFields)
	return tool, unknownFields
}

func isSupportedWebSearchContextSize(value string) bool {
	switch value {
	case "low", "medium", "high":
		return true
	default:
		return false
	}
}

func chatWebSearchLocationForResponses(rawLocation json.RawMessage) (*WebSearchUserLocation, []string, bool) {
	if string(rawLocation) == "null" {
		return nil, nil, true
	}

	var locationFields map[string]json.RawMessage
	if json.Unmarshal(rawLocation, &locationFields) != nil || locationFields == nil {
		return nil, nil, false
	}

	rawType, ok := locationFields["type"]
	if !ok {
		return nil, []string{"web_search_options.user_location.type"}, true
	}
	var locationType string
	if json.Unmarshal(rawType, &locationType) != nil || locationType != "approximate" {
		return nil, []string{"web_search_options.user_location.type"}, true
	}

	location := &WebSearchUserLocation{Type: locationType}
	unknownFields := make([]string, 0)
	for field, rawValue := range locationFields {
		switch field {
		case "type":
			continue
		case "approximate":
			approximateUnknownFields, ok := populateChatWebSearchApproximateLocation(location, rawValue)
			if !ok {
				unknownFields = append(unknownFields, "web_search_options.user_location.approximate")
				continue
			}
			unknownFields = append(unknownFields, approximateUnknownFields...)
		default:
			unknownFields = append(unknownFields, "web_search_options.user_location."+field)
		}
	}
	return location, unknownFields, true
}

func populateChatWebSearchApproximateLocation(location *WebSearchUserLocation, rawApproximate json.RawMessage) ([]string, bool) {
	if string(rawApproximate) == "null" {
		return nil, true
	}

	var approximateFields map[string]json.RawMessage
	if json.Unmarshal(rawApproximate, &approximateFields) != nil || approximateFields == nil {
		return nil, false
	}

	unknownFields := make([]string, 0)
	for field, rawValue := range approximateFields {
		var target *string
		switch field {
		case "city":
			target = &location.City
		case "country":
			target = &location.Country
		case "region":
			target = &location.Region
		case "timezone":
			target = &location.Timezone
		default:
			unknownFields = append(unknownFields, "web_search_options.user_location.approximate."+field)
			continue
		}
		if json.Unmarshal(rawValue, target) != nil {
			unknownFields = append(unknownFields, "web_search_options.user_location.approximate."+field)
		}
	}
	return unknownFields, true
}

func mergeChatWebSearchOptionsIntoResponsesTools(tools []Tool, source Tool) []Tool {
	for index := range tools {
		if tools[index].Type != "web_search" {
			continue
		}
		if tools[index].SearchContextSize == "" {
			tools[index].SearchContextSize = source.SearchContextSize
		}
		if tools[index].UserLocation == nil && source.UserLocation != nil {
			location := *source.UserLocation
			tools[index].UserLocation = &location
		}
		return tools
	}
	return append(tools, source)
}

// buildFullRequestURL constructs the appropriate URL based on the platform.
func (t *OutboundTransformer) buildFullRequestURL(_ *llm.Request) (string, error) {
	if t.config.RawURL {
		return t.config.BaseURL, nil
	}

	if t.config.EndpointPath != "" {
		return t.config.BaseURL + t.config.EndpointPath, nil
	}

	return t.config.BaseURL + "/responses", nil
}

// TransformResponse converts an OpenAI Responses API HTTP response to unified llm.Response.
// It focuses on image generation results (image_generation_call) and maps them to
// assistant message content with image_url parts.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Route compact responses to specialized handler
	if httpResp.Request != nil && httpResp.Request.RequestType == string(llm.RequestTypeCompact) {
		return t.transformCompactResponse(ctx, httpResp)
	}

	return t.transformStandardResponse(ctx, httpResp)
}

func (t *OutboundTransformer) transformStandardResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, strings.TrimSpace(string(httpResp.Body)))
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var resp Response
	if err := json.Unmarshal(httpResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal responses api response: %w", err)
	}

	// Validate that we got a valid response
	if resp.ID == "" && resp.Model == "" && len(resp.Output) == 0 {
		return nil, fmt.Errorf("responses api returned empty response: body=%s", string(httpResp.Body))
	}

	llmResp := &llm.Response{
		Object:              "chat.completion",
		ID:                  resp.ID,
		Model:               resp.Model,
		Created:             resp.CreatedAt,
		PreviousResponseID:  resp.PreviousResponseID,
		Choices:             make([]llm.Choice, 0),
		TransformerMetadata: map[string]any{},
	}

	if resp.ServiceTier != nil {
		llmResp.ServiceTier = *resp.ServiceTier
	}
	if resp.Error != nil {
		llmResp.Error = &llm.ResponseError{
			Detail: llm.ErrorDetail{
				Type:    resp.Error.Type,
				Code:    resp.Error.Code,
				Message: resp.Error.Message,
			},
		}
	}

	// Convert usage if present
	if resp.Usage != nil {
		llmResp.Usage = resp.Usage.ToUsage()
	}

	captureOpenAIResponsesResponseTopLevelFields(httpResp.Body, llmResp)
	if resp.Status != nil && (*resp.Status == "queued" || *resp.Status == "in_progress") {
		llm.EnsureOpenAIResponsesResponseExtensions(llmResp).Status = lo.ToPtr(*resp.Status)
	}
	if rawOutputItems := responsesOutputItemsRequiringRawReplay(httpResp.Body); len(rawOutputItems) > 0 {
		ext := llm.EnsureOpenAIResponsesResponseExtensions(llmResp)
		ext.RawOutputItems = rawOutputItems
	}

	shared.MergeResponseMetadata(llmResp, httpResp)

	msg := convertOutputToMessage(resp.Output, llmResp.TransformerMetadata)

	choice := llm.Choice{
		Index:   0,
		Message: &msg,
	}

	if len(msg.ToolCalls) > 0 {
		choice.FinishReason = lo.ToPtr("tool_calls")
	} else if resp.Status != nil {
		switch *resp.Status {
		case "completed":
			choice.FinishReason = lo.ToPtr("stop")
		case "failed":
			choice.FinishReason = lo.ToPtr("error")
		case "incomplete":
			choice.FinishReason = lo.ToPtr("length")
		case "canceled", "cancelled":
			choice.FinishReason = lo.ToPtr("cancelled")
		}
	}

	llmResp.Choices = append(llmResp.Choices, choice)

	// If no choices were created, create a default empty choice
	if len(llmResp.Choices) == 0 {
		llmResp.Choices = []llm.Choice{
			{
				Index:        0,
				FinishReason: lo.ToPtr("stop"),
				Message: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr(""),
					},
				},
			},
		}
	}

	return llmResp, nil
}

var openAIResponsesRawResponseTopLevelFields = [...]string{"completed_at", "output_text", "incomplete_details"}

func captureOpenAIResponsesResponseTopLevelFields(body []byte, llmResp *llm.Response) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return
	}

	var captured map[string]json.RawMessage
	for _, field := range openAIResponsesRawResponseTopLevelFields {
		raw, ok := envelope[field]
		if !ok {
			continue
		}
		if captured == nil {
			captured = make(map[string]json.RawMessage)
		}
		captured[field] = append(json.RawMessage(nil), raw...)
	}
	if len(captured) == 0 {
		return
	}

	ext := llm.EnsureOpenAIResponsesResponseExtensions(llmResp)
	if ext.RawTopLevelFields == nil {
		ext.RawTopLevelFields = make(map[string]json.RawMessage)
	}
	for field, raw := range captured {
		ext.RawTopLevelFields[field] = raw
	}
}

// responsesOutputItemsRequiringRawReplay extracts output[] members that the
// canonical response model cannot represent without losing identity or ordering.
// Raw replay is same-Responses only and preserves original ordering through
// OriginalIndex.
func responsesOutputItemsRequiringRawReplay(body []byte) []llm.OpenAIResponsesRawFragment {
	var envelope struct {
		Output []json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}

	fragments := make([]llm.OpenAIResponsesRawFragment, 0)
	for index, raw := range envelope.Output {
		var probe struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		// Reasoning output items carry provider-bound identity/ciphertext that the
		// common response model cannot own. Preserve every native reasoning item in
		// the Responses response sidecar, including a single item.
		if probe.Type != "reasoning" && isStructurallyRepresentedResponsesOutputType(probe.Type) {
			continue
		}
		fragments = append(fragments, llm.OpenAIResponsesRawFragment{
			Type:          probe.Type,
			OriginalIndex: index,
			Raw:           append(json.RawMessage(nil), raw...),
		})
	}
	return fragments
}

func isStructurallyRepresentedResponsesOutputType(itemType string) bool {
	switch itemType {
	case "message", "output_text", "function_call", "custom_tool_call", "reasoning",
		"image_generation_call", "web_search_call", "compaction", "compaction_summary", "input_image":
		return true
	default:
		return false
	}
}
