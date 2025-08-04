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
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// InboundTransformer implements transformer.Inbound for Anthropic format.
type InboundTransformer struct{}

// NewInboundTransformer creates a new Anthropic InboundTransformer.
func NewInboundTransformer() transformer.Inbound {
	return &InboundTransformer{}
}

// TransformRequest transforms Anthropic HTTP request to ChatCompletionRequest.
func (t *InboundTransformer) TransformRequest(
	ctx context.Context,
	httpReq *httpclient.Request,
) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("http request is nil")
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}

	// Check content type
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = httpReq.Headers.Get("Content-Type")
	}

	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	var anthropicReq MessageRequest
	if err := json.Unmarshal(httpReq.Body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("failed to decode anthropic request: %w", err)
	}

	// Validate required fields
	if anthropicReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if len(anthropicReq.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	if anthropicReq.MaxTokens <= 0 {
		return nil, fmt.Errorf("max_tokens is required and must be positive")
	}

	// Validate system prompt format
	if anthropicReq.System != nil {
		if anthropicReq.System.Prompt == nil && len(anthropicReq.System.MultiplePrompts) > 0 {
			// Validate that all system prompts are text type
			for _, prompt := range anthropicReq.System.MultiplePrompts {
				if prompt.Type != "text" {
					return nil, fmt.Errorf(
						"system prompt array must contain only text type elements",
					)
				}
			}
		}
	}

	// Convert to ChatCompletionRequest
	chatReq := &llm.Request{
		Model:       anthropicReq.Model,
		MaxTokens:   &anthropicReq.MaxTokens,
		Temperature: anthropicReq.Temperature,
		TopP:        anthropicReq.TopP,
		Stream:      anthropicReq.Stream,
	}

	// Convert messages
	messages := make([]llm.Message, 0, len(anthropicReq.Messages))

	// Add system message if present
	if anthropicReq.System != nil {
		var systemContent *string
		if anthropicReq.System.Prompt != nil {
			systemContent = anthropicReq.System.Prompt
		} else if len(anthropicReq.System.MultiplePrompts) > 0 {
			// Join multiple system prompts
			var systemText string
			for _, prompt := range anthropicReq.System.MultiplePrompts {
				systemText += prompt.Text + "\n"
			}
			systemContent = &systemText
		}
		if systemContent != nil {
			messages = append(messages, llm.Message{
				Role: "system",
				Content: llm.MessageContent{
					Content: systemContent,
				},
			})
		}
	}

	// Convert Anthropic messages to ChatCompletionMessage
	for _, msg := range anthropicReq.Messages {
		chatMsg := llm.Message{
			Role: msg.Role,
		}

		// Convert content
		if msg.Content.Content != nil {
			chatMsg.Content = llm.MessageContent{
				Content: msg.Content.Content,
			}
		} else if len(msg.Content.MultipleContent) > 0 {
			contentParts := make([]llm.MessageContentPart, 0, len(msg.Content.MultipleContent))
			for _, block := range msg.Content.MultipleContent {
				switch block.Type {
				case "text":
					contentParts = append(contentParts, llm.MessageContentPart{
						Type: "text",
						Text: &block.Text,
					})
				case "image":
					if block.Source != nil {
						// Convert Anthropic image format to OpenAI format
						imageURL := fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)
						contentParts = append(contentParts, llm.MessageContentPart{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: imageURL,
							},
						})
					}
				}
			}
			chatMsg.Content = llm.MessageContent{
				MultipleContent: contentParts,
			}
		}

		messages = append(messages, chatMsg)
	}

	chatReq.Messages = messages

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

	return chatReq, nil
}

// TransformResponse transforms ChatCompletionResponse to Anthropic HTTP response.
func (t *InboundTransformer) TransformResponse(
	ctx context.Context,
	chatResp *llm.Response,
) (*httpclient.Response, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	// Convert to Anthropic response format
	anthropicResp := t.convertToAnthropicResponse(chatResp)

	body, err := json.Marshal(anthropicResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic response: %w", err)
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

func (t *InboundTransformer) convertToAnthropicResponse(chatResp *llm.Response) *Message {
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
			// Convert content
			if message.Content.Content != nil {
				resp.Content = []ContentBlock{
					{
						Type: "text",
						Text: *message.Content.Content,
					},
				}
			} else if len(message.Content.MultipleContent) > 0 {
				content := make([]ContentBlock, 0, len(message.Content.MultipleContent))
				for _, part := range message.Content.MultipleContent {
					if part.Type == "text" && part.Text != nil {
						content = append(content, ContentBlock{
							Type: "text",
							Text: *part.Text,
						})
					}
				}
				resp.Content = content
			}
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
			default:
				resp.StopReason = choice.FinishReason
			}
		}
	}

	// Convert usage
	if chatResp.Usage != nil {
		resp.Usage = &Usage{
			InputTokens:  int64(chatResp.Usage.PromptTokens),
			OutputTokens: int64(chatResp.Usage.CompletionTokens),
		}
	}

	return resp
}

