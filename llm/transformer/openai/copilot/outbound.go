package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/tidwall/gjson"
)

var modelVersionRegex = regexp.MustCompile(`^gpt-(\d+)`)

const (
	DefaultCopilotBaseURL          = "https://api.githubcopilot.com"
	CopilotChatCompletionsEndpoint = "/chat/completions"
	EditorVersionHeader            = "editor-version"
	EditorPluginVersionHeader      = "editor-plugin-version"
	UserAgentHeader                = "user-agent"
	OpenAIIntentHeader             = "openai-intent"
	CopilotIntegrationIDHeader     = "copilot-integration-id"
	GitHubAPIVersionHeader         = "x-github-api-version"
	RequestIDHeader                = "x-request-id"
	VSCodeUserAgentLibHeader       = "x-vscode-user-agent-library-version"
	CopilotVisionRequestHeader     = "Copilot-Vision-Request"
	InitiatorHeader                = "X-Initiator"
	// LiteLLM-style defaults for Copilot quota tracking
	DefaultEditorVersion       = "vscode/1.95.0"
	DefaultEditorPluginVersion = "copilot-chat/0.26.7"
	DefaultUserAgent           = "GitHubCopilotChat/0.26.7"
	DefaultOpenAIIntent        = "conversation-edits"
	DefaultCopilotIntegrationID = "vscode-chat"
	DefaultGitHubAPIVersion     = "2025-04-01"
	DefaultVSCodeUserAgentLib   = "electron-fetch"
)

type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
}

type OutboundTransformer struct {
	tokenProvider     TokenProvider
	baseURL           string
	responses         *responses.OutboundTransformer
	openAITransformer transformer.Outbound
}

type OutboundTransformerParams struct {
	TokenProvider TokenProvider // required
	BaseURL       string         // optional, defaults to DefaultCopilotBaseURL
}

var _ transformer.Outbound = (*OutboundTransformer)(nil)

func NewOutboundTransformer(params OutboundTransformerParams) (*OutboundTransformer, error) {
	if params.TokenProvider == nil {
		return nil, errors.New("token provider is required")
	}

	baseURL := params.BaseURL
	if baseURL == "" {
		baseURL = DefaultCopilotBaseURL
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	// For GPT-5+ models that use Responses API
	responsesTransformer, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(""),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create responses transformer: %w", err)
	}

	return &OutboundTransformer{
		tokenProvider: params.TokenProvider,
		baseURL:       baseURL,
		responses:     responsesTransformer,
	}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIChatCompletion
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, errors.New("request is nil")
	}

	if llmReq.Model == "" {
		return nil, errors.New("model is required")
	}

	if len(llmReq.Messages) == 0 {
		return nil, errors.New("messages are required")
	}

	if usesResponsesAPI(llmReq.Model) {
		return t.transformResponsesRequest(ctx, llmReq)
	}

	token, err := t.tokenProvider.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get copilot token: %w", err)
	}

	oaiReq := openai.RequestFromLLM(llmReq)

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := t.baseURL + CopilotChatCompletionsEndpoint

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// LiteLLM-style editor headers required by Copilot
	setCopilotHeaders(headers)

	if hasVisionContent(llmReq) {
		headers.Set(CopilotVisionRequestHeader, "true")
	}

	headers.Set(InitiatorHeader, resolveCopilotInitiator(llmReq))

	authConfig := &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: token,
	}

	return &httpclient.Request{
		Method:    http.MethodPost,
		URL:       url,
		Headers:   headers,
		Body:      body,
		Auth:      authConfig,
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
	}, nil
}

func setCopilotHeaders(headers http.Header) {
	headers.Set(EditorVersionHeader, DefaultEditorVersion)
	headers.Set(EditorPluginVersionHeader, DefaultEditorPluginVersion)
	headers.Set(UserAgentHeader, DefaultUserAgent)
	headers.Set(CopilotIntegrationIDHeader, DefaultCopilotIntegrationID)
	headers.Set(OpenAIIntentHeader, DefaultOpenAIIntent)
	headers.Set(GitHubAPIVersionHeader, DefaultGitHubAPIVersion)
	headers.Set(VSCodeUserAgentLibHeader, DefaultVSCodeUserAgentLib)
}

