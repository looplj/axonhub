package responses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/internal/pkg/xmap"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

var _ transformer.Inbound = (*InboundTransformer)(nil)

// InboundTransformer implements transformer.Inbound for OpenAI Responses API format.
type InboundTransformer struct{}

// NewInboundTransformer creates a new OpenAI Responses InboundTransformer.
func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

// APIFormat returns the API format of the transformer.
func (t *InboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIResponse
}

// TransformRequest transforms OpenAI Responses API HTTP request to llm.Request.
func (t *InboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	// Check content type
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	var req Request
	if err := json.Unmarshal(httpReq.Body, &req); err != nil {
		return nil, fmt.Errorf("%w: failed to decode responses api request: %w", transformer.ErrInvalidRequest, err)
	}

	// Validate required fields
	if req.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	return convertToLLMRequest(&req, httpReq.Body)
}

// TransformResponse transforms llm.Response to OpenAI Responses API HTTP response.
func (t *InboundTransformer) TransformResponse(ctx context.Context, chatResp *llm.Response) (*httpclient.Response, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	// Convert to Responses API format
	resp := convertToResponsesAPIResponse(chatResp)

	body, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal responses api response: %w", err)
	}
	body, err = restoreOpenAIResponsesResponseTopLevelFields(body, chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to restore responses api response fields: %w", err)
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

func restoreOpenAIResponsesResponseTopLevelFields(body []byte, chatResp *llm.Response) ([]byte, error) {
	if chatResp.ProviderExtensions == nil || chatResp.ProviderExtensions.OpenAIResponses == nil ||
		chatResp.ProviderExtensions.OpenAIResponses.Response == nil {
		return body, nil
	}
	rawFields := chatResp.ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields
	if len(rawFields) == 0 {
		return body, nil
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	changed := false
	for _, field := range openAIResponsesRawResponseTopLevelFields {
		if _, typed := envelope[field]; typed {
			continue
		}
		raw := rawFields[field]
		if len(raw) == 0 || !json.Valid(raw) {
			continue
		}
		envelope[field] = append(json.RawMessage(nil), raw...)
		changed = true
	}
	if !changed {
		return body, nil
	}
	return json.Marshal(envelope)
}

type ResponseError struct {
	Error ResponseErrorDetail `json:"error"`
}

type ResponseErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// TransformError transforms LLM error response to HTTP error response in Responses API format.
func (t *InboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	if rawErr == nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body:       xjson.MustMarshal(&ResponseError{Error: ResponseErrorDetail{Message: "internal server error", Type: "internal_error"}}),
		}
	}

	if errors.Is(rawErr, transformer.ErrInvalidModel) {
		return &httpclient.Error{
			StatusCode: http.StatusUnprocessableEntity,
			Status:     http.StatusText(http.StatusUnprocessableEntity),
			Body:       xjson.MustMarshal(&ResponseError{Error: ResponseErrorDetail{Message: rawErr.Error(), Type: "invalid_model_error"}}),
		}
	}

	if llmErr, ok := errors.AsType[*llm.ResponseError](rawErr); ok {
		errResp := ResponseError{
			Error: ResponseErrorDetail{
				Message: llmErr.Detail.Message,
				Type:    llmErr.Detail.Type,
				Code:    llmErr.Detail.Code,
			},
		}

		return &httpclient.Error{
			StatusCode: llmErr.StatusCode,
			Status:     http.StatusText(llmErr.StatusCode),
			Body:       xjson.MustMarshal(&errResp),
		}
	}

	if httpErr, ok := errors.AsType[*httpclient.Error](rawErr); ok {
		return httpErr
	}

	// Handle validation errors
	if errors.Is(rawErr, transformer.ErrInvalidRequest) {
		errResp := ResponseError{
			Error: ResponseErrorDetail{
				Message: rawErr.Error(),
				Type:    "invalid_request_error",
			},
		}

		return &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Status:     http.StatusText(http.StatusBadRequest),
			Body:       xjson.MustMarshal(&errResp),
		}
	}

	errResp := ResponseError{
		Error: ResponseErrorDetail{
			Message: rawErr.Error(),
			Type:    "internal_error",
		},
	}

	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       xjson.MustMarshal(&errResp),
	}
}

