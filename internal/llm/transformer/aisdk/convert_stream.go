package aisdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// TransformStream transforms LLM response stream to AI SDK data stream protocol format.
func (t *DataStreamTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	// Create a custom stream that handles the stateful transformation
	aisdkStream := &aiSDKConvertStream{
		source:          stream,
		ctx:             ctx,
		activeToolCalls: make(map[string]*llm.ToolCall),
	}
	doneEvent := lo.ToPtr(llm.DoneStreamEvent)
	// Append the DONE event to the filtered stream
	streamWithDone := streams.AppendStream(aisdkStream, doneEvent)

	return streams.NoNil(streamWithDone), nil
}

// aiSDKConvertStream implements the stateful stream transformation for AI SDK data stream protocol.
//
//nolint:containedctx // Checked.
type aiSDKConvertStream struct {
	source      streams.Stream[*llm.Response]
	ctx         context.Context
	hasStarted  bool
	hasFinished bool
	messageID   string
	eventQueue  []*httpclient.StreamEvent
	queueIndex  int
	err         error

	// State tracking for content blocks
	hasTextContentStarted      bool
	hasReasoningContentStarted bool
	hasToolContentStarted      bool
	currentTextID              string
	currentReasoningID         string
	activeToolCalls            map[string]*llm.ToolCall // Track tool calls by ID
}

func (s *aiSDKConvertStream) enqueueEvent(_ string, data interface{}) error {
	eventData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	s.eventQueue = append(s.eventQueue, &httpclient.StreamEvent{
		Type: "", // Set to empty string to avoid sending type.
		Data: eventData,
	})

	return nil
}

//nolint:maintidx // Complex stream processing logic
func (s *aiSDKConvertStream) Next() bool {
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

	// Initialize message ID from first chunk
	if s.messageID == "" && chunk.ID != "" {
		s.messageID = chunk.ID
	}

	// Generate start event if this is the first chunk
	if !s.hasStarted {
		s.hasStarted = true

		startEvent := StreamEvent{
			Type:      "start",
			MessageID: s.messageID,
		}

		err := s.enqueueEvent("start", startEvent)
		if err != nil {
			s.err = fmt.Errorf("failed to enqueue start event: %w", err)
			return false
		}
	}

	// Process the current chunk
	if len(chunk.Choices) > 0 {
		choice := chunk.Choices[0]

		// Handle reasoning content (thinking) delta
		if choice.Delta != nil && choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			// If tool content has started, stop it first
			if s.hasToolContentStarted {
				if err := s.endToolContent(); err != nil {
					s.err = err
					return false
				}
			}

			// If text content has started, stop it first
			if s.hasTextContentStarted {
				if err := s.endTextContent(); err != nil {
					s.err = err
					return false
				}
			}

			// Start reasoning content if not already started
			if !s.hasReasoningContentStarted {
				if err := s.startReasoningContent(); err != nil {
					s.err = err
					return false
				}
			}

			// Reasoning delta
			reasoningDelta := StreamEvent{
				Type:  "reasoning-delta",
				ID:    s.currentReasoningID,
				Delta: *choice.Delta.ReasoningContent,
			}
			if err := s.enqueueEvent("reasoning-delta", reasoningDelta); err != nil {
				s.err = fmt.Errorf("failed to enqueue reasoning-delta event: %w", err)
				return false
			}
		}

		// Handle text content delta
		if choice.Delta != nil && choice.Delta.Content.Content != nil && *choice.Delta.Content.Content != "" {
			// If reasoning content has started, stop it first
			if s.hasReasoningContentStarted {
				if err := s.endReasoningContent(); err != nil {
					s.err = err
					return false
				}
			}

			// If tool content has started, stop it first
			if s.hasToolContentStarted {
				if err := s.endToolContent(); err != nil {
					s.err = err
					return false
				}
			}

			// Start text content if not already started
			if !s.hasTextContentStarted {
				if err := s.startTextContent(); err != nil {
					s.err = err
					return false
				}
			}

			// Text delta
			textDelta := StreamEvent{
				Type:  "text-delta",
				ID:    s.currentTextID,
				Delta: *choice.Delta.Content.Content,
			}
			if err := s.enqueueEvent("text-delta", textDelta); err != nil {
				s.err = fmt.Errorf("failed to enqueue text-delta event: %w", err)
				return false
			}
		}

		// Handle tool calls
		if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
			// If text content has started, stop it first
			if s.hasTextContentStarted {
				if err := s.endTextContent(); err != nil {
					s.err = err
					return false
				}
			}

			// If reasoning content has started, stop it first
			if s.hasReasoningContentStarted {
				if err := s.endReasoningContent(); err != nil {
					s.err = err
					return false
				}
			}

			for _, deltaToolCall := range choice.Delta.ToolCalls {
				toolCallID := deltaToolCall.ID
				if toolCallID == "" {
					toolCallID = generateID("tool")
				}

				// Initialize tool call if it doesn't exist
				if _, exists := s.activeToolCalls[toolCallID]; !exists {
					s.activeToolCalls[toolCallID] = &llm.ToolCall{
						ID:   toolCallID,
						Type: deltaToolCall.Type,
						Function: llm.FunctionCall{
							Name:      deltaToolCall.Function.Name,
							Arguments: "",
						},
					}

					// Start tool content if not already started
					if !s.hasToolContentStarted {
						s.hasToolContentStarted = true
					}

					// Tool input start
					toolInputStart := StreamEvent{
						Type:       "tool-input-start",
						ToolCallID: toolCallID,
						ToolName:   deltaToolCall.Function.Name,
					}
					if err := s.enqueueEvent("tool-input-start", toolInputStart); err != nil {
						s.err = fmt.Errorf("failed to enqueue tool-input-start event: %w", err)
						return false
					}
				}

				// Update arguments
				if deltaToolCall.Function.Arguments != "" {
					s.activeToolCalls[toolCallID].Function.Arguments += deltaToolCall.Function.Arguments

					// Tool input delta
					toolInputDelta := StreamEvent{
						Type:           "tool-input-delta",
						ToolCallID:     toolCallID,
						InputTextDelta: deltaToolCall.Function.Arguments,
					}
					if err := s.enqueueEvent("tool-input-delta", toolInputDelta); err != nil {
						s.err = fmt.Errorf("failed to enqueue tool-input-delta event: %w", err)
						return false
					}
				}
			}
		}

		// Handle complete tool calls (tool-input-available)
		if choice.Message != nil && len(choice.Message.ToolCalls) > 0 {
			for _, toolCall := range choice.Message.ToolCalls {
				var input interface{}
				if toolCall.Function.Arguments != "" {
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
						s.err = fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
						return false
					}
				}

				inputData, err := json.Marshal(input)
				if err != nil {
					s.err = fmt.Errorf("failed to marshal tool input: %w", err)
					return false
				}

				// Tool input available
				toolInputAvailable := StreamEvent{
					Type:       "tool-input-available",
					ToolCallID: toolCall.ID,
					ToolName:   toolCall.Function.Name,
					Input:      json.RawMessage(inputData),
				}
				if err := s.enqueueEvent("data", toolInputAvailable); err != nil {
					s.err = fmt.Errorf("failed to enqueue tool-input-available event: %w", err)
					return false
				}
			}
		}

		// Handle finish reason
		if choice.FinishReason != nil && !s.hasFinished {
			s.hasFinished = true

			// End any active content blocks
			if s.hasTextContentStarted {
				if err := s.endTextContent(); err != nil {
					s.err = err
					return false
				}
			}

			if s.hasReasoningContentStarted {
				if err := s.endReasoningContent(); err != nil {
					s.err = err
					return false
				}
			}

			if s.hasToolContentStarted {
				if err := s.endToolContent(); err != nil {
					s.err = err
					return false
				}
			}

			// Finish message
			finish := StreamEvent{
				Type: "finish",
			}
			if err := s.enqueueEvent("finish", finish); err != nil {
				s.err = fmt.Errorf("failed to enqueue finish event: %w", err)
				return false
			}
		}
	}

	// If we have events in the queue, return true
	return len(s.eventQueue) > 0
}

