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
)

// InboundTransformer implements transformer.Inbound for Anthropic format
type InboundTransformer struct {
	name string
}

// NewInboundTransformer creates a new Anthropic InboundTransformer
func NewInboundTransformer() transformer.Inbound {
	return &InboundTransformer{
		name: "anthropic-inbound",
	}
}

// MessageRequest represents the Anthropic Messages API request format
type MessageRequest struct {
	Model         string         `json:"model"`
	MaxTokens     int64          `json:"max_tokens"`
	Messages      []Message      `json:"messages"`
	System        *string        `json:"system,omitempty"`
	Temperature   *float64       `json:"temperature,omitempty"`
	TopP          *float64       `json:"top_p,omitempty"`
	TopK          *int64         `json:"top_k,omitempty"`
	StopSequences []string       `json:"stop_sequences,omitempty"`
	Stream        *bool          `json:"stream,omitempty"`
	Tools         []Tool         `json:"tools,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// Tool represents a tool definition for Anthropic API
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// InputSchema represents the JSON schema for tool input
type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// Message represents a message in Anthropic format
type Message struct {
	Role    string         `json:"role"`
	Content MessageContent `json:"content"`
}

// MessageContent supports both string and array formats
type MessageContent struct {
	Content         *string        `json:"content,omitempty"`
	MultipleContent []ContentBlock `json:"multiple_content,omitempty"`
}

func (c MessageContent) MarshalJSON() ([]byte, error) {
	if c.Content != nil {
		return json.Marshal(c.Content)
	}
	return json.Marshal(c.MultipleContent)
}

func (c *MessageContent) UnmarshalJSON(data []byte) error {
	// Handle null values
	if string(data) == "null" {
		return fmt.Errorf("content cannot be null")
	}
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		c.MultipleContent = blocks
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		c.Content = &str
		return nil
	}
	return fmt.Errorf("invalid content type")
}

// ContentBlock represents different types of content blocks
type ContentBlock struct {
	// Type is the type of the content block.
	// Available values: text, image, tool_use, tool_result
	Type   string       `json:"type"`
	Text   *string      `json:"text,omitempty"`
	Source *ImageSource `json:"source,omitempty"`

	// Tool use fields
	ID    *string         `json:"id,omitempty"`
	Name  *string         `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// Tool result fields
	ToolUseID *string `json:"tool_use_id,omitempty"`
	Content   *string `json:"content,omitempty"`
	IsError   *bool   `json:"is_error,omitempty"`
}

// ImageSource represents image source for Anthropic
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// TransformRequest transforms Anthropic HTTP request to ChatCompletionRequest
func (t *InboundTransformer) TransformRequest(ctx context.Context, httpReq *llm.GenericHttpRequest) (*llm.ChatCompletionRequest, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("http request is nil")
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}

	// Check content type
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = httpReq.Headers.Get("content-type")
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

	// Convert to ChatCompletionRequest
	chatReq := &llm.ChatCompletionRequest{
		Model:       anthropicReq.Model,
		MaxTokens:   &anthropicReq.MaxTokens,
		Temperature: anthropicReq.Temperature,
		TopP:        anthropicReq.TopP,
		Stream:      anthropicReq.Stream,
	}

	// Convert messages
	messages := make([]llm.ChatCompletionMessage, 0, len(anthropicReq.Messages))

	// Add system message if present
	if anthropicReq.System != nil {
		messages = append(messages, llm.ChatCompletionMessage{
			Role: "system",
			Content: llm.ChatCompletionMessageContent{
				Content: anthropicReq.System,
			},
		})
	}

	// Convert Anthropic messages to ChatCompletionMessage
	for _, msg := range anthropicReq.Messages {
		chatMsg := llm.ChatCompletionMessage{
			Role: msg.Role,
		}

		// Convert content
		if msg.Content.Content != nil {
			chatMsg.Content = llm.ChatCompletionMessageContent{
				Content: msg.Content.Content,
			}
		} else if len(msg.Content.MultipleContent) > 0 {
			contentParts := make([]llm.ContentPart, 0, len(msg.Content.MultipleContent))
			for _, block := range msg.Content.MultipleContent {
				switch block.Type {
				case "text":
					contentParts = append(contentParts, llm.ContentPart{
						Type: "text",
						Text: block.Text,
					})
				case "image":
					if block.Source != nil {
						// Convert Anthropic image format to OpenAI format
						imageURL := fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)
						contentParts = append(contentParts, llm.ContentPart{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: imageURL,
							},
						})
					}
				}
			}
			chatMsg.Content = llm.ChatCompletionMessageContent{
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

// TransformResponse transforms ChatCompletionResponse to Anthropic HTTP response
func (t *InboundTransformer) TransformResponse(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericHttpResponse, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	// Convert to Anthropic response format
	anthropicResp := t.convertToAnthropicResponse(chatResp)

	body, err := json.Marshal(anthropicResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic response: %w", err)
	}

	return &llm.GenericHttpResponse{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

// MessageResponse represents the Anthropic Messages API response format
type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   *string        `json:"stop_reason,omitempty"`
	StopSequence *string        `json:"stop_sequence,omitempty"`
	Usage        *Usage         `json:"usage,omitempty"`
}

// Usage represents usage information in Anthropic format
type Usage struct {
	InputTokens              int    `json:"input_tokens"`
	OutputTokens             int    `json:"output_tokens"`
	CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
	ServiceTier              string `json:"service_tier"`
}

func (t *InboundTransformer) convertToAnthropicResponse(chatResp *llm.ChatCompletionResponse) *MessageResponse {
	resp := &MessageResponse{
		ID:    chatResp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: chatResp.Model,
	}

	// Convert choices to content blocks
	if len(chatResp.Choices) > 0 {
		choice := chatResp.Choices[0]
		var message *llm.ChatCompletionMessage

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
						Text: message.Content.Content,
					},
				}
			} else if len(message.Content.MultipleContent) > 0 {
				content := make([]ContentBlock, 0, len(message.Content.MultipleContent))
				for _, part := range message.Content.MultipleContent {
					if part.Type == "text" && part.Text != nil {
						content = append(content, ContentBlock{
							Type: "text",
							Text: part.Text,
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
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
		}
	}

	return resp
}

// TransformStreamChunk transforms ChatCompletionResponse to GenericStreamEvent
func (t *InboundTransformer) TransformStreamChunk(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericStreamEvent, error) {
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
			usage.InputTokens = chatResp.Usage.PromptTokens
			usage.OutputTokens = chatResp.Usage.CompletionTokens
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
			Index: func() *int { i := 0; return &i }(),
			ContentBlock: &ContentBlock{
				Type: "text",
				Text: lo.ToPtr(""),
			},
		}

	case "content_block_delta":
		streamEvent = StreamEvent{
			Type:  "content_block_delta",
			Index: func() *int { i := 0; return &i }(),
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
			Index: lo.ToPtr(0),
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
				InputTokens:  chatResp.Usage.PromptTokens,
				OutputTokens: chatResp.Usage.CompletionTokens,
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
			var message *llm.ChatCompletionMessage

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

	return &llm.GenericStreamEvent{
		Type: eventType,
		Data: eventData,
	}, nil
}