// TransformStreamChunk transforms ChatCompletionResponse to StreamEvent.
func (t *InboundTransformer) TransformStreamChunk(
	ctx context.Context,
	chatResp *llm.Response,
) (*httpclient.StreamEvent, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	// Use the object field to determine the event type, similar to OutboundTransformer
	eventType := chatResp.Object

	// Convert ChatCompletionResponse to Anthropic StreamEvent based on the object type
	var streamEvent StreamEvent

	switch eventType {
	case "message_start":
		usage := &Usage{
			ServiceTier: chatResp.ServiceTier,
		}
		if chatResp.Usage != nil {
			usage.InputTokens = int64(chatResp.Usage.PromptTokens)
			usage.OutputTokens = int64(chatResp.Usage.CompletionTokens)
		}
		streamEvent = StreamEvent{
			Type: "message_start",
			Message: &StreamMessage{
				ID:      chatResp.ID,
				Type:    "message",
				Role:    "assistant",
				Model:   chatResp.Model,
				Content: []ContentBlock{},
				Usage:   usage,
			},
		}
	case "ping":
		streamEvent = StreamEvent{
			Type: "ping",
		}
	case "content_block_start":
		streamEvent = StreamEvent{
			Type:  "content_block_start",
			Index: func() *int64 { i := int64(0); return &i }(),
			ContentBlock: &ContentBlock{
				Type: "text",
				Text: "",
			},
		}

	case "content_block_delta":
		streamEvent = StreamEvent{
			Type:  "content_block_delta",
			Index: func() *int64 { i := int64(0); return &i }(),
		}

		// Extract content from choices
		if len(chatResp.Choices) > 0 && chatResp.Choices[0].Delta != nil {
			choice := chatResp.Choices[0]
			if choice.Delta.Content.Content != nil {
				streamEvent.Delta = &StreamDelta{
					Type: lo.ToPtr("text_delta"),
					Text: choice.Delta.Content.Content,
				}
			}
		}

	case "content_block_stop":
		streamEvent = StreamEvent{
			Type:  "content_block_stop",
			Index: lo.ToPtr(int64(0)),
		}

	case "message_delta":
		streamEvent = StreamEvent{
			Type: "message_delta",
		}

		// Extract finish reason and usage from choices
		if len(chatResp.Choices) > 0 {
			choice := chatResp.Choices[0]
			if choice.FinishReason != nil {
				// Convert finish reason to Anthropic format
				var stopReason *string
				switch *choice.FinishReason {
				case "stop":
					reason := "end_turn"
					stopReason = &reason
				case "length":
					reason := "max_tokens"
					stopReason = &reason
				case "tool_calls":
					reason := "tool_use"
					stopReason = &reason
				default:
					stopReason = choice.FinishReason
				}

				streamEvent.Delta = &StreamDelta{
					StopReason: stopReason,
				}
			}
		}

		// Add usage if available
		if chatResp.Usage != nil {
			streamEvent.Usage = &Usage{
				InputTokens:  int64(chatResp.Usage.PromptTokens),
				OutputTokens: int64(chatResp.Usage.CompletionTokens),
			}
		}

	case "message_stop":
		streamEvent = StreamEvent{
			Type: "message_stop",
		}

	default:
		// For unknown types or "data", create a generic event
		streamEvent = StreamEvent{
			Type: eventType,
		}

		// Try to extract content from choices if available
		if len(chatResp.Choices) > 0 {
			choice := chatResp.Choices[0]
			var message *llm.Message

			if choice.Message != nil {
				message = choice.Message
			} else if choice.Delta != nil {
				message = choice.Delta
			}

			if message != nil && message.Content.Content != nil {
				streamEvent.Delta = &StreamDelta{
					Type: lo.ToPtr("text"),
					Text: message.Content.Content,
				}
			}
		}
	}

	// Marshal the stream event to JSON
	eventData, err := json.Marshal(streamEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stream event: %w", err)
	}

	return &httpclient.StreamEvent{
		Type: eventType,
		Data: eventData,
	}, nil
}
