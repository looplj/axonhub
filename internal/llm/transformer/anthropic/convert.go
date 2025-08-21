package anthropic

import (
	"encoding/json"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
)

// convertToAnthropicRequest converts ChatCompletionRequest to Anthropic MessageRequest.
func convertToAnthropicRequest(chatReq *llm.Request) *MessageRequest {
	req := &MessageRequest{
		Model:       chatReq.Model,
		Temperature: chatReq.Temperature,
		TopP:        chatReq.TopP,
		Stream:      chatReq.Stream,
	}

	// Set max_tokens (required for Anthropic)
	if chatReq.MaxTokens != nil {
		req.MaxTokens = *chatReq.MaxTokens
	} else if chatReq.MaxCompletionTokens != nil {
		req.MaxTokens = *chatReq.MaxCompletionTokens
	} else {
		// TODO: add a way to configure default max_tokens
		req.MaxTokens = 4096
	}

	// Convert tools if present
	if len(chatReq.Tools) > 0 {
		tools := make([]Tool, 0, len(chatReq.Tools))
		for _, tool := range chatReq.Tools {
			if tool.Type == "function" {
				anthropicTool := Tool{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					InputSchema: tool.Function.Parameters,
				}
				tools = append(tools, anthropicTool)
			}
		}

		req.Tools = tools
	}

	// Convert messages
	messages := make([]MessageParam, 0, len(chatReq.Messages))

	for _, msg := range chatReq.Messages {
		// Handle system messages separately
		if msg.Role == "system" {
			if msg.Content.Content != nil {
				req.System = &SystemPrompt{
					Prompt: msg.Content.Content,
				}
			}

			continue
		}

		anthropicMsg := MessageParam{
			Role: msg.Role,
		}

		// Convert content
		if msg.Content.Content != nil {
			anthropicMsg.Content = MessageContent{
				Content: msg.Content.Content,
			}
		} else if len(msg.Content.MultipleContent) > 0 {
			content, ok := convertMultiplePartContent(msg)
			if ok {
				anthropicMsg.Content = content
			}
		}

		messages = append(messages, anthropicMsg)
	}

	req.Messages = messages

	// Convert stop sequences
	if chatReq.Stop != nil {
		if chatReq.Stop.Stop != nil {
			req.StopSequences = []string{*chatReq.Stop.Stop}
		} else if len(chatReq.Stop.MultipleStop) > 0 {
			req.StopSequences = chatReq.Stop.MultipleStop
		}
	}

	return req
}

func convertMultiplePartContent(msg llm.Message) (MessageContent, bool) {
	blocks := make([]ContentBlock, 0, len(msg.Content.MultipleContent))
	for _, part := range msg.Content.MultipleContent {
		switch part.Type {
		case "text":
			if part.Text != nil {
				blocks = append(blocks, ContentBlock{
					Type: "text",
					Text: *part.Text,
				})
			}
		case "image_url":
			if part.ImageURL != nil && part.ImageURL.URL != "" {
				// Convert OpenAI image format to Anthropic format
				// Extract media type and data from data URL
				url := part.ImageURL.URL
				if strings.HasPrefix(url, "data:") {
					parts := strings.SplitN(url, ",", 2)
					if len(parts) == 2 {
						headerParts := strings.Split(parts[0], ";")
						if len(headerParts) >= 2 {
							mediaType := strings.TrimPrefix(headerParts[0], "data:")
							blocks = append(blocks, ContentBlock{
								Type: "image",
								Source: &ImageSource{
									Type:      "base64",
									MediaType: mediaType,
									Data:      parts[1],
								},
							})
						}
					}
				} else {
					blocks = append(blocks, ContentBlock{
						Type: "image",
						Source: &ImageSource{
							Type: "url",
							URL:  part.ImageURL.URL,
						},
					})
				}
			}
		}
	}

	if len(blocks) == 0 {
		return MessageContent{}, false
	}

	return MessageContent{
		MultipleContent: blocks,
	}, true
}

// convertUsage converts Anthropic Usage to unified Usage format.
func convertUsage(usage Usage) llm.Usage {
	u := llm.Usage{
		PromptTokens:     int(usage.InputTokens),
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens: int(
			usage.InputTokens + usage.OutputTokens,
		),
	}

	// Map detailed token information from Anthropic format to unified model
	if usage.CacheReadInputTokens > 0 {
		u.PromptTokensDetails = &llm.PromptTokensDetails{
			CachedTokens: int(usage.CacheReadInputTokens),
		}
	}

	return u
}

