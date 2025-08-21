package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// InboundTransformer implements transformer.Inbound for Anthropic format.
type InboundTransformer struct{}

// NewInboundTransformer creates a new Anthropic InboundTransformer.
func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

// Name returns the name of the transformer.
func (t *InboundTransformer) Name() string {
	return "claude/messages"
}

// TransformRequest transforms Anthropic HTTP request to ChatCompletionRequest.
func (t *InboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
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

	err := json.Unmarshal(httpReq.Body, &anthropicReq)
	if err != nil {
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
					if part.Type == "text" && part.Text != nil {
						contentBlocks = append(contentBlocks, ContentBlock{
							Type: "text",
							Text: *part.Text,
						})
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

func (t *InboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, error) {
	return AggregateStreamChunks(ctx, chunks)
}

// TransformError transforms LLM error response to HTTP error response in Anthropic format.
func (t *InboundTransformer) TransformError(ctx context.Context, rawErr *llm.ResponseError) *httpclient.Error {
	if rawErr == nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body:       []byte(`{"message":"internal server error","request_id":""}`),
		}
	}

	aErr := &AnthropicErr{
		StatusCode: rawErr.StatusCode,
		Message:    rawErr.Detail.Message,
		RequestID:  rawErr.Detail.RequestID,
	}

	body, err := json.Marshal(aErr)
	if err != nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body:       []byte(`{"message":"internal server error","type":"internal_server_error"}`),
		}
	}

	return &httpclient.Error{
		StatusCode: lo.Ternary(rawErr.StatusCode != 0, rawErr.StatusCode, http.StatusInternalServerError),
		Status:     http.StatusText(rawErr.StatusCode),
		Body:       body,
	}
}
