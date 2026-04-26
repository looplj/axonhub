package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
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
	messageStoped             bool
	messageID                 string
	model                     string
	contentIndex              int64
	eventQueue                []*httpclient.StreamEvent
	queueIndex                int
	err                       error
	stopReason                *string
	// Tool call tracking
	toolCalls map[int]*llm.ToolCall // Track tool calls by index

	lastEventType string

	// Buffered signature: when signature arrives before thinking starts,
	// we hold it until thinking finishes.
	pendingSignature *string
}

// generateSignature generates a random signature using base64(uuid).
func generateSignature() string {
	return base64.StdEncoding.EncodeToString([]byte(uuid.New().String()))
}

// closeThinkingBlock ensures any open or implied thinking block is properly
// closed. It handles three scenarios:
//  1. pendingSignature exists but no thinking block was started — creates a
//     synthetic empty thinking block (start + signature_delta + stop).
//  2. A thinking block is open — flushes any pending signature as
//     signature_delta, then emits content_block_stop.
//  3. Neither — no-op.
//
// If no signature is available when closing a thinking block, a random
// base64-encoded UUID is generated as a placeholder signature.
func (s *anthropicInboundStream) closeThinkingBlock() error {
	if s.pendingSignature != nil && !s.hasThinkingContentStarted {
		sig := s.pendingSignature
		s.pendingSignature = nil

		// Close any previously open content block before creating the synthetic thinking block.
		if s.hasTextContentStarted {
			s.hasTextContentStarted = false

			if err := s.enqueEvent(&StreamEvent{
				Type:  "content_block_stop",
				Index: &s.contentIndex,
			}); err != nil {
				return fmt.Errorf("failed to enqueue content_block_stop for text before pending signature: %w", err)
			}

			s.contentIndex += 1
		}

		if s.hasToolContentStarted {
			s.hasToolContentStarted = false

			if err := s.enqueEvent(&StreamEvent{
				Type:  "content_block_stop",
				Index: &s.contentIndex,
			}); err != nil {
				return fmt.Errorf("failed to enqueue content_block_stop for tool before pending signature: %w", err)
			}

			s.contentIndex += 1
		}

		if err := s.enqueEvent(&StreamEvent{
			Type:  "content_block_start",
			Index: &s.contentIndex,
			ContentBlock: &MessageContentBlock{
				Type:     "thinking",
				Thinking: lo.ToPtr(""),
			},
		}); err != nil {
			return fmt.Errorf("failed to enqueue thinking content_block_start for pending signature: %w", err)
		}

		if err := s.enqueEvent(&StreamEvent{
			Type:  "content_block_delta",
			Index: &s.contentIndex,
			Delta: &StreamDelta{
				Type:      lo.ToPtr("signature_delta"),
				Signature: sig,
			},
		}); err != nil {
			return fmt.Errorf("failed to enqueue signature_delta for pending signature: %w", err)
		}

		if err := s.enqueEvent(&StreamEvent{
			Type:  "content_block_stop",
			Index: &s.contentIndex,
		}); err != nil {
			return fmt.Errorf("failed to enqueue content_block_stop for pending signature: %w", err)
		}

		s.contentIndex += 1

		return nil
	}

	if s.hasThinkingContentStarted {
		s.hasThinkingContentStarted = false

		// Use pending signature if available, otherwise generate a random one.
		sig := s.pendingSignature
		s.pendingSignature = nil
		if sig == nil {
			rs := generateSignature()
			sig = &rs
		}

		if err := s.enqueEvent(&StreamEvent{
			Type:  "content_block_delta",
			Index: &s.contentIndex,
			Delta: &StreamDelta{
				Type:      lo.ToPtr("signature_delta"),
				Signature: sig,
			},
		}); err != nil {
			return fmt.Errorf("failed to enqueue signature_delta event: %w", err)
		}

		if err := s.enqueEvent(&StreamEvent{
			Type:  "content_block_stop",
			Index: &s.contentIndex,
		}); err != nil {
			return fmt.Errorf("failed to enqueue content_block_stop event: %w", err)
		}

		s.contentIndex += 1
	}

	return nil
}

