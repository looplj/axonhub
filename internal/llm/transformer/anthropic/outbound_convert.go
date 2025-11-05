package anthropic

import (
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/xjson"
)

// convertToAnthropicRequest converts ChatCompletionRequest to Anthropic MessageRequest.
// Deprecated: Use convertToAnthropicRequestWithConfig instead.
func convertToAnthropicRequest(chatReq *llm.Request) *MessageRequest {
	return convertToAnthropicRequestWithConfig(chatReq, nil)
}

func convertCacheControlToAnthropic(cacheControl *llm.CacheControl) *CacheControl {
	if cacheControl == nil {
		return nil
	}

	return &CacheControl{
		Type: cacheControl.Type,
		TTL:  cacheControl.TTL,
	}
}

// convertToAnthropicRequestWithConfig converts ChatCompletionRequest to Anthropic MessageRequest with config.
//
//nolint:maintidx // TODO: fix.
func convertToAnthropicRequestWithConfig(chatReq *llm.Request, config *Config) *MessageRequest {
	req := &MessageRequest{
		Model:       chatReq.Model,
		Temperature: chatReq.Temperature,
		TopP:        chatReq.TopP,
		Stream:      chatReq.Stream,
		System:      convertToAnthropicSystemPrompt(chatReq),
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
					Name:         tool.Function.Name,
					Description:  tool.Function.Description,
					InputSchema:  tool.Function.Parameters,
					CacheControl: convertCacheControlToAnthropic(tool.CacheControl),
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
			// One tool call.
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
								CacheControl: convertCacheControlToAnthropic(msg.CacheControl),
							},
						},
					},
				})
			} else {
				// Multiple tool calls.
				// DeepSeek Anthropic require the request content be same with the response content, so we need to handle it.
				if processedToolMessageIndexes[*msg.MessageIndex] {
					continue
				}

				toolMsgs := lo.Filter(chatReq.Messages, func(item llm.Message, _ int) bool {
					return item.Role == "tool" && item.MessageIndex != nil && *item.MessageIndex == *msg.MessageIndex
				})
				if len(toolMsgs) == 0 {
					continue
				}

				// Build tool_result blocks
				contentBlocks := lo.Map(toolMsgs, func(item llm.Message, _ int) MessageContentBlock {
					var toolResultContent *MessageContent
					if item.Content.Content != nil {
						// String content - keep as string
						toolResultContent = &MessageContent{
							Content: item.Content.Content,
						}
					} else if len(item.Content.MultipleContent) > 0 {
						// MultipleContent format - convert to Anthropic format
						toolResultContent = &MessageContent{
							MultipleContent: lo.Map(item.Content.MultipleContent, func(part llm.MessageContentPart, _ int) MessageContentBlock {
								return MessageContentBlock{
									Type: part.Type,
									Text: lo.FromPtrOr(part.Text, ""),
								}
							}),
						}
					}

					return MessageContentBlock{
						Type:         "tool_result",
						ToolUseID:    item.ToolCallID,
						Content:      toolResultContent,
						IsError:      item.ToolCallIsError,
						CacheControl: convertCacheControlToAnthropic(item.CacheControl),
					}
				})

				// Check if there's a user message with the same MessageIndex
				// If so, merge the tool_result blocks with the user message content
				userMsgAtSameIndex := lo.Filter(chatReq.Messages, func(item llm.Message, _ int) bool {
					return item.Role == "user" && item.MessageIndex != nil && *item.MessageIndex == *msg.MessageIndex
				})

				if len(userMsgAtSameIndex) > 0 {
					// There's a user message with the same MessageIndex
					// Merge tool_result blocks with the user message content
					for _, userMsg := range userMsgAtSameIndex {
						if userMsg.Content.Content != nil && *userMsg.Content.Content != "" {
							contentBlocks = append(contentBlocks, MessageContentBlock{
								Type:         "text",
								Text:         *userMsg.Content.Content,
								CacheControl: convertCacheControlToAnthropic(userMsg.CacheControl),
							})
						} else if len(userMsg.Content.MultipleContent) > 0 {
							// Handle multiple content parts
							for _, part := range userMsg.Content.MultipleContent {
								if part.Type == "text" && part.Text != nil {
									contentBlocks = append(contentBlocks, MessageContentBlock{
										Type:         "text",
										Text:         *part.Text,
										CacheControl: convertCacheControlToAnthropic(part.CacheControl),
									})
								}
							}
						}
					}
				}

				messages = append(messages, MessageParam{
					Role: "user",
					Content: MessageContent{
						MultipleContent: contentBlocks,
					},
				})
				processedToolMessageIndexes[*msg.MessageIndex] = true
			}

			continue
		}

		// Skip user messages that were already merged with tool results
		if msg.Role == "user" && msg.MessageIndex != nil && processedToolMessageIndexes[*msg.MessageIndex] {
			continue
		}

		anthropicMsg := MessageParam{
			Role: lo.Ternary(msg.Role == "assistant", "assistant", "user"),
		}

		if len(msg.ToolCalls) > 0 {
			var preBlocks []MessageContentBlock

			if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
				preBlocks = append(preBlocks, MessageContentBlock{
					Type:     "thinking",
					Thinking: *msg.ReasoningContent,
				})
			}

			if msg.Content.Content != nil && *msg.Content.Content != "" {
				preBlocks = append(preBlocks, MessageContentBlock{
					Type:         "text",
					Text:         *msg.Content.Content,
					CacheControl: convertCacheControlToAnthropic(msg.CacheControl),
				})
			}

			content, existMultipleParts := convertMultiplePartContent(msg)

			switch {
			case existMultipleParts && len(preBlocks) > 0:
				content.MultipleContent = append(preBlocks, content.MultipleContent...)
			case existMultipleParts:
				// Do nothing, reuse the content directly.
			case len(preBlocks) > 0:
				// If only has one block, we need to use single Content format.
				if len(preBlocks) == 1 {
					if preBlocks[0].Type == "text" {
						content = MessageContent{
							Content: &preBlocks[0].Text,
						}
					} else {
						// This should not happen, but just in case.
						content = MessageContent{
							Content: &preBlocks[0].Thinking,
						}
					}
				} else {
					content = MessageContent{
						MultipleContent: preBlocks,
					}
				}
			default:
				continue
			}

			anthropicMsg.Content = content
			messages = append(messages, anthropicMsg)
		} else {
			if msg.Content.Content != nil {
				// If message has cache control or reasoning content, we need to use MultipleContent format
				if msg.CacheControl != nil || msg.ReasoningContent != nil {
					contentBlocks := make([]MessageContentBlock, 0, 2)

					// Add reasoning content (thinking) first if present
					// This matches Anthropic's expected format where thinking comes before text
					if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
						contentBlocks = append(contentBlocks, MessageContentBlock{
							Type:     "thinking",
							Thinking: *msg.ReasoningContent,
						})
					}

					// Add text content
					contentBlocks = append(contentBlocks, MessageContentBlock{
						Type:         "text",
						Text:         *msg.Content.Content,
						CacheControl: convertCacheControlToAnthropic(msg.CacheControl),
					})

					anthropicMsg.Content = MessageContent{
						MultipleContent: contentBlocks,
					}
				} else {
					anthropicMsg.Content = MessageContent{
						Content: msg.Content.Content,
					}
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

	if chatReq.Stop != nil {
		if chatReq.Stop.Stop != nil {
			req.StopSequences = []string{*chatReq.Stop.Stop}
		} else if len(chatReq.Stop.MultipleStop) > 0 {
			req.StopSequences = chatReq.Stop.MultipleStop
		}
	}

	return req
}

func convertToAnthropicSystemPrompt(chatReq *llm.Request) *SystemPrompt {
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
				part := SystemPromptPart{
					Type:         "text",
					Text:         *msg.Content.Content,
					CacheControl: convertCacheControlToAnthropic(msg.CacheControl),
				}

				return part
			}),
		}
	}
}

