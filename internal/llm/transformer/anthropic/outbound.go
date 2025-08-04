package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// OutboundTransformer implements transformer.Outbound for Anthropic format.
type OutboundTransformer struct {
	name    string
	baseURL string
	apiKey  string
}

// NewOutboundTransformer creates a new Anthropic OutboundTransformer.
func NewOutboundTransformer(baseURL, apiKey string) transformer.Outbound {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	return &OutboundTransformer{
		name:    "anthropic-outbound",
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// TransformRequest transforms ChatCompletionRequest to Anthropic HTTP request.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	chatReq *llm.Request,
) (*httpclient.Request, error) {
	if chatReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Validate required fields
	if chatReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if len(chatReq.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	// Validate max_tokens
	if chatReq.MaxTokens != nil && *chatReq.MaxTokens <= 0 {
		return nil, fmt.Errorf("max_tokens must be positive")
	}

	// Convert to Anthropic request format
	anthropicReq := t.convertToAnthropicRequest(chatReq)

	// Marshal the request body
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	headers.Set("Anthropic-Version", "2023-06-01")

	// Prepare authentication
	var auth *httpclient.AuthConfig
	if t.apiKey != "" {
		auth = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    t.apiKey,
			HeaderKey: "x-api-key",
		}
	}

	// Determine endpoint
	endpoint := "/v1/messages"
	url := strings.TrimSuffix(t.baseURL, "/") + endpoint

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

func (t *OutboundTransformer) convertToAnthropicRequest(chatReq *llm.Request) *MessageRequest {
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

// TransformResponse transforms Anthropic HTTP response to ChatCompletionResponse.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check for HTTP errors
	if httpResp.StatusCode >= 400 {
		if httpResp.Error != nil {
			return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, httpResp.Error.Message)
		}
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var anthropicResp Message
	if err := json.Unmarshal(httpResp.Body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal anthropic response: %w", err)
	}

	// Convert to ChatCompletionResponse
	chatResp := t.convertToChatCompletionResponse(&anthropicResp)

	return chatResp, nil
}

// TransformStreamChunk transforms a single Anthropic streaming chunk to ChatCompletionResponse.
func (t *OutboundTransformer) TransformStreamChunk(
	ctx context.Context,
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	if event == nil {
		return nil, fmt.Errorf("stream event is nil")
	}

	if len(event.Data) == 0 {
		return nil, fmt.Errorf("event data is empty")
	}

	// Parse the streaming event
	var streamEvent StreamEvent
	if err := json.Unmarshal(event.Data, &streamEvent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal anthropic stream event: %w", err)
	}

	// Convert the stream event to ChatCompletionResponse
	resp := &llm.Response{
		Object: streamEvent.Type,
	}

	switch streamEvent.Type {
	case "message_start":
		if streamEvent.Message != nil {
			resp.ID = streamEvent.Message.ID
			resp.Model = streamEvent.Message.Model
			resp.Created = 0
			resp.ServiceTier = streamEvent.Message.Usage.ServiceTier
			resp.Usage = &llm.Usage{
				PromptTokens:     int(streamEvent.Message.Usage.InputTokens),
				CompletionTokens: int(streamEvent.Message.Usage.OutputTokens),
				TotalTokens: int(
					streamEvent.Message.Usage.InputTokens + streamEvent.Message.Usage.OutputTokens,
				),
			}
		}
		// For message_start, we return an empty choice to indicate the start
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr(""),
					},
				},
			},
		}

	case "content_block_start":
		// Initialize content block
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr(""),
					},
				},
			},
		}

	case "ping":
		// Ping event - return empty response to indicate connection is alive

	case "content_block_delta":
		if streamEvent.Delta != nil && streamEvent.Delta.Text != nil {
			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: streamEvent.Delta.Text,
						},
					},
				},
			}
		}

	case "content_block_stop":
		// Content block finished
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr(""),
					},
				},
			},
		}

	case "message_delta":
		if streamEvent.Delta != nil && streamEvent.Delta.StopReason != nil {
			// Determine finish reason
			var finishReason *string
			switch *streamEvent.Delta.StopReason {
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
				finishReason = streamEvent.Delta.StopReason
			}

			resp.Choices = []llm.Choice{
				{
					Index:        0,
					FinishReason: finishReason,
					Delta: &llm.Message{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: func() *string { s := ""; return &s }(),
						},
					},
				},
			}
		}

		// Add usage if available
		if streamEvent.Usage != nil {
			resp.Usage = &llm.Usage{
				PromptTokens:     int(streamEvent.Usage.InputTokens),
				CompletionTokens: int(streamEvent.Usage.OutputTokens),
				TotalTokens: int(
					streamEvent.Usage.InputTokens + streamEvent.Usage.OutputTokens,
				),
			}
		}

	case "message_stop":
		// Final event - return empty response to indicate completion
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: func() *string { s := ""; return &s }(),
					},
				},
				FinishReason: func() *string { s := "stop"; return &s }(),
			},
		}

	default:
		// Unknown event type, return empty response
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: func() *string { s := ""; return &s }(),
					},
				},
			},
		}
	}

	return resp, nil
}