// convertToChatCompletionResponse converts Anthropic Message to unified Response format.
func convertToChatCompletionResponse(anthropicResp *Message) *llm.Response {
	if anthropicResp == nil {
		return &llm.Response{
			ID:      "",
			Object:  "chat.completion",
			Model:   "",
			Created: 0,
		}
	}

	resp := &llm.Response{
		ID:      anthropicResp.ID,
		Object:  "chat.completion",
		Model:   anthropicResp.Model,
		Created: 0, // Anthropic doesn't provide created timestamp
	}

	// Convert content to message
	var (
		content   llm.MessageContent
		toolCalls []llm.ToolCall
		textParts []string
	)

	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
				content.MultipleContent = append(content.MultipleContent, llm.MessageContentPart{
					Type:     "text",
					Text:     &block.Text,
					ImageURL: &llm.ImageURL{},
				})
			}
		case "image":
			if block.Source != nil {
				content.MultipleContent = append(content.MultipleContent, llm.MessageContentPart{
					Type: "image",
					ImageURL: &llm.ImageURL{
						URL:    block.Source.Data,
						Detail: "",
					},
				})
			}
		case "tool_use":
			if block.ID != "" && block.Name != nil {
				toolCall := llm.ToolCall{
					ID:   block.ID,
					Type: "function",
					Function: llm.FunctionCall{
						Name:      *block.Name,
						Arguments: string(block.Input),
					},
				}
				toolCalls = append(toolCalls, toolCall)
			}
		case "thinking":
			if block.Thinking != "" {
				// Add thinking content as a text part but don't include in textParts
				// to preserve it as a separate content block
				thinkingText := block.Thinking
				content.MultipleContent = append(content.MultipleContent, llm.MessageContentPart{
					Type:     "text",
					Text:     &thinkingText,
					ImageURL: &llm.ImageURL{},
				})
			}
		}
	}

	// If we only have text content and no other types, set Content.Content
	if len(textParts) > 0 && len(content.MultipleContent) == len(textParts) {
		// Join all text parts
		var allText string
		for _, text := range textParts {
			allText += text
		}

		content.Content = &allText
		// Clear MultipleContent since we're using the simple string format
		content.MultipleContent = nil
	}

	message := &llm.Message{
		Role:      anthropicResp.Role,
		Content:   content,
		ToolCalls: toolCalls,
	}

	// Convert finish reason
	var finishReason *string

	if anthropicResp.StopReason != nil {
		switch *anthropicResp.StopReason {
		case "end_turn":
			reason := "stop"
			finishReason = &reason
		case "max_tokens":
			reason := "length"
			finishReason = &reason
		case "stop_sequence":
			reason := "stop"
			finishReason = &reason
		case "tool_use":
			reason := "tool_calls"
			finishReason = &reason
		default:
			finishReason = anthropicResp.StopReason
		}
	}

	choice := llm.Choice{
		Index:        0,
		Message:      message,
		FinishReason: finishReason,
	}

	resp.Choices = []llm.Choice{choice}

	// Convert usage
	if anthropicResp.Usage != nil {
		usage := &llm.Usage{
			PromptTokens:     int(anthropicResp.Usage.InputTokens),
			CompletionTokens: int(anthropicResp.Usage.OutputTokens),
			TotalTokens: int(
				anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
			),
		}

		// Map detailed token information from Anthropic format to unified model
		if anthropicResp.Usage.CacheReadInputTokens > 0 {
			usage.PromptTokensDetails = &llm.PromptTokensDetails{
				CachedTokens: int(anthropicResp.Usage.CacheReadInputTokens),
			}
		}

		// Note: Anthropic doesn't currently provide reasoning tokens breakdown
		// but we can add it in the future if they support it
		usage.CompletionTokensDetails = &llm.CompletionTokensDetails{
			ReasoningTokens: 0, // Anthropic doesn't provide this yet
		}

		resp.Usage = usage
	}

	return resp
}