// convertToLLMRequest converts OpenAI Responses API Request to llm.Request.
func convertToLLMRequest(req *Request, rawBody ...[]byte) (*llm.Request, error) {
	chatReq := &llm.Request{
		Model:               req.Model,
		Temperature:         req.Temperature,
		FrequencyPenalty:    req.FrequencyPenalty,
		PresencePenalty:     req.PresencePenalty,
		Stream:              req.Stream,
		Metadata:            maps.Clone(req.Metadata),
		RequestType:         llm.RequestTypeChat,
		APIFormat:           llm.APIFormatOpenAIResponse,
		MaxCompletionTokens: req.MaxOutputTokens,
		User:                req.User,
		Store:               req.Store,
		TopLogprobs:         req.TopLogprobs,
		TopP:                req.TopP,
		SafetyIdentifier:    req.SafetyIdentifier,
		ServiceTier:         req.ServiceTier,
		ParallelToolCalls:   req.ParallelToolCalls,
		PromptCacheKey:      req.PromptCacheKey,
		PreviousResponseID:  req.PreviousResponseID,
		TransformerMetadata: map[string]any{},
		TransformOptions:    llm.TransformOptions{},
	}

	// Preserve top_k through TransformerMetadata; canonical llm.Request has no
	// TopK field, so without this the sampling parameter is dropped on cross-format
	// conversion (mirrors Anthropic top_k handling, shared neutral key).
	if req.TopK != nil {
		topK := *req.TopK
		chatReq.TransformerMetadata[shared.TransformerMetadataKeyTopK] = &topK
	}

	// Convert reasoning
	if req.Reasoning != nil {
		if req.Reasoning.Effort != "" {
			chatReq.ReasoningEffort = req.Reasoning.Effort
		}

		if req.Reasoning.MaxTokens != nil {
			chatReq.ReasoningBudget = req.Reasoning.MaxTokens
		}

		// Keep summary and deprecated generate_summary as distinct identities.
		// summary maps to common ReasoningSummary; generate_summary is also
		// projected there only when summary is absent, but always retains a
		// separate origin/value sidecar for same-protocol wire fidelity.
		if req.Reasoning.Summary != "" {
			chatReq.ReasoningSummary = lo.ToPtr(req.Reasoning.Summary)
		} else if req.Reasoning.GenerateSummary != "" {
			chatReq.ReasoningSummary = lo.ToPtr(req.Reasoning.GenerateSummary)
			chatReq.TransformerMetadata[responsesReasoningGenerateSummaryOriginTransformerMetadataKey] = true
		}
		if req.Reasoning.GenerateSummary != "" {
			chatReq.TransformerMetadata[responsesReasoningGenerateSummaryValueTransformerMetadataKey] = req.Reasoning.GenerateSummary
		}

		// Preserve reasoning.enabled through TransformerMetadata; canonical
		// llm.Request has no Enabled slot, so without this the toggle is dropped
		// on cross-format conversion (mirrors top_k/output_config handling).
		if req.Reasoning.Enabled != nil {
			chatReq.TransformerMetadata[responsesReasoningEnabledTransformerMetadataKey] = req.Reasoning.Enabled
		}

		// reasoning.context is Responses-native configuration owned by
		// ProviderExtensions.OpenAIResponses.Request.ReasoningContext
		// (attachOpenAIResponsesRequestExtensions). Do not dual-write into
		// TransformerMetadata.
	}

	// Preserve top-level cache_control (OpenRouter/Anthropic prompt-caching
	// marker) through TransformerMetadata as opaque json.RawMessage; canonical
	// llm.Request has no CacheControl field, so without this it is dropped on
	// cross-format conversion (mirrors top_k handling).
	if req.CacheControl != nil {
		if b, err := json.Marshal(req.CacheControl); err == nil {
			chatReq.TransformerMetadata[shared.TransformerMetadataKeyCacheControl] = json.RawMessage(b)
		}
	}

	// Convert tool choice
	if req.ToolChoice != nil {
		chatReq.ToolChoice = convertToolChoiceToLLM(req.ToolChoice)
	}

	// Convert stream options
	if req.StreamOptions != nil {
		chatReq.StreamOptions = &llm.StreamOptions{}
	}

	// Convert instructions to system message
	messages := make([]llm.Message, 0)
	if req.Instructions != "" {
		messages = append(messages, llm.Message{
			Role: "system",
			Content: llm.MessageContent{
				Content: lo.ToPtr(req.Instructions),
			},
		})
	}

	// Convert input to messages
	if req.Input.Items != nil {
		chatReq.TransformOptions.ArrayInputs = lo.ToPtr(true)
	}

	inputMessages, err := convertInputToMessages(&req.Input)
	if err != nil {
		return nil, err
	}

	messages = append(messages, inputMessages...)

	chatReq.Messages = messages

	if len(req.Tools) > 0 {
		tools, err := convertToolsToLLM(req.Tools, chatReq.TransformerMetadata)
		if err != nil {
			return nil, err
		}

		chatReq.Tools = tools
	}

	// Convert text format to response format
	if req.Text != nil && req.Text.Format != nil && req.Text.Format.Type != "" {
		chatReq.ResponseFormat = &llm.ResponseFormat{
			Type: req.Text.Format.Type,
		}

		// Reconstruct json_schema from TextFormat fields
		if req.Text.Format.Type == "json_schema" && req.Text.Format.Name != "" {
			jsonSchema := rawJSONSchema{
				Name:        req.Text.Format.Name,
				Description: req.Text.Format.Description,
				Schema:      req.Text.Format.Schema,
				Strict:      req.Text.Format.Strict,
			}
			if data, err := json.Marshal(jsonSchema); err == nil {
				chatReq.ResponseFormat.JSONSchema = data
			}
		}
	}

	// Convert text verbosity
	if req.Text != nil {
		chatReq.Verbosity = req.Text.Verbosity
	}

	var rawRequestBody []byte
	if len(rawBody) > 0 {
		rawRequestBody = rawBody[0]
	}
	attachOpenAIResponsesRequestExtensions(chatReq, req, rawRequestBody)
	if len(rawRequestBody) > 0 {
		if rawReasoning := extractRawReasoningObject(rawRequestBody); len(rawReasoning) > 0 {
			chatReq.TransformerMetadata[responsesReasoningRawObjectTransformerMetadataKey] = rawReasoning
		}
	}

	return chatReq, nil
}

func extractRawReasoningObject(rawBody []byte) json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &obj); err != nil {
		return nil
	}
	raw, ok := obj["reasoning"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

// convertToolChoiceToLLM converts Responses API ToolChoice to llm.ToolChoice.
func convertToolChoiceToLLM(src *ToolChoice) *llm.ToolChoice {
	if src == nil {
		return nil
	}

	result := &llm.ToolChoice{}

	if src.Mode != nil {
		result.ToolChoice = src.Mode
		return result
	}
	if src.Type == nil || src.Name == nil {
		return nil
	}

	switch *src.Type {
	case "custom":
		result.OpenAIChatCustomToolChoice = &llm.OpenAIChatCustomToolChoice{
			Name: *src.Name,
		}
	case "function":
		result.NamedToolChoice = &llm.NamedToolChoice{
			Type: *src.Type,
			Function: llm.ToolFunction{
				Name: *src.Name,
			},
		}
	default:
		return nil
	}

	return result
}

// convertInputToMessages converts Responses API input to llm.Message slice.
// It handles merging reasoning items with subsequent function_call items into a single assistant message.
func convertInputToMessages(input *Input) ([]llm.Message, error) {
	if input == nil {
		return nil, nil
	}

	// If input is a simple text string
	if input.Text != nil {
		return []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: input.Text,
				},
			},
		}, nil
	}

	// If input is an array of items
	messages := make([]llm.Message, 0, len(input.Items))
	i := 0

	for i < len(input.Items) {
		item := &input.Items[i]

		// Handle reasoning item - merge with subsequent function_call or text items
		if item.Type == "reasoning" {
			msg, consumed, err := convertReasoningWithFollowing(input.Items, i)
			if err != nil {
				return nil, err
			}

			if msg != nil {
				messages = append(messages, *msg)
			}

			i += consumed

			continue
		}

		// Chat Completions requires parallel tool calls to be represented in one
		// assistant message with multiple tool_calls entries. Responses represents
		// those calls as consecutive input items, so preserve that grouping before
		// the following function_call_output items become individual tool messages.
		if item.Type == "function_call" || item.Type == "custom_tool_call" {
			msg, consumed, err := convertConsecutiveToolCalls(input.Items, i)
			if err != nil {
				return nil, err
			}
			messages = append(messages, *msg)
			i += consumed
			continue
		}

		// Handle regular items
		msg, err := convertItemToMessage(item)
		if err != nil {
			return nil, err
		}

		if msg != nil {
			messages = append(messages, *msg)
		}

		i++
	}

	return messages, nil
}

