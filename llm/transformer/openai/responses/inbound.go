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

	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/internal/pkg/xmap"
	"github.com/looplj/axonhub/internal/pkg/xurl"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

var _ transformer.Inbound = (*InboundTransformer)(nil)

// InboundTransformer implements transformer.Inbound for OpenAI Responses API format.
type InboundTransformer struct{}

// NewInboundTransformer creates a new OpenAI Responses InboundTransformer.
func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

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

	return convertToLLMRequest(&req)
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

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
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

	if llmErr, ok := xerrors.As[*llm.ResponseError](rawErr); ok {
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

	if httpErr, ok := xerrors.As[*httpclient.Error](rawErr); ok {
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
func convertToLLMRequest(req *Request) (*llm.Request, error) {
	chatReq := &llm.Request{
		Model:               req.Model,
		Temperature:         req.Temperature,
		Stream:              req.Stream,
		Metadata:            maps.Clone(req.Metadata),
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
		TransformerMetadata: map[string]any{},
		TransformOptions:    llm.TransformOptions{},
	}

	// Store help fields in TransformerMetadata
	if len(req.Include) > 0 {
		chatReq.TransformerMetadata["include"] = req.Include
	}

	if req.MaxToolCalls != nil {
		chatReq.TransformerMetadata["max_tool_calls"] = req.MaxToolCalls
	}

	if req.PromptCacheRetention != nil {
		chatReq.TransformerMetadata["prompt_cache_retention"] = req.PromptCacheRetention
	}

	if req.Truncation != nil {
		chatReq.TransformerMetadata["truncation"] = req.Truncation
	}

	// Convert reasoning
	if req.Reasoning != nil {
		if req.Reasoning.Effort != "" {
			chatReq.ReasoningEffort = req.Reasoning.Effort
		}

		if req.Reasoning.MaxTokens != nil {
			chatReq.ReasoningBudget = req.Reasoning.MaxTokens
		}

		// Priority: summary > generate_summary
		if req.Reasoning.Summary != "" {
			chatReq.ReasoningSummary = lo.ToPtr(req.Reasoning.Summary)
		} else if req.Reasoning.GenerateSummary != "" {
			chatReq.ReasoningSummary = lo.ToPtr(req.Reasoning.GenerateSummary)
		}
	}

	// Convert tool choice
	if req.ToolChoice != nil {
		chatReq.ToolChoice = convertToolChoiceToLLM(req.ToolChoice)
	}

	// Convert stream options
	if req.StreamOptions != nil {
		chatReq.StreamOptions = &llm.StreamOptions{}
		if req.StreamOptions.IncludeObfuscation != nil {
			chatReq.TransformerMetadata["include_obfuscation"] = req.StreamOptions.IncludeObfuscation
		}
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

	// Convert tools
	if len(req.Tools) > 0 {
		tools, err := convertToolsToLLM(req.Tools)
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
	}

	// Convert text verbosity
	if req.Text != nil {
		chatReq.Verbosity = req.Text.Verbosity
	}

	return chatReq, nil
}

// convertToolChoiceToLLM converts Responses API ToolChoice to llm.ToolChoice.
func convertToolChoiceToLLM(src *ToolChoice) *llm.ToolChoice {
	if src == nil {
		return nil
	}

	result := &llm.ToolChoice{}

	if src.Mode != nil {
		result.ToolChoice = src.Mode
	} else if src.Type != nil && src.Name != nil {
		result.NamedToolChoice = &llm.NamedToolChoice{
			Type: *src.Type,
			Function: llm.ToolFunction{
				Name: *src.Name,
			},
		}
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

// convertReasoningWithFollowing converts a reasoning item and merges it with subsequent
// function_call items or text content into a single assistant message.
// Returns the merged message and the number of items consumed.
func convertReasoningWithFollowing(items []Item, startIdx int) (*llm.Message, int, error) {
	if startIdx >= len(items) || items[startIdx].Type != "reasoning" {
		return nil, 0, nil
	}

	reasoningItem := &items[startIdx]
	msg := &llm.Message{
		Role: "assistant",
	}

	// Extract reasoning content
	var reasoningText strings.Builder

	for _, summary := range reasoningItem.Summary {
		reasoningText.WriteString(summary.Text)
	}

	if reasoningText.Len() > 0 {
		msg.ReasoningContent = lo.ToPtr(reasoningText.String())
	}

	if reasoningItem.EncryptedContent != nil && *reasoningItem.EncryptedContent != "" {
		msg.ReasoningSignature = reasoningItem.EncryptedContent
	}

	consumed := 1

	// Look ahead for subsequent function_call items to merge
	for i := startIdx + 1; i < len(items); i++ {
		nextItem := &items[i]

		switch nextItem.Type {
		case "function_call":
			// Merge function_call into the same assistant message
			msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{
				ID:   nextItem.CallID,
				Type: "function",
				Function: llm.FunctionCall{
					Name:      nextItem.Name,
					Arguments: nextItem.Arguments,
				},
			})
			consumed++

		case "message", "input_text", "":
			// If we encounter a text message with assistant role, merge its content
			if nextItem.Role == "assistant" {
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

	case "function_call":
		// Function call from assistant - convert to tool call
		return &llm.Message{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{
					ID:   item.CallID,
					Type: "function",
					Function: llm.FunctionCall{
						Name:      item.Name,
						Arguments: item.Arguments,
					},
				},
			},
		}, nil

	case "function_call_output":
		// Function call output - convert to tool message
		return &llm.Message{
			Role:       "tool",
			ToolCallID: lo.ToPtr(item.CallID),
			Content:    convertToMessageContent(*item.Output),
		}, nil

	case "reasoning":
		// Reasoning is handled by convertReasoningWithFollowing in convertInputToMessages
		// This case should not be reached in normal flow, but return nil to skip if it does
		return nil, nil

	default:
		// Skip unknown types
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
				Type: "text",
				Text: item.Text,
			}, nil
		}

		return nil, nil

	case "input_image":
		if item.ImageURL != nil {
			return &llm.MessageContentPart{
				Type: "image_url",
				ImageURL: &llm.ImageURL{
					URL:    *item.ImageURL,
					Detail: item.Detail,
				},
			}, nil
		}

		return nil, nil

	default:
		return nil, nil
	}
}

// convertToolsToLLM converts Responses API tools to llm.Tool slice.
func convertToolsToLLM(tools []Tool) ([]llm.Tool, error) {
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
					Background:        tool.Background,
					InputFidelity:     tool.InputFidelity,
					Moderation:        tool.Moderation,
					OutputCompression: tool.OutputCompression,
					OutputFormat:      tool.OutputFormat,
					PartialImages:     tool.PartialImages,
					Quality:           tool.Quality,
					Size:              tool.Size,
				},
			})

		default:
			// Skip unsupported tool types
			continue
		}
	}

	return result, nil
}