func normalizeInitiator(val string) string {
	normalized := strings.ToLower(strings.TrimSpace(val))
	if normalized == "user" || normalized == "agent" {
		return normalized
	}
	return ""
}

// inferCopilotInitiator determines X-Initiator based on the last message (oh-my-pi logic).
func inferCopilotInitiator(messages []llm.Message) string {
	if len(messages) == 0 {
		return "user"
	}

	lastMsg := messages[len(messages)-1]

	// Attribution field takes priority
	if normalized := normalizeInitiator(lastMsg.Attribution); normalized != "" {
		return normalized
	}

	// Non-user role indicates agent turn
	if lastMsg.Role != "user" {
		return "agent"
	}

	// Check if LAST content block is tool_result (oh-my-pi checks only the last block)
	if len(lastMsg.Content.MultipleContent) > 0 {
		lastBlock := lastMsg.Content.MultipleContent[len(lastMsg.Content.MultipleContent)-1]
		if lastBlock.Type == "tool_result" {
			return "agent"
		}
	}

	return "user"
}

func resolveCopilotInitiator(llmReq *llm.Request) string {
	initiator := inferCopilotInitiator(llmReq.Messages)

	// Header override takes priority if valid
	if llmReq.RawRequest != nil && llmReq.RawRequest.Headers != nil {
		if val := llmReq.RawRequest.Headers.Get(InitiatorHeader); val != "" {
			if normalized := normalizeInitiator(val); normalized != "" {
				initiator = normalized
			}
		}
	}

	return initiator
}

func hasVisionContent(llmReq *llm.Request) bool {
	for _, msg := range llmReq.Messages {
		if msg.Content.Content != nil {
			if isImageDataURL(*msg.Content.Content) {
				return true
			}
		}

		for _, part := range msg.Content.MultipleContent {
			if part.Type == "image_url" || part.ImageURL != nil {
				return true
			}
			if part.Text != nil && isImageDataURL(*part.Text) {
				return true
			}
		}
	}
	return false
}

