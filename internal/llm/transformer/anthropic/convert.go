package anthropic

import (
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
		// Default max_tokens if not specified
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
					if part.ImageURL != nil {
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
						}
					}
				}
			}

			anthropicMsg.Content = MessageContent{
				MultipleContent: blocks,
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

	// Note: Anthropic doesn't support top_k parameter directly
	// It's handled through their model's internal sampling

	return req
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