func (t *OutboundTransformer) convertToChatCompletionResponse(
	anthropicResp *Message,
) *llm.Response {
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
	var content llm.MessageContent
	var toolCalls []llm.ToolCall
	var textParts []string

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
		resp.Usage = &llm.Usage{
			PromptTokens:     int(anthropicResp.Usage.InputTokens),
			CompletionTokens: int(anthropicResp.Usage.OutputTokens),
			TotalTokens: int(
				anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
			),
		}
	}

	return resp
}

// AggregateStreamChunks aggregates Anthropic streaming response chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks [][]byte,
) (*llm.Response, error) {
	if len(chunks) == 0 {
		return &llm.Response{}, nil
	}

	var messageStart *StreamEvent
	var contentBlocks []ContentBlock
	var usage *Usage
	var stopReason *string

	for _, chunk := range chunks {
		var event StreamEvent
		if err := json.Unmarshal(chunk, &event); err != nil {
			continue // Skip invalid chunks
		}

		log.Debug(ctx, "chat stream event", log.Any("event", event))

		switch event.Type {
		case "message_start":
			messageStart = &event
		case "content_block_start":
			if event.ContentBlock != nil {
				contentBlocks = append(contentBlocks, *event.ContentBlock)
			}
		case "content_block_delta":
			if event.Index != nil {
				index := int(*event.Index)
				// Ensure we have enough content blocks
				for len(contentBlocks) <= index {
					contentBlocks = append(contentBlocks, ContentBlock{Type: "text", Text: ""})
				}

				if event.Delta != nil {
					if event.Delta.Text != nil {
						if contentBlocks[index].Type == "text" {
							contentBlocks[index].Text += *event.Delta.Text
						}
					}
					if event.Delta.Thinking != nil {
						if contentBlocks[index].Type == "thinking" {
							contentBlocks[index].Thinking += *event.Delta.Thinking
						} else {
							// Convert to thinking block if it's not already
							contentBlocks[index].Type = "thinking"
							contentBlocks[index].Thinking = *event.Delta.Thinking
						}
					}
					if event.Delta.Signature != nil {
						// Handle signature delta - append to thinking block signature
						if contentBlocks[index].Type == "thinking" {
							contentBlocks[index].Signature += *event.Delta.Signature
						} else {
							// Convert to thinking block if it's not already
							contentBlocks[index].Type = "thinking"
							contentBlocks[index].Signature = *event.Delta.Signature
						}
					}
					if event.Delta.PartialJSON != nil {
						switch contentBlocks[index].Type {
						case "tool_use":
							if contentBlocks[index].Input == nil {
								contentBlocks[index].Input = []byte(*event.Delta.PartialJSON)
							} else {
								contentBlocks[index].Input = append(contentBlocks[index].Input, []byte(*event.Delta.PartialJSON)...)
							}
						case "text":
							contentBlocks[index].Text += *event.Delta.PartialJSON
						}
					}
				}
			}
		case "message_delta":
			if event.Delta != nil {
				if event.Delta.StopReason != nil {
					stopReason = event.Delta.StopReason
				}
			}
			if event.Usage != nil {
				usage = event.Usage
			}
		case "message_stop":
			// Final event, no additional processing needed
		}
	}

	// If no message_start event, create a default message
	var message *Message
	if messageStart != nil {
		// Ensure we have at least one content block
		if len(contentBlocks) == 0 {
			contentBlocks = []ContentBlock{
				{Type: "text", Text: ""},
			}
		}

		message = &Message{
			ID:         messageStart.Message.ID,
			Type:       messageStart.Message.Type,
			Role:       messageStart.Message.Role,
			Content:    contentBlocks,
			Model:      messageStart.Message.Model,
			StopReason: stopReason,
			Usage:      usage,
		}
	} else {
		// Ensure we have at least one content block
		if len(contentBlocks) == 0 {
			contentBlocks = []ContentBlock{
				{Type: "text", Text: ""},
			}
		}

		// Create a default message when no message_start event is received
		message = &Message{
			ID:         "msg_unknown",
			Type:       "message",
			Role:       "assistant",
			Content:    contentBlocks,
			Model:      "claude-3-sonnet-20240229",
			StopReason: stopReason,
			Usage:      usage,
		}
	}

	return t.convertToChatCompletionResponse(message), nil
}

// SetAPIKey updates the API key.
func (t *OutboundTransformer) SetAPIKey(apiKey string) {
	t.apiKey = apiKey
}

// SetBaseURL updates the base URL.
func (t *OutboundTransformer) SetBaseURL(baseURL string) {
	t.baseURL = baseURL
}
