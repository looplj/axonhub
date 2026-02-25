package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

const (
	// DefaultCopilotBaseURL is the base URL for GitHub Copilot API.
	DefaultCopilotBaseURL = "https://api.githubcopilot.com"
	CopilotChatCompletionsEndpoint = "/chat/completions"
	EditorVersionHeader        = "editor-version"
	EditorPluginVersionHeader  = "editor-plugin-version"
	UserAgentHeader            = "user-agent"
	OpenAIIntentHeader         = "openai-intent"
	CopilotIntegrationIDHeader = "copilot-integration-id"
	GitHubAPIVersionHeader     = "x-github-api-version"
	RequestIDHeader            = "x-request-id"
	VSCodeUserAgentLibHeader   = "x-vscode-user-agent-library-version"
	CopilotVisionRequestHeader = "Copilot-Vision-Request"
	// Default editor header values (VSCode pattern) - from LiteLLM
	DefaultEditorVersion       = "vscode/1.95.0"
	DefaultEditorPluginVersion = "copilot-chat/0.26.7"
	DefaultUserAgent           = "GitHubCopilotChat/0.26.7"
	DefaultOpenAIIntent        = "conversation-panel"
	DefaultCopilotIntegrationID = "vscode-chat"
	DefaultGitHubAPIVersion    = "2025-04-01"
	DefaultVSCodeUserAgentLib  = "electron-fetch"
)

// TokenProvider defines the interface for getting Copilot tokens.
// This is typically implemented by CopilotTokenProvider.
type TokenProvider interface {
	// GetToken returns a valid Copilot token for API authentication.
	GetToken(ctx context.Context) (string, error)
}

// OutboundTransformer implements transformer.Outbound for GitHub Copilot.
// It transforms unified LLM requests to GitHub Copilot API format with LiteLLM-style headers.
type OutboundTransformer struct {
	tokenProvider  TokenProvider
	baseURL        string
	codexResponses *responses.OutboundTransformer
}



// OutboundTransformerParams contains the parameters for creating a new OutboundTransformer.
type OutboundTransformerParams struct {
	// TokenProvider provides Copilot tokens for authentication (required).
	TokenProvider TokenProvider

	// BaseURL is the base URL for the Copilot API (optional, defaults to DefaultCopilotBaseURL).
	BaseURL string
}

// Compile-time interface check.
var _ transformer.Outbound = (*OutboundTransformer)(nil)

// NewOutboundTransformer creates a new GitHub Copilot outbound transformer.
func NewOutboundTransformer(params OutboundTransformerParams) (*OutboundTransformer, error) {
	if params.TokenProvider == nil {
		return nil, errors.New("token provider is required")
	}

	baseURL := params.BaseURL
	if baseURL == "" {
		baseURL = DefaultCopilotBaseURL
	}

	// Normalize base URL (remove trailing slash)
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Create responses transformer for Codex models
	responsesTransformer, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(""),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create responses transformer: %w", err)
	}

	return &OutboundTransformer{
		tokenProvider:  params.TokenProvider,
		baseURL:        baseURL,
		codexResponses: responsesTransformer,
	}, nil
}

// APIFormat returns the API format for this transformer.
func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIChatCompletion
}

// TransformRequest transforms a unified LLM request to a GitHub Copilot HTTP request.
// It adds LiteLLM-style editor headers required by the Copilot API.
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

	// Check if this is a Codex model - use Responses API for Codex
	if isCodexModel(llmReq.Model) {
		return t.transformCodexResponsesRequest(ctx, llmReq)
	}

	// Get Copilot token from token provider.
	token, err := t.tokenProvider.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get copilot token: %w", err)
	}

	// Convert to OpenAI request format.
	oaiReq := openai.RequestFromLLM(llmReq)

	// Marshal request body.
	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL.
	url := t.baseURL + CopilotChatCompletionsEndpoint

	// Prepare headers with LiteLLM-style editor headers.
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Add LiteLLM-style editor headers (required by Copilot).
	headers.Set(EditorVersionHeader, DefaultEditorVersion)
	headers.Set(EditorPluginVersionHeader, DefaultEditorPluginVersion)
	headers.Set(UserAgentHeader, DefaultUserAgent)
	headers.Set(CopilotIntegrationIDHeader, DefaultCopilotIntegrationID)
	headers.Set(OpenAIIntentHeader, DefaultOpenAIIntent)
	headers.Set(GitHubAPIVersionHeader, DefaultGitHubAPIVersion)
	headers.Set(VSCodeUserAgentLibHeader, DefaultVSCodeUserAgentLib)

	// Add vision header if request contains image content.
	if hasVisionContent(llmReq) {
		headers.Set(CopilotVisionRequestHeader, "true")
	}

	// Build authentication config.
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

// hasVisionContent checks if the request contains image content (vision capabilities).
// It returns true if any message contains image_url content or data URLs.
func hasVisionContent(llmReq *llm.Request) bool {
	for _, msg := range llmReq.Messages {
		// Check single content.
		if msg.Content.Content != nil {
			content := *msg.Content.Content
			if isImageDataURL(content) {
				return true
			}
		}

		// Check multiple content parts.
		for _, part := range msg.Content.MultipleContent {
			// Check for image_url type.
			if part.Type == "image_url" || part.ImageURL != nil {
				return true
			}

			// Check for data URLs in text.
			if part.Text != nil && isImageDataURL(*part.Text) {
				return true
			}
		}
	}

	return false
}

