package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// TransformStream transforms OpenAI Responses API SSE events to unified llm.Response stream.
func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	// Append the DONE event to the stream
	doneEvent := lo.ToPtr(llm.DoneStreamEvent)
	streamWithDone := streams.AppendStream(stream, doneEvent)

	return streams.NoNil(newResponsesOutboundStream(streamWithDone)), nil
}

// responsesOutboundStream wraps a stream and maintains state during processing.
type responsesOutboundStream struct {
	stream streams.Stream[*httpclient.StreamEvent]
	state  *outboundStreamState

	// Event queue
	eventQueue []*llm.Response
	queueIndex int
	err        error
}

// outboundStreamState holds the state for a streaming session.
type outboundStreamState struct {
	responseID    string
	responseModel string
	usage         *llm.Usage
	created       int64

	// Content accumulation
	textContent      strings.Builder
	reasoningContent strings.Builder

	// Tool call tracking
	toolCalls     map[string]*llm.ToolCall // callID -> tool call
	itemToCallID  map[string]string        // item.id -> call_id mapping
	toolCallIndex map[string]int           // callID -> index in the output
}

func newResponsesOutboundStream(stream streams.Stream[*httpclient.StreamEvent]) *responsesOutboundStream {
	return &responsesOutboundStream{
		stream: stream,
		state: &outboundStreamState{
			toolCalls:     make(map[string]*llm.ToolCall),
			itemToCallID:  make(map[string]string),
			toolCallIndex: make(map[string]int),
		},
	}
}

func (s *responsesOutboundStream) enqueue(resp *llm.Response) {
	s.eventQueue = append(s.eventQueue, resp)
}

func (s *responsesOutboundStream) Next() bool {
	// If we have events in the queue, return them first
	if s.queueIndex < len(s.eventQueue) {
		return true
	}

	// Clear the queue and reset index for new events
	s.eventQueue = nil
	s.queueIndex = 0

	// Try to get the next chunk from source
	if !s.stream.Next() {
		return false
	}

	event := s.stream.Current()

	err := s.transformStreamChunk(event)
	if err != nil {
		s.err = err
		return false
	}

	// Continue to the next event if no events were enqueued
	return s.Next()
}