func isImageDataURL(content string) bool {
	return strings.HasPrefix(content, "data:image/")
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, errors.New("http response is nil")
	}

	// Check for HTTP error status codes.
	if httpResp.StatusCode >= 400 {
		bodyLen := len(httpResp.Body)
		var bodyMsg string
		if bodyLen == 0 {
			bodyMsg = "(empty body)"
		} else if bodyLen > 100 {
			bodyMsg = fmt.Sprintf("(first 100 chars: %s, total length: %d)", string(httpResp.Body[:100]), bodyLen)
		} else {
			bodyMsg = fmt.Sprintf("(body: %s, length: %d)", string(httpResp.Body), bodyLen)
		}
		return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, bodyMsg)
	}

	// Check for empty response body.
	if len(httpResp.Body) == 0 {
		return nil, errors.New("response body is empty")
	}

	// Check if this is a Responses API response (has "output" field or "object" == "response")
	isResponsesFormat := gjson.GetBytes(httpResp.Body, "output").Exists() ||
		gjson.GetBytes(httpResp.Body, "object").String() == "response"

	// Check for Copilot's wrapped response format: {"response": {...}}
	var unwrappedBody []byte
	if !isResponsesFormat && gjson.GetBytes(httpResp.Body, "response").Exists() {
		// Extract the inner response object
		innerResponse := gjson.GetBytes(httpResp.Body, "response").Raw
		slog.DebugContext(ctx, "Copilot wrapped response detected, extracting inner response")
		isResponsesFormat = gjson.GetBytes([]byte(innerResponse), "output").Exists() ||
			gjson.GetBytes([]byte(innerResponse), "object").String() == "response"
		if isResponsesFormat {
			// Use the unwrapped body for TransformResponse
			unwrappedBody = []byte(innerResponse)
		}
	}

	if isResponsesFormat {
		// Use the responses transformer to parse Responses API format
		// If we have an unwrapped body, create a response with that body
		if len(unwrappedBody) > 0 {
			wrappedResp := &httpclient.Response{
				StatusCode: httpResp.StatusCode,
				Headers:    httpResp.Headers,
				Body:       unwrappedBody,
			}
			return t.responses.TransformResponse(ctx, wrappedResp)
		}
		return t.responses.TransformResponse(ctx, httpResp)
	}

	// Parse into OpenAI Response type (Chat Completions format).
	var oaiResp openai.Response

	err := json.Unmarshal(httpResp.Body, &oaiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert to unified llm.Response.
	return oaiResp.ToLLMResponse(), nil
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	// Check if this is a Responses API format stream (Codex) or Chat Completions format
	// Peek at the first event to determine the format
	var isResponsesFormat bool
	var firstEvent *httpclient.StreamEvent
	if stream.Next() {
		firstEvent = stream.Current()
		if firstEvent != nil && len(firstEvent.Data) > 0 {
			eventType := gjson.GetBytes(firstEvent.Data, "type").String()
			isResponsesFormat = strings.HasPrefix(eventType, "response.")
		}
	}
	// Create a composite stream that preserves true streaming:
	// First yields the firstEvent (if non-nil), then forwards from the original stream
	var compositeStream streams.Stream[*httpclient.StreamEvent]
	if firstEvent != nil {
		compositeStream = &prependedStream{
			firstEvent:   firstEvent,
			upstream:     stream,
			firstYielded: false,
		}
	} else {
		compositeStream = stream
	}

	stream = compositeStream

	if !isResponsesFormat {
		// Non-Codex model: use standard OpenAI chat completions stream transformer
		// Use cached transformer to avoid repeated allocations
		if t.openAITransformer == nil {
			var err error
			t.openAITransformer, err = openai.NewOutboundTransformer(t.baseURL, "")
			if err != nil {
				return nil, fmt.Errorf("failed to create openai transformer: %w", err)
			}
		}
		return t.openAITransformer.TransformStream(ctx, stream)
	}

	// Codex model: process the Responses API format stream
	// Local state for tracking item_id to call_id mapping
	// This allows us to handle multiple concurrent tool calls
	itemIDToCallID := make(map[string]string)
	var mostRecentCallID string

	// For Codex models, we need to convert the Copilot-specific stream format
	// to standard OpenAI Responses API format, then delegate to the responses transformer
	convertedStream := streams.Map(stream, func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
		if event == nil || len(event.Data) == 0 {
			return event
		}

		// Handle [DONE] marker
		if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
			return event
		}

		// Convert Copilot's custom format to standard Responses API format
		convertedData := convertCopilotStreamEvent(ctx, event.Data, itemIDToCallID, &mostRecentCallID)

		if convertedData == nil {
			// Event was consumed (e.g., delta/arguments accumulated)
			return nil
		}

		return &httpclient.StreamEvent{
			Data: convertedData,
		}
	})

	// Filter out nil events
	filteredStream := streams.Filter(convertedStream, func(event *httpclient.StreamEvent) bool {
		return event != nil && len(event.Data) > 0
	})

	// Delegate to the responses transformer's stream handling
	return t.responses.TransformStream(ctx, filteredStream)
}