// convertToResponsesAPIResponse converts llm.Response to Responses API Response.
func convertToResponsesAPIResponse(chatResp *llm.Response) *Response {
	resp := &Response{
		Object:    "response",
		ID:        chatResp.ID,
		Model:     chatResp.Model,
		CreatedAt: chatResp.Created,
		Output:    make([]Item, 0),
		Status:    lo.ToPtr("completed"),
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

		// Handle reasoning content
		if message.ReasoningContent != nil && *message.ReasoningContent != "" {
			resp.Output = append(resp.Output, Item{
				ID:     generateItemID(),
				Type:   "reasoning",
				Status: lo.ToPtr("completed"),
				Summary: []ReasoningSummary{
					{
						Type: "summary_text",
						Text: *message.ReasoningContent,
					},
				},
				EncryptedContent: message.ReasoningSignature,
			})
		}

		// Handle tool calls (function calls)
		if len(message.ToolCalls) > 0 {
			for _, toolCall := range message.ToolCalls {
				resp.Output = append(resp.Output, Item{
					ID:        toolCall.ID,
					Type:      "function_call",
					CallID:    toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
					Status:    lo.ToPtr("completed"),
				})
			}
		}

		// Handle text content
		if message.Content.Content != nil && *message.Content.Content != "" {
			text := *message.Content.Content
			resp.Output = append(resp.Output, Item{
				ID:   generateItemID(),
				Type: "message",
				Role: "assistant",
				Content: &Input{
					Items: []Item{
						{
							Type:        "output_text",
							Text:        &text,
							Annotations: []Annotation{},
						},
					},
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
							Result:       lo.ToPtr(extractBase64FromDataURL(part.ImageURL.URL)),
							Status:       lo.ToPtr("completed"),
							Background:   xmap.GetStringPtr(part.TransformerMetadata, "background"),
							OutputFormat: xmap.GetStringPtr(part.TransformerMetadata, "output_format"),
							Quality:      xmap.GetStringPtr(part.TransformerMetadata, "quality"),
							Size:         xmap.GetStringPtr(part.TransformerMetadata, "size"),
						}
						resp.Output = append(resp.Output, imageItem)
					}
				}
			}

			if len(contentItems) > 0 {
				resp.Output = append(resp.Output, Item{
					ID:      generateItemID(),
					Type:    "message",
					Role:    "assistant",
					Content: &Input{Items: contentItems},
					Status:  lo.ToPtr("completed"),
				})
			}
		}

		// Set status based on finish reason
		if choice.FinishReason != nil {
			switch *choice.FinishReason {
			case "stop":
				resp.Status = lo.ToPtr("completed")
			case "length":
				resp.Status = lo.ToPtr("incomplete")
			case "tool_calls":
				resp.Status = lo.ToPtr("completed")
			case "error":
				resp.Status = lo.ToPtr("failed")
			}
		}
	}

	// If no output items were created, create an empty message
	if len(resp.Output) == 0 {
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

	return resp
}

// generateItemID generates a unique item ID for output items.
func generateItemID() string {
	return fmt.Sprintf("item_%s", lo.RandomString(16, lo.AlphanumericCharset))
}

// extractBase64FromDataURL extracts base64 data from a data URL.
func extractBase64FromDataURL(url string) string {
	return xurl.ExtractBase64FromDataURL(url)
}