// convertConsecutiveToolCalls merges adjacent Responses tool-call input items
// into one canonical assistant message. The next output item deliberately ends
// the group, so it is emitted as its own role="tool" message by the caller.
//
// Reasoning items that appear between a tool call and its tool output are also
// attached to this assistant message. Chat providers validate tool history as
// assistant(tool_calls) immediately followed by role=tool; an intervening
// assistant-only reasoning message is rejected as a missing tool output.
func convertConsecutiveToolCalls(items []Item, startIdx int) (*llm.Message, int, error) {
	msg := &llm.Message{Role: "assistant"}
	consumed := 0
	var reasoningText strings.Builder

	for index := startIdx; index < len(items); index++ {
		item := &items[index]
		switch item.Type {
		case "function_call", "custom_tool_call":
			toolCallMessage, err := convertItemToMessage(item)
			if err != nil {
				return nil, 0, err
			}
			if toolCallMessage != nil {
				msg.ToolCalls = append(msg.ToolCalls, toolCallMessage.ToolCalls...)
			}
			consumed++
		case "reasoning":
			// Only fold mid-call reasoning once at least one tool call has been
			// collected. Leading reasoning is handled by convertReasoningWithFollowing.
			if len(msg.ToolCalls) == 0 {
				if reasoningText.Len() > 0 {
					msg.ReasoningContent = lo.ToPtr(reasoningText.String())
				}
				return msg, consumed, nil
			}
			if msg.ResponseReasoningItemID == nil {
				reasoningItemID := item.ID
				msg.ResponseReasoningItemID = &reasoningItemID
			}
			if msg.ReasoningSignature == nil && item.EncryptedContent != nil {
				msg.ReasoningSignature = item.EncryptedContent
			}
			// Prefer raw reasoning_text content[] over summary when both exist.
			itemText := strings.Builder{}
			for _, part := range item.ReasoningContent {
				if part.Text != "" {
					itemText.WriteString(part.Text)
				}
			}
			if itemText.Len() == 0 {
				for _, summary := range item.Summary {
					itemText.WriteString(summary.Text)
				}
			}
			if itemText.Len() > 0 {
				reasoningText.WriteString(itemText.String())
			}
			consumed++
		default:
			// function_call_output / custom_tool_call_output / other items end
			// the assistant tool-call group so outputs stay adjacent.
			if reasoningText.Len() > 0 {
				msg.ReasoningContent = lo.ToPtr(reasoningText.String())
			}
			return msg, consumed, nil
		}
	}

	if reasoningText.Len() > 0 {
		msg.ReasoningContent = lo.ToPtr(reasoningText.String())
	}
	return msg, consumed, nil
}

// convertReasoningWithFollowing converts a reasoning item and merges it with subsequent
// function_call items or text content into a single assistant message.
// Returns the merged message and the number of items consumed.
func convertReasoningWithFollowing(items []Item, startIdx int) (*llm.Message, int, error) {
	if startIdx >= len(items) || items[startIdx].Type != "reasoning" {
		return nil, 0, nil
	}

	reasoningItem := &items[startIdx]
	// Always mark Responses reasoning origin. Empty string means source omitted id;
	// do not leave the pointer nil or outbound cannot distinguish from Chat/Anthropic
	// ReasoningContent.
	reasoningItemID := reasoningItem.ID
	msg := &llm.Message{
		Role:                    "assistant",
		ReasoningSignature:      reasoningItem.EncryptedContent,
		ResponseReasoningItemID: &reasoningItemID,
	}

	// Prefer raw reasoning_text content[] over summary when both exist.
	var reasoningText strings.Builder
	for _, part := range reasoningItem.ReasoningContent {
		if part.Text != "" {
			reasoningText.WriteString(part.Text)
		}
	}
	if reasoningText.Len() == 0 {
		for _, summary := range reasoningItem.Summary {
			reasoningText.WriteString(summary.Text)
		}
	}
	if reasoningText.Len() > 0 {
		msg.ReasoningContent = lo.ToPtr(reasoningText.String())
	}

	consumed := 1

	// Look ahead for subsequent function_call items to merge
	for i := startIdx + 1; i < len(items); i++ {
		nextItem := &items[i]

		switch nextItem.Type {
		case "function_call":
			// Merge function_call into the same assistant message
			msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{
				ID:             nextItem.CallID,
				ResponseItemID: nextItem.ID,
				Type:           "function",
				Function: llm.FunctionCall{
					Name:      nextItem.Name,
					Namespace: nextItem.Namespace,
					Arguments: nextItem.Arguments,
				},
			})
			consumed++

		case "custom_tool_call":
			// Merge custom_tool_call into the same assistant message
			inputStr := ""
			if nextItem.Input != nil {
				inputStr = *nextItem.Input
			}

			msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{
				ID:             nextItem.CallID,
				ResponseItemID: nextItem.ID,
				Type:           llm.ToolTypeResponsesCustomTool,
				ResponseCustomToolCall: &llm.ResponseCustomToolCall{
					CallID:    nextItem.CallID,
					Name:      nextItem.Name,
					Namespace: nextItem.Namespace,
					Input:     inputStr,
				},
			})
			consumed++

		case "message", "input_text", "":
			// If we encounter a text message with assistant role, merge its content
			if nextItem.Role == "assistant" {
				msg.ID = nextItem.ID
				if nextItem.Content != nil && len(nextItem.Content.Items) > 0 && nextItem.isOutputMessageContent() {
					msg.Content = convertContentItemsToMessageContent(nextItem.GetContentItems())
				} else if nextItem.Content != nil {
					msg.Content = convertToMessageContent(*nextItem.Content)
				} else if nextItem.Text != nil {
					msg.Content = llm.MessageContent{Content: nextItem.Text}
				}

				consumed++
			} else {
				// Non-assistant message, stop merging
				return msg, consumed, nil
			}

		default:
			// Any other type (including function_call_output), stop merging
			return msg, consumed, nil
		}
	}

	return msg, consumed, nil
}