func convertToAnthropicResponse(chatResp *llm.Response) *Message {
	resp := &Message{
		ID:    chatResp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: chatResp.Model,
	}

	// Convert choices to content blocks
	if len(chatResp.Choices) > 0 {
		choice := chatResp.Choices[0]

		var message *llm.Message

		if choice.Message != nil {
			message = choice.Message
		} else if choice.Delta != nil {
			message = choice.Delta
		}

		if message != nil {
			var contentBlocks []ContentBlock

			// Handle reasoning content (thinking) first if present
			if message.ReasoningContent != nil && *message.ReasoningContent != "" {
				contentBlocks = append(contentBlocks, ContentBlock{
					Type:     "thinking",
					Thinking: *message.ReasoningContent,
				})
			}

			// Handle regular content
			if message.Content.Content != nil {
				contentBlocks = append(contentBlocks, ContentBlock{
					Type: "text",
					Text: *message.Content.Content,
				})
			} else if len(message.Content.MultipleContent) > 0 {
				for _, part := range message.Content.MultipleContent {
					switch part.Type {
					case "text":
						if part.Text != nil {
							contentBlocks = append(contentBlocks, ContentBlock{
								Type: "text",
								Text: *part.Text,
							})
						}
					case "image_url":
						if part.ImageURL != nil && part.ImageURL.URL != "" {
							// Convert OpenAI image format to Anthropic format
							url := part.ImageURL.URL
							if strings.HasPrefix(url, "data:") {
								// Extract media type and data from data URL
								parts := strings.SplitN(url, ",", 2)
								if len(parts) == 2 {
									headerParts := strings.Split(parts[0], ";")
									if len(headerParts) >= 2 {
										mediaType := strings.TrimPrefix(headerParts[0], "data:")
										contentBlocks = append(contentBlocks, ContentBlock{
											Type: "image",
											Source: &ImageSource{
												Type:      "base64",
												MediaType: mediaType,
												Data:      parts[1],
											},
										})
									}
								}
							} else {
								contentBlocks = append(contentBlocks, ContentBlock{
									Type: "image",
									Source: &ImageSource{
										Type: "url",
										URL:  part.ImageURL.URL,
									},
								})
							}
						}
					}
				}
			}

			// Handle tool calls
			if len(message.ToolCalls) > 0 {
				for _, toolCall := range message.ToolCalls {
					var input json.RawMessage

					if toolCall.Function.Arguments != "" {
						// Validate JSON before using it as RawMessage
						var temp interface{}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &temp); err != nil {
							// If invalid JSON, wrap it in a string field
							escapedArgs, _ := json.Marshal(toolCall.Function.Arguments)
							input = json.RawMessage(`{"raw_arguments": ` + string(escapedArgs) + `}`)
						} else {
							input = json.RawMessage(toolCall.Function.Arguments)
						}
					} else {
						input = json.RawMessage("{}")
					}

					contentBlocks = append(contentBlocks, ContentBlock{
						Type:  "tool_use",
						ID:    toolCall.ID,
						Name:  &toolCall.Function.Name,
						Input: input,
					})
				}
			}

			resp.Content = contentBlocks
		}

		// Convert finish reason
		if choice.FinishReason != nil {
			switch *choice.FinishReason {
			case "stop":
				stopReason := "end_turn"
				resp.StopReason = &stopReason
			case "length":
				stopReason := "max_tokens"
				resp.StopReason = &stopReason
			case "tool_calls":
				stopReason := "tool_use"
				resp.StopReason = &stopReason
			default:
				resp.StopReason = choice.FinishReason
			}
		}
	}

	// Convert usage
	if chatResp.Usage != nil {
		usage := &Usage{
			InputTokens:  int64(chatResp.Usage.PromptTokens),
			OutputTokens: int64(chatResp.Usage.CompletionTokens),
		}

		// Map detailed token information from unified model to Anthropic format
		if chatResp.Usage.PromptTokensDetails != nil {
			usage.CacheReadInputTokens = int64(chatResp.Usage.PromptTokensDetails.CachedTokens)
		}

		// Note: Anthropic doesn't have a direct equivalent for reasoning tokens in their current API
		// but we can store it in cache_creation_input_tokens as a workaround if needed
		if chatResp.Usage.CompletionTokensDetails != nil {
			// For now, we don't map reasoning tokens as Anthropic doesn't have a direct field
			// This could be extended in the future if Anthropic adds support
		}

		resp.Usage = usage
	}

	return resp
}