// isImageDataURL checks if the content is an image data URL.
func isImageDataURL(content string) bool {
	return strings.HasPrefix(content, "data:image/")
}

// TransformResponse transforms a GitHub Copilot HTTP response to a unified LLM response.
func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, errors.New("http response is nil")
	}

	// Check for HTTP error status codes.
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, string(httpResp.Body))
	}

	// Check for empty response body.
	if len(httpResp.Body) == 0 {
		return nil, errors.New("response body is empty")
	}

	// Check if this is a Responses API response (has "output" field or "object" == "response")
	isResponsesFormat := gjson.GetBytes(httpResp.Body, "output").Exists() ||
		gjson.GetBytes(httpResp.Body, "object").String() == "response"

	// Check for Copilot's wrapped response format: {"response": {...}}
	if !isResponsesFormat && gjson.GetBytes(httpResp.Body, "response").Exists() {
		// Extract the inner response object
		innerResponse := gjson.GetBytes(httpResp.Body, "response").Raw
		slog.InfoContext(ctx, "Copilot wrapped response detected, extracting inner response")
		httpResp.Body = []byte(innerResponse)
		isResponsesFormat = gjson.GetBytes(httpResp.Body, "output").Exists() ||
			gjson.GetBytes(httpResp.Body, "object").String() == "response"
	}


	if isResponsesFormat {
		// Use the responses transformer to parse Responses API format
		return t.codexResponses.TransformResponse(ctx, httpResp)
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

	// Collect all remaining events from the stream
	remainingEvents := collectStreamEvents(stream)

	// Reconstruct the full stream including the first event
	allEvents := make([]*httpclient.StreamEvent, 0, 1+len(remainingEvents))
	if firstEvent != nil {
		allEvents = append(allEvents, firstEvent)
	}
	allEvents = append(allEvents, remainingEvents...)
	stream = streams.SliceStream(allEvents)

	if !isResponsesFormat {
		// Non-Codex model: use standard OpenAI chat completions stream transformer
		openaiTransformer, err := openai.NewOutboundTransformer(t.baseURL, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create openai transformer: %w", err)
		}
		return openaiTransformer.TransformStream(ctx, stream)
	}

	// Codex model: process the Responses API format stream
	// Local state for tracking item_id to call_id mapping
	// This allows us to handle multiple concurrent tool calls
	itemIDToCallID := make(map[string]string)

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
		convertedData := convertCopilotStreamEvent(ctx, event.Data, itemIDToCallID)

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
	return t.codexResponses.TransformStream(ctx, filteredStream)
}

// collectStreamEvents collects all remaining events from a stream into a slice
func collectStreamEvents(stream streams.Stream[*httpclient.StreamEvent]) []*httpclient.StreamEvent {
	var events []*httpclient.StreamEvent
	for stream.Next() {
		if event := stream.Current(); event != nil {
			events = append(events, event)
		}
	}
	return events
}


// convertCopilotStreamEvent fixes up Copilot's standard Responses API stream events.
// Copilot correctly uses the Responses API format, but it sends multiple output_item.added
// events for the same call_id, and it incorrectly sets the item_id on delta/done events.
func convertCopilotStreamEvent(ctx context.Context, data []byte, itemIDToCallID map[string]string) []byte {
	eventType := gjson.GetBytes(data, "type").String()

	if eventType == "response.output_item.added" {
		callID := gjson.GetBytes(data, "item.call_id").String()
		if callID != "" {
			// Track the call_id in our mapping
			itemIDToCallID[callID] = callID

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
		// Fallback: use any known call_id if not found
		if callID == "" {
			for _, cid := range itemIDToCallID {
				callID = cid
				break
			}
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
			for _, cid := range itemIDToCallID {
				callID = cid
				break
			}
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

// transformCodexResponsesRequest transforms a request for Codex models using the Responses API.
func (t *OutboundTransformer) transformCodexResponsesRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	// Use the responses transformer to properly convert to Responses API format
	responsesReq, err := t.codexResponses.TransformRequest(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request for Responses API: %w", err)
	}

	slog.InfoContext(ctx, "Codex Responses API request prepared",
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
	responsesReq.Headers.Set(EditorVersionHeader, DefaultEditorVersion)
	responsesReq.Headers.Set(EditorPluginVersionHeader, DefaultEditorPluginVersion)
	responsesReq.Headers.Set(UserAgentHeader, DefaultUserAgent)
	responsesReq.Headers.Set(CopilotIntegrationIDHeader, DefaultCopilotIntegrationID)
	responsesReq.Headers.Set(OpenAIIntentHeader, DefaultOpenAIIntent)
	responsesReq.Headers.Set(GitHubAPIVersionHeader, DefaultGitHubAPIVersion)
	responsesReq.Headers.Set(VSCodeUserAgentLibHeader, DefaultVSCodeUserAgentLib)

	// Add vision header if request contains image content.
	if hasVisionContent(llmReq) {
		responsesReq.Headers.Set(CopilotVisionRequestHeader, "true")
	}

	return responsesReq, nil
}

// isCodexModel checks if the model is a Codex model.
func isCodexModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "codex")
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