// convertCopilotStreamEvent fixes up Copilot's standard Responses API stream events.
// Copilot correctly uses the Responses API format, but it sends multiple output_item.added
// events for the same call_id, and it incorrectly sets the item_id on delta/done events.
func convertCopilotStreamEvent(ctx context.Context, data []byte, itemIDToCallID map[string]string, mostRecentCallID *string) []byte {
	eventType := gjson.GetBytes(data, "type").String()

	if eventType == "response.output_item.added" {
		callID := gjson.GetBytes(data, "item.call_id").String()
		if callID != "" {
			// Capture original item ID before overriding
			originalID := gjson.GetBytes(data, "item.id").String()

			// Track the call_id in our mapping - store both original ID and callID
			itemIDToCallID[callID] = callID
			if originalID != "" && originalID != callID {
				itemIDToCallID[originalID] = callID
			}
			*mostRecentCallID = callID

			// Copilot sends an item with arguments="" first, then later sends another item
			// with the full arguments. We must ensure only ONE item is created in the aggregator.
			// The aggregator creates new items based on item.id, so we MUST override the item.id
			// to equal the call_id!
			// By forcing item.id = call_id, the aggregator will merge the second item into the first one
			// instead of creating a duplicate item!

			var event map[string]any
			if err := json.Unmarshal(data, &event); err == nil {
				if item, ok := event["item"].(map[string]any); ok {
					item["id"] = callID // Force ID to match CallID

					// Also provide a fallback name if missing
					if name, ok := item["name"].(string); !ok || name == "" {
						item["name"] = "function"
					}

					event["item"] = item
					if fixedData, err := json.Marshal(event); err == nil {
						return fixedData
					}
				}
			}
		}
	} else if eventType == "response.function_call_arguments.delta" {
		// Copilot uses random hashes for item_id in delta events, which the aggregator can't find.
		// We MUST override the item_id to equal the call_id we forced above.
		// Try to find the call_id from the event's item_id first, then fall back
		itemID := gjson.GetBytes(data, "item_id").String()
		callID := ""
		if itemID != "" {
			callID = itemIDToCallID[itemID]
		}
		// Fallback: use most recent call_id if not found
		if callID == "" {
			callID = *mostRecentCallID
		}
		if callID != "" {
			var event map[string]any
			if err := json.Unmarshal(data, &event); err == nil {
				event["item_id"] = callID
				if fixedData, err := json.Marshal(event); err == nil {
					return fixedData
				}
			}
		}
	} else if eventType == "response.function_call_arguments.done" {
		// Fix item_id for done events too, and also set call_id just in case.
		// Same lookup pattern as delta
		itemID := gjson.GetBytes(data, "item_id").String()
		callID := ""
		if itemID != "" {
			callID = itemIDToCallID[itemID]
		}
		if callID == "" {
			callID = *mostRecentCallID
		}
		if callID != "" {
			var event map[string]any
			if err := json.Unmarshal(data, &event); err == nil {
				event["item_id"] = callID
				event["call_id"] = callID
				if fixedData, err := json.Marshal(event); err == nil {
					return fixedData
				}
			}
		}
	} else if eventType == "response.output_item.done" {
		// If Copilot sends a done event for the item, it might have a random hash for id.
		// Force it to match our call_id so the aggregator updates the right item.
		callID := gjson.GetBytes(data, "item.call_id").String()
		if callID != "" {
			var event map[string]any
			if err := json.Unmarshal(data, &event); err == nil {
				if item, ok := event["item"].(map[string]any); ok {
					item["id"] = callID
					event["item"] = item
					if fixedData, err := json.Marshal(event); err == nil {
						return fixedData
					}
				}
			}
		}
	}

	return data
}

// TransformError transforms an HTTP error to a unified response error.
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

	// Try to parse as OpenAI error format.
	var openaiError struct {
		Error  llm.ErrorDetail `json:"error"`
		Errors llm.ErrorDetail `json:"errors"`
	}

	err := json.Unmarshal(rawErr.Body, &openaiError)
	if err == nil && (openaiError.Error.Message != "" || openaiError.Errors.Message != "") {
		errDetail := openaiError.Error
		if errDetail.Message == "" {
			errDetail = openaiError.Errors
		}

		return &llm.ResponseError{
			StatusCode: rawErr.StatusCode,
			Detail:     errDetail,
		}
	}

	// If JSON parsing fails, use the upstream status text.
	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: http.StatusText(rawErr.StatusCode),
			Type:    "api_error",
		},
	}
}