// transformStreamChunk transforms a single OpenAI Responses API streaming chunk to unified llm.Response.
// Events are enqueued via s.enqueue() instead of being returned.
//
//nolint:maintidx,gocognit // It is complex and hard to split.
func (s *responsesOutboundStream) transformStreamChunk(event *httpclient.StreamEvent) error {
	if event == nil || len(event.Data) == 0 {
		return nil
	}

	// Handle [DONE] marker
	if string(event.Data) == "[DONE]" {
		s.enqueue(llm.DoneResponse)
		return nil
	}

	// Parse the streaming event
	var streamEvent StreamEvent

	err := json.Unmarshal(event.Data, &streamEvent)
	if err != nil {
		return fmt.Errorf("failed to unmarshal responses api stream event: %w", err)
	}

	if log.DebugEnabled(context.Background()) {
		log.Debug(context.Background(), "received response stream event", log.Any("event", streamEvent))
	}

	// Build base response
	resp := &llm.Response{
		Object:  "chat.completion.chunk",
		ID:      s.state.responseID,
		Model:   s.state.responseModel,
		Created: s.state.created,
	}

	//nolint:exhaustive //Only process events we care about.
	switch streamEvent.Type {
	case StreamEventTypeResponseCreated:
		if streamEvent.Response != nil {
			s.state.responseID = streamEvent.Response.ID
			s.state.responseModel = streamEvent.Response.Model
			s.state.created = streamEvent.Response.CreatedAt

			resp.ID = s.state.responseID
			resp.Model = s.state.responseModel
			resp.Created = s.state.created

			if streamEvent.Response.Usage != nil {
				s.state.usage = streamEvent.Response.Usage.ToUsage()
				resp.Usage = s.state.usage
			}
		}

		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
				},
			},
		}

	case StreamEventTypeResponseInProgress:
		// Update state but don't emit an event
		if streamEvent.Response != nil {
			s.state.responseID = streamEvent.Response.ID
			s.state.responseModel = streamEvent.Response.Model
			s.state.created = streamEvent.Response.CreatedAt

			if streamEvent.Response.Usage != nil {
				s.state.usage = streamEvent.Response.Usage.ToUsage()
			}
		}

		return nil // Intentionally skip this event
	case StreamEventTypeOutputItemAdded:
		// Output item added - check type to determine how to handle
		if streamEvent.Item != nil {
			item := streamEvent.Item
			switch item.Type {
			case "function_call":
				// Initialize tool call tracking
				toolCallIdx := len(s.state.toolCalls)
				s.state.toolCalls[item.CallID] = &llm.ToolCall{
					ID:   item.CallID,
					Type: "function",
					Function: llm.FunctionCall{
						Name:      item.Name,
						Arguments: "",
					},
				}
				// Map item.id to call_id for later lookup
				s.state.itemToCallID[item.ID] = item.CallID
				s.state.toolCallIndex[item.CallID] = toolCallIdx

				resp.Choices = []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							ToolCalls: []llm.ToolCall{
								{
									ID:    item.CallID,
									Type:  "function",
									Index: toolCallIdx,
									Function: llm.FunctionCall{
										Name: item.Name,
									},
								},
							},
						},
					},
				}
			default:
				// For other item types (e.g., message), skip - no meaningful content to emit
				return nil // Intentionally skip this event
			}
		} else {
			// No item data, skip
			return nil // Intentionally skip this event
		}

	case StreamEventTypeFunctionCallArgumentsDelta:
		// Function call arguments delta
		if streamEvent.ItemID != nil {
			// Look up call_id from item_id mapping
			callID, ok := s.state.itemToCallID[*streamEvent.ItemID]
			if !ok {
				// Fallback: item_id might be the call_id itself
				callID = *streamEvent.ItemID
			}

			if tc, ok := s.state.toolCalls[callID]; ok {
				tc.Function.Arguments += streamEvent.Delta
				toolCallIdx := s.state.toolCallIndex[callID]

				resp.Choices = []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							ToolCalls: []llm.ToolCall{
								{
									Index: toolCallIdx,
									Function: llm.FunctionCall{
										Arguments: streamEvent.Delta,
									},
								},
							},
						},
					},
				}
			}
		}

	case StreamEventTypeFunctionCallArgumentsDone:
		// Function call completed - update state but don't emit an event
		if streamEvent.CallID != "" {
			if tc, ok := s.state.toolCalls[streamEvent.CallID]; ok {
				tc.Function.Name = streamEvent.Name
				tc.Function.Arguments = streamEvent.Arguments
			}
		}

		return nil // Intentionally skip this event

	case StreamEventTypeContentPartAdded:
		// Content part added - skip, no meaningful content to emit
		return nil // Intentionally skip this event

	case StreamEventTypeOutputTextDelta:
		// Text content delta
		s.state.textContent.WriteString(streamEvent.Delta)

		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{
						Content: &streamEvent.Delta,
					},
				},
			},
		}

	case StreamEventTypeReasoningSummaryTextDelta:
		// Reasoning content delta
		s.state.reasoningContent.WriteString(streamEvent.Delta)

		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					ReasoningContent: &streamEvent.Delta,
				},
			},
		}

	case StreamEventTypeOutputTextDone:
		// Text content completed - skip, content was already streamed via deltas
		return nil // Intentionally skip this event

	case StreamEventTypeReasoningSummaryTextDone:
		// Reasoning content completed - skip, content was already streamed via deltas
		return nil // Intentionally skip this event

	case StreamEventTypeOutputItemDone, StreamEventTypeContentPartDone,
		StreamEventTypeReasoningSummaryPartAdded, StreamEventTypeReasoningSummaryPartDone:
		// These events don't need special handling - skip
		return nil // Intentionally skip this event

	case StreamEventTypeResponseCompleted:
		// Response completed - emit two events: one with finish_reason, one with usage
		finishReason := "stop"
		if len(s.state.toolCalls) > 0 {
			finishReason = "tool_calls"
		}

		// First event: finish_reason with empty delta
		resp.Choices = []llm.Choice{
			{
				Index:        0,
				Delta:        &llm.Message{},
				FinishReason: &finishReason,
			},
		}

		// Second event: usage (if available)
		if streamEvent.Response != nil && streamEvent.Response.Usage != nil {
			s.state.usage = streamEvent.Response.Usage.ToUsage()
			usageResp := &llm.Response{
				Object:  "chat.completion.chunk",
				ID:      s.state.responseID,
				Model:   s.state.responseModel,
				Created: s.state.created,
				Choices: []llm.Choice{},
				Usage:   s.state.usage,
			}

			s.enqueue(resp)
			s.enqueue(usageResp)

			return nil
		}

	case StreamEventTypeResponseFailed:
		// Response failed
		finishReason := "error"
		resp.Choices = []llm.Choice{
			{
				Index:        0,
				FinishReason: &finishReason,
			},
		}

	case StreamEventTypeResponseIncomplete:
		// Response incomplete (e.g., max tokens)
		finishReason := "length"
		resp.Choices = []llm.Choice{
			{
				Index:        0,
				FinishReason: &finishReason,
			},
		}

	case StreamEventTypeError:
		return &llm.ResponseError{
			Detail: llm.ErrorDetail{
				Code:    streamEvent.Code,
				Message: streamEvent.Message,
				Param:   streamEvent.Param,
			},
		}

	case StreamEventTypeImageGenerationPartialImage,
		StreamEventTypeImageGenerationGenerating,
		StreamEventTypeImageGenerationInProgress,
		StreamEventTypeImageGenerationCompleted:
		// Handle image generation events
		if streamEvent.PartialImageB64 != "" {
			imageURL := "data:image/png;base64," + streamEvent.PartialImageB64
			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: imageURL,
									},
								},
							},
						},
					},
				},
			}
		} else {
			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{},
				},
			}
		}

	default:
		// Unknown event type - skip
		return nil // Intentionally skip this event
	}

	s.enqueue(resp)

	return nil
}

func (s *responsesOutboundStream) Current() *llm.Response {
	if s.queueIndex < len(s.eventQueue) {
		event := s.eventQueue[s.queueIndex]
		s.queueIndex++

		return event
	}

	return nil
}

func (s *responsesOutboundStream) Err() error {
	if s.err != nil {
		return s.err
	}

	return s.stream.Err()
}

func (s *responsesOutboundStream) Close() error {
	return s.stream.Close()
}

// AggregateStreamChunks aggregates OpenAI Responses API streaming chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}
