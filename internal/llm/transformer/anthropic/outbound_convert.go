package anthropic

import (
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
)

// convertToAnthropicRequest converts ChatCompletionRequest to Anthropic MessageRequest.
// Deprecated: Use convertToAnthropicRequestWithConfig instead.
func convertToAnthropicRequest(chatReq *llm.Request) *MessageRequest {
	return convertToAnthropicRequestWithConfig(chatReq, nil)
}

// convertToAnthropicRequestWithConfig converts ChatCompletionRequest to Anthropic MessageRequest with config.
func convertToAnthropicRequestWithConfig(chatReq *llm.Request, config *Config) *MessageRequest {
	req := &MessageRequest{
		Model:       chatReq.Model,
		Temperature: chatReq.Temperature,
		TopP:        chatReq.TopP,
		Stream:      chatReq.Stream,
		System:      convertAoAnthropicSystemPrompt(chatReq),
	}
	if chatReq.Metadata != nil {
		if chatReq.Metadata["user_id"] != "" {
			req.Metadata = &AnthropicMetadata{
				UserID: chatReq.Metadata["user_id"],
			}
		}
	}

	// Convert ReasoningEffort to Thinking if present
	if chatReq.ReasoningEffort != "" {
		req.Thinking = &Thinking{
			Type:         "enabled",
			BudgetTokens: getThinkingBudgetTokensWithConfig(chatReq.ReasoningEffort, config),
		}
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

	processedToolMessageIndexes := make(map[int]bool)

	for _, msg := range chatReq.Messages {
		// Handle system messages separately
		if msg.Role == "system" {
			continue
		}

		if msg.Role == "tool" {
			// Simple tool call.
			if msg.MessageIndex == nil {
				messages = append(messages, MessageParam{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{
								Type:      "tool_result",
								ToolUseID: msg.ToolCallID,
								Content: &MessageContent{
									Content: msg.Content.Content,
								},
							},
						},
					},
				})
			} else {
				// Complex tool call.
				if processedToolMessageIndexes[*msg.MessageIndex] {
					continue
				}

				toolMsgs := lo.Filter(chatReq.Messages, func(item llm.Message, _ int) bool {
					return item.MessageIndex != nil && *item.MessageIndex == *msg.MessageIndex
				})
				if len(toolMsgs) == 0 {
					continue
				}

				messages = append(messages, MessageParam{
					Role: "user",
					Content: MessageContent{
						MultipleContent: lo.Map(toolMsgs, func(item llm.Message, _ int) MessageContentBlock {
							return MessageContentBlock{
								Type:      "tool_result",
								ToolUseID: item.ToolCallID,
								Content: &MessageContent{
									Content: item.Content.Content,
								},
								IsError: item.ToolCallIsError,
							}
						}),
					},
				})
				processedToolMessageIndexes[*msg.MessageIndex] = true
			}

			continue
		}

		anthropicMsg := MessageParam{
			Role: lo.Ternary(msg.Role == "assistant", "assistant", "user"),
		}

		if len(msg.ToolCalls) > 0 {
			var contextBlock *MessageContentBlock
			if msg.Content.Content != nil {
				contextBlock = &MessageContentBlock{
					Type: "text",
					Text: *msg.Content.Content,
				}
			}

			content, _ := convertMultiplePartContent(msg)
			if contextBlock != nil {
				content.MultipleContent = append([]MessageContentBlock{*contextBlock}, content.MultipleContent...)
			}

			anthropicMsg.Content = content
			messages = append(messages, anthropicMsg)
		} else {
			if msg.Content.Content != nil {
				anthropicMsg.Content = MessageContent{
					Content: msg.Content.Content,
				}
				messages = append(messages, anthropicMsg)
			} else if len(msg.Content.MultipleContent) > 0 {
				content, ok := convertMultiplePartContent(msg)
				if ok {
					anthropicMsg.Content = content
					messages = append(messages, anthropicMsg)
				}
			}
		}
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

func convertAoAnthropicSystemPrompt(chatReq *llm.Request) *SystemPrompt {
	systemMessages := lo.Filter(chatReq.Messages, func(msg llm.Message, _ int) bool {
		return msg.Role == "system"
	})

	switch len(systemMessages) {
	case 0:
		// Leave System as nil when there are no system messages
		return nil
	case 1:
		return &SystemPrompt{
			Prompt: systemMessages[0].Content.Content,
		}
	default:
		return &SystemPrompt{
			MultiplePrompts: lo.Map(systemMessages, func(msg llm.Message, _ int) SystemPromptPart {
				return SystemPromptPart{
					Type: "text",
					Text: *msg.Content.Content,
					CacheControl: &CacheControl{
						Type: "ephemeral",
					},
				}
			}),
		}
	}
}

func convertMultiplePartContent(msg llm.Message) (MessageContent, bool) {
	blocks := make([]MessageContentBlock, 0, len(msg.Content.MultipleContent))
	for _, part := range msg.Content.MultipleContent {
		switch part.Type {
		case "text":
			if part.Text != nil {
				blocks = append(blocks, MessageContentBlock{
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
							blocks = append(blocks, MessageContentBlock{
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
					blocks = append(blocks, MessageContentBlock{
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

	for _, toolCall := range msg.ToolCalls {
		blocks = append(blocks, MessageContentBlock{
			Type:  "tool_use",
			ID:    toolCall.ID,
			Name:  &toolCall.Function.Name,
			Input: []byte(toolCall.Function.Arguments),
			CacheControl: &CacheControl{
				Type: "ephemeral",
			},
		})
	}

	if len(blocks) == 0 {
		return MessageContent{}, false
	}

	return MessageContent{
		MultipleContent: blocks,
	}, true
}

// convertToLlmUsage converts Anthropic Usage to unified Usage format.
func convertToLlmUsage(usage Usage) llm.Usage {
	// For some channel, like deepseek anthropic endpoint, the input tokens is greater than cache read input tokens, not same with anthropic official.
	// I guess the input tokens include the cached tokens, so we handle it here.
	if usage.CacheReadInputTokens > usage.InputTokens {
		usage.InputTokens = usage.CacheReadInputTokens + usage.InputTokens
	}

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

	choice := llm.Choice{
		Index:        0,
		Message:      message,
		FinishReason: convertFinishReason(anthropicResp.StopReason),
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

func convertFinishReason(stopReason *string) *string {
	if stopReason == nil {
		return nil
	}

	switch *stopReason {
	case "end_turn":
		return lo.ToPtr("stop")
	case "max_tokens":
		return lo.ToPtr("length")
	case "stop_sequence", "pause_turn":
		return lo.ToPtr("stop")
	case "tool_use":
		return lo.ToPtr("tool_calls")
	case "refusal":
		return lo.ToPtr("content_filter")
	default:
		return stopReason
	}
}