// convertItemToMessage converts a single input item to an llm.Message.
func convertItemToMessage(item *Item) (*llm.Message, error) {
	if item == nil {
		return nil, nil
	}

	switch item.Type {
	case "message", "input_text", "":
		msg := &llm.Message{
			ID:   item.ID,
			Role: item.Role,
		}

		// Handle content - check Content.Items first (output message format from JSON)
		if item.Content != nil && len(item.Content.Items) > 0 && item.isOutputMessageContent() {
			msg.Content = convertContentItemsToMessageContent(item.GetContentItems())
		} else if item.Content != nil {
			msg.Content = convertToMessageContent(*item.Content)
		} else if item.Text != nil {
			msg.Content = llm.MessageContent{Content: item.Text}
		}

		return msg, nil
	case "input_image":
		// Input image as a standalone item
		if item.ImageURL != nil {
			return &llm.Message{
				Role: lo.Ternary(item.Role != "", item.Role, "user"),
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL:    *item.ImageURL,
								Detail: item.Detail,
							},
						},
					},
				},
			}, nil
		}

		return nil, nil

	case "input_audio":
		// Input audio as a standalone item
		if item.InputAudio != nil {
			return &llm.Message{
				Role: lo.Ternary(item.Role != "", item.Role, "user"),
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type:       "input_audio",
							InputAudio: item.InputAudio,
						},
					},
				},
			}, nil
		}

		return nil, nil

	case "input_file":
		part, err := convertContentItemToPart(item)
		if err != nil || part == nil {
			return nil, err
		}

		return &llm.Message{
			Role: lo.Ternary(item.Role != "", item.Role, "user"),
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{*part},
			},
		}, nil

	case "function_call":
		// Function call from assistant - convert to tool call.
		// item.ID is the Responses item identity; item.CallID is the tool-call
		// correlation id. Keep them separate for same-protocol replay.
		return &llm.Message{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{
					ID:             item.CallID,
					ResponseItemID: item.ID,
					Type:           "function",
					Function: llm.FunctionCall{
						Name:      item.Name,
						Namespace: item.Namespace,
						Arguments: item.Arguments,
					},
				},
			},
		}, nil

	case "custom_tool_call":
		// Custom tool call from assistant - convert to tool call with ResponseCustomToolCall
		inputStr := ""
		if item.Input != nil {
			inputStr = *item.Input
		}

		return &llm.Message{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{
					ID:             item.CallID,
					ResponseItemID: item.ID,
					Type:           llm.ToolTypeResponsesCustomTool,
					ResponseCustomToolCall: &llm.ResponseCustomToolCall{
						CallID:    item.CallID,
						Name:      item.Name,
						Namespace: item.Namespace,
						Input:     inputStr,
					},
				},
			},
		}, nil

	case "function_call_output":
		if item.Output == nil {
			return nil, fmt.Errorf("%w: %s", transformer.ErrInvalidRequest, "function_call_output item must have non-nil Output")
		}
		// Function call output - convert to tool message
		msg := &llm.Message{
			ID:         item.ID,
			Role:       "tool",
			ToolCallID: lo.ToPtr(item.CallID),
			Content:    convertToMessageContent(*item.Output),
		}
		if item.Name != "" {
			msg.ToolCallName = lo.ToPtr(item.Name)
		}

		return msg, nil

	case "custom_tool_call_output":
		if item.Output == nil {
			return nil, fmt.Errorf("%w: %s", transformer.ErrInvalidRequest, "custom_tool_call_output item must have non-nil Output")
		}
		// Custom tool call output - convert to tool message
		msg := &llm.Message{
			ID:         item.ID,
			Role:       "tool",
			ToolCallID: lo.ToPtr(item.CallID),
			Content:    convertToMessageContent(*item.Output),
		}
		if item.Name != "" {
			msg.ToolCallName = lo.ToPtr(item.Name)
		}

		return msg, nil

	case "reasoning":
		// Reasoning is handled by convertReasoningWithFollowing in convertInputToMessages
		// This case should not be reached in normal flow, but return nil to skip if it does
		return nil, nil

	case "compaction", "compaction_summary":
		return compactionMessageFromItem(item, item.Type), nil

	default:
		// Unknown/raw-only input items are preserved via request extensions when the
		// original body is available. Do not invent a canonical message shape here.
		return nil, nil
	}
}

func convertToMessageContent(content Input) llm.MessageContent {
	items := convertToMessageContentParts(content)
	// If only one text item, return simple Content
	if len(items) == 1 && (items[0].Type == "text" || items[0].Type == "input_text") && items[0].Text != nil {
		return llm.MessageContent{
			Content: items[0].Text,
		}
	}

	return llm.MessageContent{
		MultipleContent: items,
	}
}

// convertContentItemsToMessageContent converts []ContentItem to llm.MessageContent.
// This handles the output message format where content is an array of ContentItem.
func convertContentItemsToMessageContent(items []ContentItem) llm.MessageContent {
	// If only one text item, return simple Content
	if len(items) == 1 && (items[0].Type == "output_text" || items[0].Type == "input_text" || items[0].Type == "text") {
		return llm.MessageContent{
			Content: lo.ToPtr(items[0].Text),
		}
	}

	// Convert to MultipleContent
	parts := make([]llm.MessageContentPart, 0, len(items))
	for _, item := range items {
		switch item.Type {
		case "output_text", "input_text", "text":
			parts = append(parts, llm.MessageContentPart{
				Type: "text",
				Text: lo.ToPtr(item.Text),
			})
		}
	}

	return llm.MessageContent{
		MultipleContent: parts,
	}
}

// convertToMessageContentParts converts content items to []llm.MessageContentPart.
func convertToMessageContentParts(input Input) []llm.MessageContentPart {
	if input.Text != nil {
		return []llm.MessageContentPart{
			{
				Type: "input_text",
				Text: input.Text,
			},
		}
	}

	parts := make([]llm.MessageContentPart, 0, len(input.Items))
	for i := range input.Items {
		part, err := convertContentItemToPart(&input.Items[i])
		if err != nil || part == nil {
			continue
		}

		parts = append(parts, *part)
	}

	return parts
}

