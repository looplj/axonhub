package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
)

func convertToLLMRequest(anthropicReq *MessageRequest) (*llm.Request, error) {
	chatReq := &llm.Request{
		Model:       anthropicReq.Model,
		MaxTokens:   &anthropicReq.MaxTokens,
		Temperature: anthropicReq.Temperature,
		TopP:        anthropicReq.TopP,
		Stream:      anthropicReq.Stream,
		Metadata:    map[string]string{},
	}
	if anthropicReq.Metadata != nil {
		chatReq.Metadata["user_id"] = anthropicReq.Metadata.UserID
	}

	// Convert messages
	messages := make([]llm.Message, 0, len(anthropicReq.Messages))

	// Add system message if present
	if anthropicReq.System != nil {
		if anthropicReq.System.Prompt != nil {
			systemContent := anthropicReq.System.Prompt
			messages = append(messages, llm.Message{
				Role: "system",
				Content: llm.MessageContent{
					Content: systemContent,
				},
			})
		} else if len(anthropicReq.System.MultiplePrompts) > 0 {
			for _, prompt := range anthropicReq.System.MultiplePrompts {
				messages = append(messages, llm.Message{
					Role: "system",
					Content: llm.MessageContent{
						Content: &prompt.Text,
					},
				})
			}
		}
	}

	// Convert Anthropic messages to ChatCompletionMessage
	for msgIndex, msg := range anthropicReq.Messages {
		chatMsg := llm.Message{
			Role: msg.Role,
		}

		var hasContent bool

		// Convert content
		if msg.Content.Content != nil {
			chatMsg.Content = llm.MessageContent{
				Content: msg.Content.Content,
			}
			hasContent = true
		} else if len(msg.Content.MultipleContent) > 0 {
			contentParts := make([]llm.MessageContentPart, 0, len(msg.Content.MultipleContent))
			for _, block := range msg.Content.MultipleContent {
				switch block.Type {
				case "text":
					contentParts = append(contentParts, llm.MessageContentPart{
						Type: "text",
						Text: &block.Text,
					})
					hasContent = true
				case "image":
					if block.Source != nil {
						if block.Source.Type == "base64" {
							// Convert Anthropic image format to OpenAI format
							imageURL := fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)
							contentParts = append(contentParts, llm.MessageContentPart{
								Type: "image_url",
								ImageURL: &llm.ImageURL{
									URL: imageURL,
								},
							})
						} else {
							contentParts = append(contentParts, llm.MessageContentPart{
								Type: "image_url",
								ImageURL: &llm.ImageURL{
									URL: block.Source.URL,
								},
							})
						}

						hasContent = true
					}
				case "tool_result":
					// TODO: support other result types
					if block.Content.Content != nil {
						messages = append(messages, llm.Message{
							Role:         "tool",
							MessageIndex: lo.ToPtr(msgIndex),
							ToolCallID:   block.ToolUseID,
							Content: llm.MessageContent{
								Content: block.Content.Content,
							},
							ToolCallIsError: block.IsError,
						})
					}
				case "tool_use":
					chatMsg.ToolCalls = append(chatMsg.ToolCalls, llm.ToolCall{
						ID:   block.ID,
						Type: "function",
						Function: llm.FunctionCall{
							Name:      lo.FromPtr(block.Name),
							Arguments: string(block.Input),
						},
					})
					hasContent = true
				}
			}

			// Check if it's a simple text-only message (single text block)
			if len(contentParts) == 1 && contentParts[0].Type == "text" {
				// Convert single text block to simple content format for compatibility
				chatMsg.Content = llm.MessageContent{
					Content: contentParts[0].Text,
				}
				hasContent = true
			} else {
				chatMsg.Content = llm.MessageContent{
					MultipleContent: contentParts,
				}
			}
		}

		if !hasContent {
			continue
		}

		messages = append(messages, chatMsg)
	}

	chatReq.Messages = messages

	// Convert tools
	if len(anthropicReq.Tools) > 0 {
		tools := make([]llm.Tool, 0, len(anthropicReq.Tools))
		for _, tool := range anthropicReq.Tools {
			llmTool := llm.Tool{
				Type: "function",
				Function: llm.Function{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
			tools = append(tools, llmTool)
		}

		chatReq.Tools = tools
	}

	// Convert stop sequences
	if len(anthropicReq.StopSequences) > 0 {
		if len(anthropicReq.StopSequences) == 1 {
			chatReq.Stop = &llm.Stop{
				Stop: &anthropicReq.StopSequences[0],
			}
		} else {
			chatReq.Stop = &llm.Stop{
				MultipleStop: anthropicReq.StopSequences,
			}
		}
	}

	// Convert thinking configuration to reasoning effort
	if anthropicReq.Thinking != nil && anthropicReq.Thinking.Type == "enabled" {
		chatReq.ReasoningEffort = thinkingBudgetToReasoningEffort(anthropicReq.Thinking.BudgetTokens)
	}

	return chatReq, nil
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
			var contentBlocks []MessageContentBlock

			// Handle reasoning content (thinking) first if present
			if message.ReasoningContent != nil && *message.ReasoningContent != "" {
				contentBlocks = append(contentBlocks, MessageContentBlock{
					Type:     "thinking",
					Thinking: *message.ReasoningContent,
				})
			}

			// Handle regular content
			if message.Content.Content != nil && *message.Content.Content != "" {
				contentBlocks = append(contentBlocks, MessageContentBlock{
					Type: "text",
					Text: *message.Content.Content,
				})
			} else if len(message.Content.MultipleContent) > 0 {
				for _, part := range message.Content.MultipleContent {
					switch part.Type {
					case "text":
						if part.Text != nil {
							contentBlocks = append(contentBlocks, MessageContentBlock{
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
										contentBlocks = append(contentBlocks, MessageContentBlock{
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
								contentBlocks = append(contentBlocks, MessageContentBlock{
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

					contentBlocks = append(contentBlocks, MessageContentBlock{
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