func (s *anthropicInboundStream) enqueEvent(ev *StreamEvent) error {
	// Some providers have a bug that generates duplicate "content_block_stop" events. This check ignores the duplicate to ensure compatibility.
	if s.lastEventType == "content_block_stop" && ev.Type == "content_block_stop" {
		return nil
	}

	s.lastEventType = ev.Type

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
		if chunk.Usage != nil {
			usage = convertToAnthropicUsage(chunk.Usage)
		}

		streamEvent := StreamEvent{
			Type: "message_start",
			Message: &StreamMessage{
				ID:      s.messageID,
				Type:    "message",
				Role:    "assistant",
				Model:   s.model,
				Content: []MessageContentBlock{},
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
					ContentBlock: &MessageContentBlock{
						Type:     "thinking",
						Thinking: lo.ToPtr(""),
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

		// Buffer signature: always defer emission to closeThinkingBlock so that
		// we emit exactly one signature_delta per thinking block (avoiding
		// duplicates when a random placeholder would otherwise be generated).
		// If multiple signature chunks arrive, concatenate them to match the
		// aggregator's behavior.
		if choice.Delta != nil && choice.Delta.ReasoningSignature != nil && *choice.Delta.ReasoningSignature != "" {
			if s.pendingSignature == nil {
				s.pendingSignature = choice.Delta.ReasoningSignature
			} else {
				combined := *s.pendingSignature + *choice.Delta.ReasoningSignature
				s.pendingSignature = &combined
			}
		}

		// Handle redacted reasoning content (redacted_thinking)
		if choice.Delta != nil && choice.Delta.RedactedReasoningContent != nil && *choice.Delta.RedactedReasoningContent != "" {
			if err := s.closeThinkingBlock(); err != nil {
				s.err = fmt.Errorf("failed to close thinking block: %w", err)
				return false
			}

			// If the tool content has started before the redacted thinking content, we need to stop it
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

			// If the text content has started before the redacted thinking content, we need to stop it
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

			// Generate content_block_start for redacted_thinking
			// Redacted thinking blocks come complete in content_block_start with their Data field already populated
			err := s.enqueEvent(&StreamEvent{
				Type:  "content_block_start",
				Index: &s.contentIndex,
				ContentBlock: &MessageContentBlock{
					Type: "redacted_thinking",
					Data: *choice.Delta.RedactedReasoningContent,
				},
			})
			if err != nil {
				s.err = fmt.Errorf("failed to enqueue redacted_thinking content_block_start event: %w", err)
				return false
			}

			// Generate content_block_stop for redacted_thinking immediately
			err = s.enqueEvent(&StreamEvent{
				Type:  "content_block_stop",
				Index: &s.contentIndex,
			})
			if err != nil {
				s.err = fmt.Errorf("failed to enqueue redacted_thinking content_block_stop event: %w", err)
				return false
			}

			s.contentIndex += 1
		}

		// Handle content delta
		if choice.Delta != nil && choice.Delta.Content.Content != nil && *choice.Delta.Content.Content != "" {
			if err := s.closeThinkingBlock(); err != nil {
				s.err = fmt.Errorf("failed to close thinking block: %w", err)
				return false
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
					ContentBlock: &MessageContentBlock{
						Type: "text",
						Text: lo.ToPtr(""),
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
			if err := s.closeThinkingBlock(); err != nil {
				s.err = fmt.Errorf("failed to close thinking block: %w", err)
				return false
			}

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

					s.hasToolContentStarted = true
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
						ContentBlock: &MessageContentBlock{
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

					// If the tool call has arguments, we need to generate a content_block_delta.
					if deltaToolCall.Function.Arguments != "" {
						s.toolCalls[toolCallIndex].Function.Arguments += deltaToolCall.Function.Arguments

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

			if err := s.closeThinkingBlock(); err != nil {
				s.err = fmt.Errorf("failed to close thinking block: %w", err)
				return false
			}

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
	}

	if chunk.Usage != nil && s.hasFinished && !s.messageStoped {
		// Usage-only chunk after finish_reason - generate message_delta with both stop reason and usage
		streamEvent := StreamEvent{
			Type: "message_delta",
		}

		if s.stopReason != nil {
			streamEvent.Delta = &StreamDelta{
				StopReason: s.stopReason,
			}
		}

		streamEvent.Usage = convertToAnthropicUsage(chunk.Usage)

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

		s.messageStoped = true
	}

	// Continue to the next event.
	return s.Next()
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