// convertContentItemToPart converts a content item to llm.MessageContentPart.
func convertContentItemToPart(item *Item) (*llm.MessageContentPart, error) {
	if item == nil {
		return nil, nil
	}

	switch item.Type {
	case "input_text", "text", "output_text":
		if item.Text != nil {
			return &llm.MessageContentPart{
				ID:   item.ID,
				Type: "text",
				Text: item.Text,
			}, nil
		}

		return nil, nil

	case "input_image":
		if item.ImageURL != nil {
			return &llm.MessageContentPart{
				ID:   item.ID,
				Type: "image_url",
				ImageURL: &llm.ImageURL{
					URL:    *item.ImageURL,
					Detail: item.Detail,
				},
			}, nil
		}

		return nil, nil

	case "input_audio":
		if item.InputAudio != nil {
			return &llm.MessageContentPart{
				ID:         item.ID,
				Type:       "input_audio",
				InputAudio: item.InputAudio,
			}, nil
		}

		return nil, nil

	case "input_file":
		metadata := map[string]any{}
		if item.FileURL != nil {
			metadata[responsesInputFileURLPartTransformerMetadataKey] = item.FileURL
		}
		if item.Detail != nil {
			metadata[responsesInputFileDetailPartTransformerMetadataKey] = item.Detail
		}
		if len(metadata) == 0 {
			metadata = nil
		}

		return &llm.MessageContentPart{
			ID:   item.ID,
			Type: "file",
			OpenAIChatFile: &llm.OpenAIChatFileContentPart{
				FileData: item.FileData,
				FileID:   item.FileID,
				Filename: item.Filename,
			},
			TransformerMetadata: metadata,
		}, nil

	case "compaction", "compaction_summary":
		return compactionContentPartFromItem(item, item.Type), nil

	default:
		return nil, nil
	}
}

// convertToolsToLLM converts Responses API tools to llm.Tool slice.
func convertToolsToLLM(tools []Tool, metadata map[string]any) ([]llm.Tool, error) {
	result := make([]llm.Tool, 0, len(tools))

	for _, tool := range tools {
		switch tool.Type {
		case "function":
			params, err := json.Marshal(tool.Parameters)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal function parameters: %w", err)
			}

			result = append(result, llm.Tool{
				Type: "function",
				Function: llm.Function{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  params,
					Strict:      tool.Strict,
				},
			})

		case "image_generation":
			result = append(result, llm.Tool{
				Type: llm.ToolTypeImageGeneration,
				ImageGeneration: &llm.ImageGeneration{
					Model:             tool.Model,
					Background:        tool.Background,
					InputFidelity:     tool.InputFidelity,
					InputImageMask:    tool.InputImageMask,
					Moderation:        tool.Moderation,
					OutputCompression: tool.OutputCompression,
					OutputFormat:      tool.OutputFormat,
					PartialImages:     tool.PartialImages,
					Quality:           tool.Quality,
					Size:              tool.Size,
				},
			})

		case "web_search":
			webSearch := &llm.WebSearch{}
			if tool.Filters != nil {
				webSearch.AllowedDomains = append(webSearch.AllowedDomains, tool.Filters.AllowedDomains...)
			}
			if tool.UserLocation != nil {
				locationType := tool.UserLocation.Type
				if locationType == "" {
					locationType = "approximate"
				}
				webSearch.UserLocation = llm.WebSearchToolUserLocation{
					Type:     locationType,
					City:     tool.UserLocation.City,
					Country:  tool.UserLocation.Country,
					Region:   tool.UserLocation.Region,
					Timezone: tool.UserLocation.Timezone,
				}
			}
			result = append(result, llm.Tool{
				Type:      llm.ToolTypeWebSearch,
				WebSearch: webSearch,
			})

		case "custom":
			customTool := &llm.ResponseCustomTool{
				Name:        tool.Name,
				Description: tool.Description,
			}
			if tool.Format != nil {
				customTool.Format = &llm.ResponseCustomToolFormat{
					Type:       tool.Format.Type,
					Syntax:     tool.Format.Syntax,
					Definition: tool.Format.Definition,
				}
			}

			result = append(result, llm.Tool{
				Type:               llm.ToolTypeResponsesCustomTool,
				ResponseCustomTool: customTool,
			})

		case "namespace":
			// Record the composite-name → {leaf, namespace} mapping so the
			// outbound side can restore the group identity via table lookup
			// (never string splitting — group names may themselves contain "__").
			var nsMap map[string]namespaceToolEntry
			if metadata != nil {
				if existing, ok := metadata[responsesNamespaceToolMapTransformerMetadataKey].(map[string]namespaceToolEntry); ok {
					nsMap = existing
				} else {
					nsMap = make(map[string]namespaceToolEntry)
				}
			}
			for _, subTool := range tool.Tools {
				if subTool.Type != "function" {
					continue
				}
				params, err := json.Marshal(subTool.Parameters)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal namespace tool parameters: %w", err)
				}
				compositeName := tool.Name + "__" + subTool.Name
				descriptionPrefix := tool.Description
				if descriptionPrefix == "" {
					descriptionPrefix = "Tools in the " + tool.Name + " namespace."
				}
				if nsMap != nil {
					nsMap[compositeName] = namespaceToolEntry{
						Leaf:      subTool.Name,
						Namespace: tool.Name,
					}
				}
				result = append(result, llm.Tool{
					Type: "function",
					Function: llm.Function{
						Name:        compositeName,
						Description: descriptionPrefix + "\n\n" + subTool.Description,
						Parameters:  params,
						Strict:      subTool.Strict,
					},
				})
			}
			if metadata != nil && nsMap != nil {
				metadata[responsesNamespaceToolMapTransformerMetadataKey] = nsMap
			}

		default:
			// Non-structural tools (tool_search/mcp/file_search/...) are preserved on
			// ProviderExtensions.OpenAIResponses.Request for same-protocol replay and
			// diagnosed on non-Responses outbounds. Do not invent llm.Tool shapes here.
			continue
		}
	}

	return result, nil
}

