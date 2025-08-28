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

func (t *InboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatAnthropicMessage
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

		var hasContent bool

		// Convert content
		if msg.Content.Content != nil {
			chatMsg.Content = llm.MessageContent{
				Content: msg.Content.Content,
			}
			hasContent = true
		} else if len(msg.Content.MultipleContent) > 0 {
			// Handle multimodal content
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
					content, err := json.Marshal(block.Content)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal tool result: %w", err)
					}
					messages = append(messages, llm.Message{
						Role:       "tool",
						ToolCallID: block.ToolUseID,
						Content: llm.MessageContent{
							Content: lo.ToPtr(string(content)),
						},
					})
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

	return chatReq, nil
}

// TransformResponse transforms ChatCompletionResponse to Anthropic HTTP response.
func (t *InboundTransformer) TransformResponse(ctx context.Context, chatResp *llm.Response) (*httpclient.Response, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	// Convert to Anthropic response format
	anthropicResp := convertToAnthropicResponse(chatResp)

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

func (t *InboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, *llm.Usage, error) {
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
