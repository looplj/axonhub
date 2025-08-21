package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

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
	source                    streams.Stream[*llm.Response]
	ctx                       context.Context
	hasStarted                bool
	hasTextContentStarted     bool
	hasThinkingContentStarted bool
	hasToolContentStarted     bool
	hasFinished               bool
	messageID                 string
	model                     string
	contentIndex              int64
	eventQueue                []*httpclient.StreamEvent
	queueIndex                int
	err                       error
	stopReason                *string
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

	// Generate message_start event if this is the first chunk
	if !s.hasStarted {
		s.hasStarted = true

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

		// Handle reasoning content (thinking) delta
		if choice.Delta != nil && choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			// If the tool content has started before the thinking content, we need to stop it
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

			// Generate content_block_start if this is the first thinking content
			if !s.hasThinkingContentStarted {
				s.hasThinkingContentStarted = true

				streamEvent := StreamEvent{
					Type:  "content_block_start",
					Index: &s.contentIndex,
					ContentBlock: &ContentBlock{
						Type:      "thinking",
						Thinking:  "",
						Signature: "",
					},
				}

				err := s.enqueEvent(&streamEvent)
				if err != nil {
					s.err = fmt.Errorf("failed to enqueue content_block_start event: %w", err)
					return false
				}
			}

			// Generate content_block_delta for thinking
			streamEvent := StreamEvent{
				Type:  "content_block_delta",
				Index: &s.contentIndex,
				Delta: &StreamDelta{
					Type:     lo.ToPtr("thinking_delta"),
					Thinking: choice.Delta.ReasoningContent,
				},
			}

			err := s.enqueEvent(&streamEvent)
			if err != nil {
				s.err = fmt.Errorf("failed to enqueue content_block_delta event: %w", err)
				return false
			}
		}

		// Handle content delta
		if choice.Delta != nil && choice.Delta.Content.Content != nil && *choice.Delta.Content.Content != "" {
			// If the thinking content has started before the text content, we need to stop it
			if s.hasThinkingContentStarted {
				s.hasThinkingContentStarted = false

				// Add signature delta before stopping thinking block
				// TODO Confirm if this is needed.
				// signatureEvent := StreamEvent{
				// 	Type:  "content_block_delta",
				// 	Index: &s.contentIndex,
				// 	Delta: &StreamDelta{
				// 		Type:      lo.ToPtr("signature_delta"),
				// 		Signature: lo.ToPtr(""),
				// 	},
				// }

				// err := s.enqueEvent(&signatureEvent)
				// if err != nil {
				// 	s.err = fmt.Errorf("failed to enqueue signature_delta event: %w", err)
				// 	return false
				// }

				stopEvent := StreamEvent{
					Type:  "content_block_stop",
					Index: &s.contentIndex,
				}

				err := s.enqueEvent(&stopEvent)
				if err != nil {
					s.err = fmt.Errorf("failed to enqueue content_block_stop event: %w", err)
					return false
				}

				s.contentIndex += 1
			}

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
				} else {
					s.toolCalls[toolCallIndex].Function.Arguments += deltaToolCall.Function.Arguments

					// Generate content_block_delta for input_json_delta
					// contentBlockIndex := int64(toolCallIndex)
					// if s.hasTextContentStarted || s.hasThinkingContentStarted {
					// 	contentBlockIndex = s.contentIndex + 1 + int64(toolCallIndex)
					// }

					streamEvent := StreamEvent{
						Type:  "content_block_delta",
						Index: &s.contentIndex,
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