func convertMultiplePartContent(msg llm.Message) (MessageContent, bool) {
	blocks := make([]MessageContentBlock, 0, len(msg.Content.MultipleContent))

	// Process content parts in order to preserve original sequence
	for _, part := range msg.Content.MultipleContent {
		switch part.Type {
		case "text":
			if part.Text != nil {
				blocks = append(blocks, MessageContentBlock{
					Type:         "text",
					Text:         *part.Text,
					CacheControl: convertCacheControlToAnthropic(part.CacheControl),
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
							block := MessageContentBlock{
								Type: "image",
								Source: &ImageSource{
									Type:      "base64",
									MediaType: mediaType,
									Data:      parts[1],
								},
								CacheControl: convertCacheControlToAnthropic(part.CacheControl),
							}

							blocks = append(blocks, block)
						}
					}
				} else {
					block := MessageContentBlock{
						Type: "image",
						Source: &ImageSource{
							Type: "url",
							URL:  part.ImageURL.URL,
						},
						CacheControl: convertCacheControlToAnthropic(part.CacheControl),
					}

					blocks = append(blocks, block)
				}
			}
		}
	}

	for _, toolCall := range msg.ToolCalls {
		// Use safe JSON repair/fallback for tool input
		blocks = append(blocks, MessageContentBlock{
			Type:         "tool_use",
			ID:           toolCall.ID,
			Name:         &toolCall.Function.Name,
			Input:        xjson.SafeJSONRawMessage(toolCall.Function.Arguments),
			CacheControl: convertCacheControlToAnthropic(toolCall.CacheControl),
		})
	}

	if len(blocks) == 0 {
		return MessageContent{}, false
	}

	return MessageContent{
		MultipleContent: blocks,
	}, true
}

// convertToChatCompletionResponse converts Anthropic Message to unified Response format.
func convertToChatCompletionResponse(anthropicResp *Message, platformType PlatformType) *llm.Response {
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
		content      llm.MessageContent
		thinkingText string
		toolCalls    []llm.ToolCall
		textParts    []string
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
						URL: block.Source.Data,
					},
				})
			}
		case "tool_use":
			if block.ID != "" && block.Name != nil {
				// Repair or safely fallback invalid JSON from provider
				repaired := xjson.SafeJSONRawMessage(string(block.Input))
				toolCall := llm.ToolCall{
					ID:   block.ID,
					Type: "function",
					Function: llm.FunctionCall{
						Name:      *block.Name,
						Arguments: string(repaired),
					},
				}
				toolCalls = append(toolCalls, toolCall)
			}
		case "thinking":
			thinkingText = block.Thinking
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

	if thinkingText != "" {
		message.ReasoningContent = &thinkingText
	}

	choice := llm.Choice{
		Index:        0,
		Message:      message,
		FinishReason: convertFinishReason(anthropicResp.StopReason),
	}

	resp.Choices = []llm.Choice{choice}

	resp.Usage = convertToLlmUsage(anthropicResp.Usage, platformType)

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