func getResponseWebSearchCallsFromMetadata(metadata map[string]any) []Item {
	if len(metadata) == 0 {
		return nil
	}

	raw, ok := metadata[responsesWebSearchCallsTransformerMetadataKey]
	if !ok || raw == nil {
		return nil
	}

	items, ok := raw.([]Item)
	if !ok {
		data, err := json.Marshal(raw)
		if err != nil {
			return nil
		}

		if err := json.Unmarshal(data, &items); err != nil {
			return nil
		}
	}

	result := make([]Item, 0, len(items))
	for _, item := range items {
		if item.Type != "web_search_call" || item.Action == nil || item.Action.WebSearch == nil {
			continue
		}

		src := item.Action.WebSearch
		result = append(result, Item{
			ID:     item.ID,
			Type:   item.Type,
			Status: item.Status,
			Action: NewWebSearchAction(&WebSearchAction{
				Type:    src.Type,
				Query:   src.Query,
				Queries: append([]string(nil), src.Queries...),
				Sources: append([]WebSearchSource(nil), src.Sources...),
			}),
		})
	}

	return result
}

func attachAnnotationsToFirstTextItem(items []Item, annotations []llm.Annotation) ([]Item, bool) {
	if len(items) == 0 || len(annotations) == 0 {
		return items, false
	}

	firstTextItemIdx := -1
	for i := range items {
		switch items[i].Type {
		case "output_text", "input_text", "text":
			firstTextItemIdx = i
		}

		if firstTextItemIdx >= 0 {
			break
		}
	}

	if firstTextItemIdx < 0 {
		return items, false
	}

	items[firstTextItemIdx].Annotations = lo.Map(annotations, func(annotation llm.Annotation, _ int) Annotation {
		result := Annotation{
			Type:       annotation.Type,
			StartIndex: annotation.StartIndex,
			EndIndex:   annotation.EndIndex,
		}

		if annotation.URLCitation != nil {
			result.URLCitation = &URLCitation{
				URL:   annotation.URLCitation.URL,
				Title: annotation.URLCitation.Title,
			}
		}

		return result
	})

	return items, true
}