func (s *aiSDKConvertStream) Current() *httpclient.StreamEvent {
	if s.queueIndex < len(s.eventQueue) {
		event := s.eventQueue[s.queueIndex]
		s.queueIndex++

		return event
	}

	return nil
}

func (s *aiSDKConvertStream) Err() error {
	if s.err != nil {
		return s.err
	}

	return s.source.Err()
}

func (s *aiSDKConvertStream) Close() error {
	return s.source.Close()
}

// Helper methods for content block lifecycle management

func (s *aiSDKConvertStream) startStep() error {
	startStep := StreamEvent{
		Type: "start-step",
	}

	return s.enqueueEvent("start-step", startStep)
}

func (s *aiSDKConvertStream) finishStep() error {
	finishStep := StreamEvent{
		Type: "finish-step",
	}

	return s.enqueueEvent("finish-step", finishStep)
}

func (s *aiSDKConvertStream) startTextContent() error {
	if err := s.startStep(); err != nil {
		return err
	}

	s.hasTextContentStarted = true
	s.currentTextID = generateID("text")

	textStart := StreamEvent{
		Type: "text-start",
		ID:   s.currentTextID,
	}

	return s.enqueueEvent("text-start", textStart)
}

func (s *aiSDKConvertStream) endTextContent() error {
	if !s.hasTextContentStarted {
		return nil
	}

	s.hasTextContentStarted = false

	textEnd := StreamEvent{
		Type: "text-end",
		ID:   s.currentTextID,
	}

	if err := s.enqueueEvent("text-end", textEnd); err != nil {
		return err
	}

	if err := s.finishStep(); err != nil {
		return err
	}

	return nil
}

func (s *aiSDKConvertStream) startReasoningContent() error {
	if err := s.startStep(); err != nil {
		return err
	}

	s.hasReasoningContentStarted = true
	s.currentReasoningID = generateID("reasoning")

	reasoningStart := StreamEvent{
		Type: "reasoning-start",
		ID:   s.currentReasoningID,
	}
	if err := s.enqueueEvent("reasoning-start", reasoningStart); err != nil {
		return err
	}

	return nil
}

func (s *aiSDKConvertStream) endReasoningContent() error {
	if !s.hasReasoningContentStarted {
		return nil
	}

	s.hasReasoningContentStarted = false

	reasoningEnd := StreamEvent{
		Type: "reasoning-end",
		ID:   s.currentReasoningID,
	}
	if err := s.enqueueEvent("reasoning-end", reasoningEnd); err != nil {
		return err
	}

	if err := s.finishStep(); err != nil {
		return err
	}

	return nil
}

func (s *aiSDKConvertStream) endToolContent() error {
	if !s.hasToolContentStarted {
		return nil
	}

	s.hasToolContentStarted = false
	// Tool content doesn't need explicit end events as they are handled per tool call

	if err := s.finishStep(); err != nil {
		return err
	}

	return nil
}

// generateID generates a unique ID with the given prefix.
func generateID(prefix string) string {
	return prefix + "_" + strings.ReplaceAll(uuid.New().String(), "-", "")
}
