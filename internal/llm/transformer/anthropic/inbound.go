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
	"github.com/looplj/axonhub/internal/pkg/streams"
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

func (t *InboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	// Create a custom stream that handles the stateful transformation
	return &anthropicInboundStream{
		source:    stream,
		ctx:       ctx,
		toolCalls: make(map[int]*llm.ToolCall),
	}, nil
}

// anthropicInboundStream implements the stateful stream transformation.
//
//nolint:containedctx // Checked.
type anthropicInboundStream struct {
	source                streams.Stream[*llm.Response]
	ctx                   context.Context
	hasStarted            bool
	hasTextContentStarted bool
	hasToolContentStarted bool
	hasFinished           bool
	messageID             string
	model                 string
	contentIndex          int64
	eventQueue            []*httpclient.StreamEvent
	queueIndex            int
	err                   error
	stopReason            *string
	// Tool call tracking
	toolCalls map[int]*llm.ToolCall // Track tool calls by index
}

func (s *anthropicInboundStream) enqueEvent(ev *StreamEvent) error {
	eventData, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	s.eventQueue = append(s.eventQueue, &httpclient.StreamEvent{
		Type: ev.Type,
		Data: eventData,
	})

	return nil
}

//nolint:maintidx // It is complex, and hard to split.
func (s *anthropicInboundStream) Next() bool {
	// If we have events in the queue, return them first
	if s.queueIndex < len(s.eventQueue) {
		return true
	}

	// Clear the queue and reset index for new events
	s.eventQueue = nil
	s.queueIndex = 0

	// Try to get the next chunk from source
	if !s.source.Next() {
		return false
	}

	chunk := s.source.Current()
	if chunk == nil {
		return s.Next() // Try next chunk
	}

	// Handle [DONE] marker
	if chunk.Object == "[DONE]" {
		return s.Next() // Try next chunk
	}

	// Initialize message ID and model from first chunk
	if s.messageID == "" && chunk.ID != "" {
		s.messageID = chunk.ID
	}

	if s.model == "" && chunk.Model != "" {
		s.model = chunk.Model
	}

	// Set defaults if still empty
	if s.messageID == "" {
		s.messageID = "msg_unknown"
	}

	if s.model == "" {
		s.model = "claude-3-sonnet-20240229"
	}

	// Generate message_start event if this is the first chunk
	if !s.hasStarted {
		s.hasStarted = true

		// For message_start, set input_tokens to 1 as default since we don't have usage yet
		usage := &Usage{
			InputTokens:  1,
			OutputTokens: 1,
		}

		streamEvent := StreamEvent{
			Type: "message_start",
			Message: &StreamMessage{
				ID:      s.messageID,
				Type:    "message",
				Role:    "assistant",
				Model:   s.model,
				Content: []ContentBlock{},
				Usage:   usage,
			},
		}

		err := s.enqueEvent(&streamEvent)
		if err != nil {
			s.err = fmt.Errorf("failed to enqueue message_start event: %w", err)
			return false
		}
	}

	// Process the current chunk
	if len(chunk.Choices) > 0 {
		choice := chunk.Choices[0]

		// Handle content delta
		if choice.Delta != nil && choice.Delta.Content.Content != nil && *choice.Delta.Content.Content != "" {
			// If the tool content has started before the content block, we need to stop it
			if s.hasToolContentStarted {
				s.hasToolContentStarted = false

				streamEvent := StreamEvent{
					Type:  "content_block_stop",
					Index: &s.contentIndex,
				}

				err := s.enqueEvent(&streamEvent)
				if err != nil {
					s.err = fmt.Errorf("failed to enqueue content_block_stop event: %w", err)
					return false
				}

				s.contentIndex += 1
			}

			// Generate content_block_start if this is the first content
			if !s.hasTextContentStarted {
				s.hasTextContentStarted = true

				streamEvent := StreamEvent{
					Type:  "content_block_start",
					Index: &s.contentIndex,
					ContentBlock: &ContentBlock{
						Type: "text",
						Text: "",
					},
				}

				err := s.enqueEvent(&streamEvent)
				if err != nil {
					s.err = fmt.Errorf("failed to enqueue content_block_start event: %w", err)
					return false
				}
			}

			// Generate content_block_delta
			streamEvent := StreamEvent{
				Type:  "content_block_delta",
				Index: &s.contentIndex,
				Delta: &StreamDelta{
					Type: lo.ToPtr("text_delta"),
					Text: choice.Delta.Content.Content,
				},
			}

			err := s.enqueEvent(&streamEvent)
			if err != nil {
				s.err = fmt.Errorf("failed to enqueue content_block_delta event: %w", err)
				return false
			}
		}

		// Handle tool calls
		if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
			// If the text content has started before the tool content, we need to stop it
			if s.hasTextContentStarted {
				s.hasTextContentStarted = false

				streamEvent := StreamEvent{
					Type:  "content_block_stop",
					Index: &s.contentIndex,
				}

				err := s.enqueEvent(&streamEvent)
				if err != nil {
					s.err = fmt.Errorf("failed to enqueue content_block_stop event: %w", err)
					return false
				}

				s.contentIndex += 1
			}

			for _, deltaToolCall := range choice.Delta.ToolCalls {
				toolCallIndex := deltaToolCall.Index

				// Initialize tool call if it doesn't exist
				if _, ok := s.toolCalls[toolCallIndex]; !ok {
					// Start a new tool use block, we should stop the previous tool use block
					if toolCallIndex > 0 {
						streamEvent := StreamEvent{
							Type:  "content_block_stop",
							Index: &s.contentIndex,
						}

						err := s.enqueEvent(&streamEvent)
						if err != nil {
							s.err = fmt.Errorf("failed to enqueue content_block_stop event: %w", err)
							return false
						}

						s.contentIndex += 1
					}

					s.toolCalls[toolCallIndex] = &llm.ToolCall{
						Index: toolCallIndex,
						ID:    deltaToolCall.ID,
						Type:  deltaToolCall.Type,
						Function: llm.FunctionCall{
							Name:      deltaToolCall.Function.Name,
							Arguments: "",
						},
					}

					streamEvent := StreamEvent{
						Type:  "content_block_start",
						Index: &s.contentIndex,
						ContentBlock: &ContentBlock{
							Type:  "tool_use",
							ID:    deltaToolCall.ID,
							Name:  &deltaToolCall.Function.Name,
							Input: json.RawMessage("{}"),
						},
					}

					err := s.enqueEvent(&streamEvent)
					if err != nil {
						s.err = fmt.Errorf("failed to enqueue content_block_start event: %w", err)
						return false
					}
				}

				// Accumulate arguments
				if deltaToolCall.Function.Arguments != "" {
					s.toolCalls[toolCallIndex].Function.Arguments += deltaToolCall.Function.Arguments

					// Generate content_block_delta for input_json_delta
					contentBlockIndex := int64(toolCallIndex)
					if s.hasTextContentStarted {
						contentBlockIndex = s.contentIndex + 1 + int64(toolCallIndex)
					}

					streamEvent := StreamEvent{
						Type:  "content_block_delta",
						Index: &contentBlockIndex,
						Delta: &StreamDelta{
							Type:        lo.ToPtr("input_json_delta"),
							PartialJSON: &deltaToolCall.Function.Arguments,
						},
					}

					err := s.enqueEvent(&streamEvent)
					if err != nil {
						s.err = fmt.Errorf("failed to enqueue content_block_delta event: %w", err)
						return false
					}
				}
			}
		}

		// Handle finish reason
		if choice.FinishReason != nil && !s.hasFinished {
			s.hasFinished = true

			streamEvent := StreamEvent{
				Type:  "content_block_stop",
				Index: &s.contentIndex,
			}

			err := s.enqueEvent(&streamEvent)
			if err != nil {
				s.err = fmt.Errorf("failed to enqueue content_block_stop event: %w", err)
				return false
			}

			// Convert finish reason to Anthropic format
			var stopReason string

			switch *choice.FinishReason {
			case "stop":
				stopReason = "end_turn"
			case "length":
				stopReason = "max_tokens"
			case "tool_calls":
				stopReason = "tool_use"
			default:
				stopReason = "end_turn"
			}

			// Store the stop reason, but don't generate message_delta yet
			// We'll wait for the usage chunk to combine them
			s.stopReason = &stopReason
		}
	} else if chunk.Usage != nil && s.hasFinished {
		// Usage-only chunk after finish_reason - generate message_delta with both stop reason and usage
		streamEvent := StreamEvent{
			Type: "message_delta",
		}

		if s.stopReason != nil {
			streamEvent.Delta = &StreamDelta{
				StopReason: s.stopReason,
			}
		}

		usage := &Usage{
			InputTokens:  int64(chunk.Usage.PromptTokens),
			OutputTokens: int64(chunk.Usage.CompletionTokens),
		}

		// Map detailed token information
		if chunk.Usage.PromptTokensDetails != nil {
			usage.CacheReadInputTokens = int64(chunk.Usage.PromptTokensDetails.CachedTokens)
		}

		streamEvent.Usage = usage

		err := s.enqueEvent(&streamEvent)
		if err != nil {
			s.err = fmt.Errorf("failed to enqueue message_delta event: %w", err)
			return false
		}

		// Generate message_stop
		stopEvent := StreamEvent{
			Type: "message_stop",
		}

		err = s.enqueEvent(&stopEvent)
		if err != nil {
			s.err = fmt.Errorf("failed to enqueue message_stop event: %w", err)
			return false
		}
	}

	// If we have events in the queue, return true
	return len(s.eventQueue) > 0
}

func (s *anthropicInboundStream) Current() *httpclient.StreamEvent {
	if s.queueIndex < len(s.eventQueue) {
		event := s.eventQueue[s.queueIndex]
		s.queueIndex++

		return event
	}

	return nil
}

func (s *anthropicInboundStream) Err() error {
	if s.err != nil {
		return s.err
	}

	return s.source.Err()
}

func (s *anthropicInboundStream) Close() error {
	return s.source.Close()
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