// convertToResponsesAPIResponse converts llm.Response to Responses API Response.
func convertToResponsesAPIResponse(chatResp *llm.Response) *Response {
	resp := &Response{
		Object:             "response",
		ID:                 chatResp.ID,
		Model:              chatResp.Model,
		CreatedAt:          chatResp.Created,
		Output:             append([]Item(nil), getResponseWebSearchCallsFromMetadata(chatResp.TransformerMetadata)...),
		Status:             lo.ToPtr("completed"),
		PreviousResponseID: chatResp.PreviousResponseID,
	}
	hasNativeNonTerminalStatus := false

	// Backfill service_tier and error so they survive conversion back to
	// the Responses API format (canonical carries both).
	if chatResp.ServiceTier != "" {
		resp.ServiceTier = lo.ToPtr(chatResp.ServiceTier)
	}
	if chatResp.Error != nil {
		resp.Error = &Error{
			Type:    chatResp.Error.Detail.Type,
			Code:    chatResp.Error.Detail.Code,
			Message: chatResp.Error.Detail.Message,
		}
	}
	if chatResp.ProviderExtensions != nil && chatResp.ProviderExtensions.OpenAIResponses != nil &&
		chatResp.ProviderExtensions.OpenAIResponses.Response != nil {
		if nativeStatus := chatResp.ProviderExtensions.OpenAIResponses.Response.Status; nativeStatus != nil {
			resp.Status = lo.ToPtr(*nativeStatus)
			hasNativeNonTerminalStatus = true
		}
		raw := chatResp.ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields["incomplete_details"]
		// Explicit JSON null is restored by the raw allowlist path. Unmarshaling
		// null into a typed struct would invent an empty object instead.
		if len(raw) > 0 && string(raw) != "null" {
			var details ResponseIncompleteDetails
			if err := json.Unmarshal(raw, &details); err == nil {
				resp.IncompleteDetails = &details
			}
		}
	}

	// Convert usage
	resp.Usage = ConvertLLMUsageToResponsesUsage(chatResp.Usage)

	// Convert choices to output items
	for _, choice := range chatResp.Choices {
		var message *llm.Message
		if choice.Message != nil {
			message = choice.Message
		} else if choice.Delta != nil {
			message = choice.Delta
		}

		if message == nil {
			continue
		}

		messageItemID := message.ID
		if messageItemID == "" {
			messageItemID = generateItemID()
		}

		// A canonical message carries one reasoning slot. When the upstream
		// response had multiple native reasoning output items, their complete
		// sequence is replayed through the Responses raw sidecar below; emitting
		// a structured item here would duplicate and reorder it.
		if !hasRawResponsesReasoningOutputItems(chatResp) {
			if reasoningItem, ok := buildReasoningItem(
				*message,
				chatResp.TransformerMetadata,
				isOpenAIResponsesAPIFormat(chatResp.APIFormat),
			); ok {
				resp.Output = append(resp.Output, reasoningItem)
			}
		}

		// Handle tool calls (function calls and custom tool calls)
		if len(message.ToolCalls) > 0 {
			for _, toolCall := range message.ToolCalls {
				if toolCall.ResponseCustomToolCall != nil {
					ctcItemID := toolCall.ResponseItemID
					if ctcItemID == "" {
						// Target envelope construction: never alias item id to call_id.
						ctcItemID = generateItemID()
					}
					ctcStatus := toolCall.Status
					if ctcStatus == "" {
						ctcStatus = "completed"
					}
					resp.Output = append(resp.Output, Item{
						ID:        ctcItemID,
						Type:      "custom_tool_call",
						CallID:    toolCall.ResponseCustomToolCall.CallID,
						Name:      toolCall.ResponseCustomToolCall.Name,
						Namespace: toolCall.ResponseCustomToolCall.Namespace,
						Input:     lo.ToPtr(toolCall.ResponseCustomToolCall.Input),
						Status:    lo.ToPtr(ctcStatus),
					})
				} else if toolCall.OpenAIChatCustomToolCall != nil {
					// Explicit Chat→Responses custom bridge for provider responses.
					ctcItemID := toolCall.ResponseItemID
					if ctcItemID == "" {
						ctcItemID = generateItemID()
					}
					ctcStatus := toolCall.Status
					if ctcStatus == "" {
						ctcStatus = "completed"
					}
					resp.Output = append(resp.Output, Item{
						ID:     ctcItemID,
						Type:   "custom_tool_call",
						CallID: toolCall.ID,
						Name:   toolCall.OpenAIChatCustomToolCall.Name,
						Input:  lo.ToPtr(toolCall.OpenAIChatCustomToolCall.Input),
						Status: lo.ToPtr(ctcStatus),
					})
				} else {
					fcItemID := toolCall.ResponseItemID
					if fcItemID == "" {
						// Target envelope construction: never alias item id to call_id.
						fcItemID = generateItemID()
					}
					fcStatus := toolCall.Status
					if fcStatus == "" {
						fcStatus = "completed"
					}
					// Restore namespace group identity via table lookup (never string
					// splitting — group names may contain "__").
					fcName, fcNamespace := resolveNamespaceFromMetadata(chatResp.TransformerMetadata, toolCall.Function.Name)
					resp.Output = append(resp.Output, Item{
						ID:        fcItemID,
						Type:      "function_call",
						CallID:    toolCall.ID,
						Name:      fcName,
						Namespace: fcNamespace,
						Arguments: toolCall.Function.Arguments,
						Status:    lo.ToPtr(fcStatus),
					})
				}
			}
		}

		// Handle text content and/or refusal content parts.
		// Preserve the historical precedence: scalar Content wins over MultipleContent.
		if message.Content.Content != nil && *message.Content.Content != "" {
			text := *message.Content.Content
			contentItems := []Item{{
				Type:        "output_text",
				Text:        &text,
				Annotations: []Annotation{},
			}}
			if message.Refusal != "" {
				refusal := message.Refusal
				contentItems = append(contentItems, Item{
					Type:    "refusal",
					Refusal: &refusal,
				})
			}
			contentItems, _ = attachAnnotationsToFirstTextItem(contentItems, message.Annotations)
			resp.Output = append(resp.Output, Item{
				ID:   messageItemID,
				Type: "message",
				Role: "assistant",
				Content: &Input{
					Items: contentItems,
				},
				Status: lo.ToPtr("completed"),
			})
		} else if len(message.Content.MultipleContent) > 0 {
			contentItems := make([]Item, 0)

			for _, part := range message.Content.MultipleContent {
				switch part.Type {
				case "text":
					if part.Text != nil {
						text := *part.Text
						contentItems = append(contentItems, Item{
							Type:        "output_text",
							Text:        &text,
							Annotations: []Annotation{},
						})
					}
				case "image_url":
					// Handle image output
					if part.ImageURL != nil {
						imageItem := Item{
							ID:           generateItemID(),
							Type:         "image_generation_call",
							Role:         "assistant",
							Result:       lo.ToPtr(xurl.ExtractBase64FromDataURL(part.ImageURL.URL)),
							Status:       lo.ToPtr("completed"),
							Background:   xmap.GetStringPtr(part.TransformerMetadata, responsesBackgroundTransformerMetadataKey),
							OutputFormat: xmap.GetStringPtr(part.TransformerMetadata, responsesImageGenOutputFormatTransformerMetadataKey),
							Quality:      xmap.GetStringPtr(part.TransformerMetadata, responsesImageGenQualityTransformerMetadataKey),
							Size:         xmap.GetStringPtr(part.TransformerMetadata, responsesImageGenSizeTransformerMetadataKey),
						}
						resp.Output = append(resp.Output, imageItem)
					}
				case "compaction", "compaction_summary":
					if part.Compact != nil {
						resp.Output = append(resp.Output, compactionItemFromPart(part, part.Type))
					}
				}
			}

			if message.Refusal != "" {
				refusal := message.Refusal
				contentItems = append(contentItems, Item{
					Type:    "refusal",
					Refusal: &refusal,
				})
			}
			if len(contentItems) > 0 {
				contentItems, _ = attachAnnotationsToFirstTextItem(contentItems, message.Annotations)
				resp.Output = append(resp.Output, Item{
					ID:      messageItemID,
					Type:    "message",
					Role:    "assistant",
					Content: &Input{Items: contentItems},
					Status:  lo.ToPtr("completed"),
				})
			}
		} else if message.Refusal != "" {
			// Refusal-only assistant message (no text/multiple content).
			refusal := message.Refusal
			resp.Output = append(resp.Output, Item{
				ID:   messageItemID,
				Type: "message",
				Role: "assistant",
				Content: &Input{Items: []Item{{
					Type:    "refusal",
					Refusal: &refusal,
				}}},
				Status: lo.ToPtr("completed"),
			})
		}

		// Set status based on finish reason
		if choice.FinishReason != nil && !hasNativeNonTerminalStatus {
			switch *choice.FinishReason {
			case "stop":
				resp.Status = lo.ToPtr("completed")
			case "length":
				resp.Status = lo.ToPtr("incomplete")
			case "tool_calls":
				resp.Status = lo.ToPtr("completed")
			case "error":
				resp.Status = lo.ToPtr("failed")
			case "cancelled", "canceled":
				resp.Status = lo.ToPtr("canceled")
			}
		}
	}

	// Preserve an intentionally empty output[] when the canonical response has
	// no representable output or carries Responses-native raw output sidecars.
	// A synthetic message is only needed for the legacy case where a canonical
	// choice exists but did not produce an output item.
	if len(resp.Output) == 0 && len(chatResp.Choices) > 0 && lo.FromPtr(resp.Status) == "completed" && !hasRawResponsesOutputItems(chatResp) {
		emptyText := ""
		resp.Output = []Item{
			{
				ID:   generateItemID(),
				Type: "message",
				Role: "assistant",
				Content: &Input{
					Items: []Item{
						{
							Type:        "output_text",
							Text:        &emptyText,
							Annotations: []Annotation{},
						},
					},
				},
				Status: lo.ToPtr("completed"),
			},
		}
	}

	resp.Output = mergeRawResponsesOutputItems(resp.Output, chatResp)

	return resp
}

func isOpenAIResponsesAPIFormat(format llm.APIFormat) bool {
	return format == llm.APIFormatOpenAIResponse || format == llm.APIFormatOpenAIResponseCompact
}