// AggregateStreamChunks aggregates streaming chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	// Check if chunks are in Responses API format (used by Codex models)
	if isResponsesAPIStream(chunks) {
		return responses.AggregateStreamChunks(ctx, chunks)
	}
	return openai.AggregateStreamChunks(ctx, chunks, openai.DefaultTransformChunk)
}

// isResponsesAPIStream checks if the stream chunks are in OpenAI Responses API format.
func isResponsesAPIStream(chunks []*httpclient.StreamEvent) bool {
	for _, chunk := range chunks {
		if chunk == nil || len(chunk.Data) == 0 {
			continue
		}
		// Check for Responses API specific event types
		data := string(chunk.Data)
		if strings.Contains(data, `"type":"response.completed"`) ||
			strings.Contains(data, `"type":"response.created"`) ||
			strings.Contains(data, `"type":"response.in_progress"`) ||
			strings.Contains(data, `"type":"response.output_item.added"`) {
			return true
		}
	}
	return false
}

// transformResponsesRequest transforms a request for models that use the Responses API.
func (t *OutboundTransformer) transformResponsesRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	// Use the responses transformer to properly convert to Responses API format
	responsesReq, err := t.responses.TransformRequest(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request for Responses API: %w", err)
	}

	slog.DebugContext(ctx, "Codex Responses API request prepared",
		slog.String("url", responsesReq.URL),
		slog.String("model", llmReq.Model))

	// Get Copilot token from token provider.
	token, err := t.tokenProvider.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get copilot token: %w", err)
	}

	// Override auth with Copilot token
	responsesReq.Auth = &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: token,
	}

	// Add Copilot-specific headers
	setCopilotHeaders(responsesReq.Headers)

	// Add vision header if request contains image content.
	if hasVisionContent(llmReq) {
		responsesReq.Headers.Set(CopilotVisionRequestHeader, "true")
	}

	// Determine X-Initiator for Copilot billing control.
	responsesReq.Headers.Set(InitiatorHeader, resolveCopilotInitiator(llmReq))

	return responsesReq, nil
}

// usesResponsesAPI checks if the model uses the responses API.
// GPT-5+ (except gpt-5-mini) uses /responses, everything else uses /chat/completions.
func usesResponsesAPI(model string) bool {
	normalizedModel := strings.ToLower(model)

	// Use package-level compiled regex
	match := modelVersionRegex.FindStringSubmatch(normalizedModel)
	if match == nil {
		return false
	}

	major, err := strconv.Atoi(match[1])
	if err != nil {
		return false
	}

	// Match OpenCode's pattern: GPT-5+ uses responses API (except gpt-5-mini)
	return major >= 5 && !strings.HasPrefix(normalizedModel, "gpt-5-mini")
}

// prependedStream is a stream that yields a first event before forwarding to the upstream stream.
// This preserves true streaming by not buffering the entire response.
type prependedStream struct {
	firstEvent   *httpclient.StreamEvent
	upstream     streams.Stream[*httpclient.StreamEvent]
	firstYielded bool
	current      *httpclient.StreamEvent
}

func (s *prependedStream) Next() bool {
	if !s.firstYielded {
		s.firstYielded = true
		s.current = s.firstEvent
		return s.firstEvent != nil
	}

	// Delegate to upstream and update current
	ok := s.upstream.Next()
	if ok {
		s.current = s.upstream.Current()
	} else {
		s.current = nil
	}
	return ok
}

func (s *prependedStream) Current() *httpclient.StreamEvent {
	if s.firstYielded && s.firstEvent != nil && s.current == s.firstEvent {
		return s.current
	}
	return s.upstream.Current()
}

func (s *prependedStream) Err() error {
	return s.upstream.Err()
}

func (s *prependedStream) Close() error {
	return s.upstream.Close()
}