func hasRawResponsesOutputItems(chatResp *llm.Response) bool {
	return chatResp != nil && chatResp.ProviderExtensions != nil &&
		chatResp.ProviderExtensions.OpenAIResponses != nil &&
		chatResp.ProviderExtensions.OpenAIResponses.Response != nil &&
		len(chatResp.ProviderExtensions.OpenAIResponses.Response.RawOutputItems) > 0
}

func hasRawResponsesReasoningOutputItems(chatResp *llm.Response) bool {
	if chatResp == nil || chatResp.ProviderExtensions == nil || chatResp.ProviderExtensions.OpenAIResponses == nil ||
		chatResp.ProviderExtensions.OpenAIResponses.Response == nil {
		return false
	}
	return lo.SomeBy(chatResp.ProviderExtensions.OpenAIResponses.Response.RawOutputItems, func(fragment llm.OpenAIResponsesRawFragment) bool {
		return fragment.Type == "reasoning"
	})
}

func mergeRawResponsesOutputItems(structured []Item, chatResp *llm.Response) []Item {
	if chatResp == nil || chatResp.ProviderExtensions == nil || chatResp.ProviderExtensions.OpenAIResponses == nil ||
		chatResp.ProviderExtensions.OpenAIResponses.Response == nil {
		return structured
	}
	rawFragments := chatResp.ProviderExtensions.OpenAIResponses.Response.RawOutputItems
	if len(rawFragments) == 0 {
		return structured
	}

	rawByIndex := make(map[int]json.RawMessage, len(rawFragments))
	maxIndex := len(structured) + len(rawFragments) - 1
	for _, fragment := range rawFragments {
		if len(fragment.Raw) == 0 {
			continue
		}
		rawByIndex[fragment.OriginalIndex] = fragment.Raw
		if fragment.OriginalIndex > maxIndex {
			maxIndex = fragment.OriginalIndex
		}
	}
	if len(rawByIndex) == 0 {
		return structured
	}

	merged := make([]Item, 0, maxIndex+1)
	structuredIndex := 0
	for index := 0; index <= maxIndex; index++ {
		if raw, ok := rawByIndex[index]; ok {
			var item Item
			if err := json.Unmarshal(raw, &item); err == nil {
				item.Raw = append(json.RawMessage(nil), raw...)
				merged = append(merged, item)
			}
			continue
		}
		if structuredIndex < len(structured) {
			merged = append(merged, structured[structuredIndex])
			structuredIndex++
		}
	}
	for structuredIndex < len(structured) {
		merged = append(merged, structured[structuredIndex])
		structuredIndex++
	}
	return merged
}

// generateItemID generates a unique item ID for output items.
func generateItemID() string {
	return fmt.Sprintf("item_%s", lo.RandomString(16, lo.AlphanumericCharset))
}

// buildReasoningItem creates a reasoning Item from a message's reasoning content and signature.
// Returns the item and true if the message has reasoning data, otherwise returns zero value and false.
// When response metadata carries original reasoning_text content[] / summary[], re-emit those shapes.
//
// Encrypted content is only emitted when preserveEncryptedContent is true AND Responses-native
// reasoning-item provenance supplies a non-empty original item id. Ciphertext is never paired
// with a synthesized rs_* id (that recreates item_id/ciphertext mismatch on conversation replay).
func buildReasoningItem(
	msg llm.Message,
	responseMetadata map[string]any,
	preserveEncryptedContent bool,
) (Item, bool) {
	hasContent := msg.ReasoningContent != nil && *msg.ReasoningContent != ""
	rawTextParts := reasoningTextContentFromMetadata(responseMetadata)
	savedSummary := reasoningSummaryFromMetadata(responseMetadata)

	nativeItem, hasNativeItemID := getResponsesReasoningItemMetadata(responseMetadata)
	itemID := ""
	if hasNativeItemID {
		itemID = nativeItem.ID
	}

	var encryptedContent *string
	if preserveEncryptedContent && hasNativeItemID && itemID != "" &&
		msg.ReasoningSignature != nil && *msg.ReasoningSignature != "" {
		encryptedContent = msg.ReasoningSignature
	}

	if !hasContent && encryptedContent == nil && len(rawTextParts) == 0 && len(savedSummary) == 0 {
		return Item{}, false
	}

	summary := []ReasoningSummary{}
	if len(savedSummary) > 0 {
		summary = savedSummary
	} else if hasContent && len(rawTextParts) == 0 {
		// Common path only had opaque reasoning text; emit as summary_text for
		// backward compatibility with existing stream/non-stream consumers.
		summary = append(summary, ReasoningSummary{
			Type: "summary_text",
			Text: *msg.ReasoningContent,
		})
	}

	if itemID == "" {
		// Summary/thinking-only paths may allocate a local envelope id. Ciphertext
		// never reaches this branch without an original Responses item id.
		// This branch uses unprefixed generateItemID().
		itemID = generateItemID()
	}

	item := Item{
		ID:               itemID,
		Type:             "reasoning",
		Status:           lo.ToPtr("completed"),
		Summary:          summary,
		EncryptedContent: encryptedContent,
	}
	// Only emit content[]/reasoning_text when the original Responses item had it.
	// Common-only ReasoningContent continues to use summary_text for backward
	// compatibility with existing clients/tests.
	if len(rawTextParts) > 0 {
		item.ReasoningContent = rawTextParts
	}
	return item, true
}

func reasoningTextContentFromMetadata(meta map[string]any) []ReasoningContent {
	if meta == nil {
		return nil
	}
	raw, ok := meta[responsesReasoningTextContentTransformerMetadataKey]
	if !ok || raw == nil {
		return nil
	}
	if parts, ok := raw.([]ReasoningContent); ok {
		return append([]ReasoningContent(nil), parts...)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var parts []ReasoningContent
	if err := json.Unmarshal(data, &parts); err != nil {
		return nil
	}
	return parts
}

func reasoningSummaryFromMetadata(meta map[string]any) []ReasoningSummary {
	if meta == nil {
		return nil
	}
	raw, ok := meta[responsesReasoningSummaryContentTransformerMetadataKey]
	if !ok || raw == nil {
		return nil
	}
	if parts, ok := raw.([]ReasoningSummary); ok {
		return append([]ReasoningSummary(nil), parts...)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var parts []ReasoningSummary
	if err := json.Unmarshal(data, &parts); err != nil {
		return nil
	}
	return parts
}
