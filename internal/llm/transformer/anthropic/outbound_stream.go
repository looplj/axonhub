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

func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	// Filter out unnecessary stream events to optimize performance
	filteredStream := streams.Filter(stream, filterStreamEvent)

	doneEvent := lo.ToPtr(llm.DoneStreamEvent)
	// Append the DONE event to the filtered stream
	streamWithDone := streams.AppendStream(filteredStream, doneEvent)

	return streams.NoNil(newOutboundStream(streamWithDone)), nil
}

// filterStreamEvent determines if a stream event should be processed
// Filters out unnecessary events like ping, content_block_start, and content_block_stop.
func filterStreamEvent(event *httpclient.StreamEvent) bool {
	if event == nil || len(event.Data) == 0 {
		return false
	}

	// Only process events that contribute to the OpenAI response format
	switch event.Type {
	case "message_start", "content_block_start", "content_block_delta", "message_delta", "message_stop":
		return true
	case "ping", "content_block_stop":
		return false // Skip these events as they're not needed for OpenAI format
	default:
		return false // Skip unknown event types
	}
}

// streamState holds the state for a streaming session.
type streamState struct {
	streamID    string
	streamModel string
	streamUsage *llm.Usage
	// Tool call tracking
	toolIndex int
	toolCalls map[int]*llm.ToolCall // index -> tool call
}

// outboundStream wraps a stream and maintains state during processing.
type outboundStream struct {
	stream  streams.Stream[*httpclient.StreamEvent]
	state   *streamState
	current *llm.Response
	err     error
}

func newOutboundStream(stream streams.Stream[*httpclient.StreamEvent]) *outboundStream {
	return &outboundStream{
		stream: stream,
		state: &streamState{
			toolCalls: make(map[int]*llm.ToolCall),
			toolIndex: -1,
		},
	}
}

func (s *outboundStream) Next() bool {
	if s.stream.Next() {
		event := s.stream.Current()

		resp, err := s.transformStreamChunk(event)
		if err != nil {
			s.err = err
			return false
		}

		s.current = resp

		return true
	}

	return false
}

// transformStreamChunk transforms a single Anthropic streaming chunk to ChatCompletionResponse with state.
//
//nolint:maintidx // Checked.
func (s *outboundStream) transformStreamChunk(event *httpclient.StreamEvent) (*llm.Response, error) {
	if event == nil {
		return nil, fmt.Errorf("stream event is nil")
	}

	if len(event.Data) == 0 {
		return nil, fmt.Errorf("event data is empty")
	}

	// Handle DONE event specially
	if string(event.Data) == "[DONE]" {
		return llm.DoneResponse, nil
	}

	if event.Type == "error" {
		return nil, &llm.ResponseError{
			Detail: llm.ErrorDetail{
				Message: fmt.Sprintf("received error while streaming: %s", string(event.Data)),
			},
		}
	}

	state := s.state

	// Parse the streaming event
	var streamEvent StreamEvent

	err := json.Unmarshal(event.Data, &streamEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal anthropic stream event: %w", err)
	}

	// Convert the stream event to ChatCompletionResponse
	resp := &llm.Response{
		Object: "chat.completion.chunk",
		ID:     state.streamID,    // Use stored ID from message_start
		Model:  state.streamModel, // Use stored model from message_start
	}

	switch streamEvent.Type {
	case "message_start":
		if streamEvent.Message != nil {
			// Store ID, model, and usage for subsequent events
			state.streamID = streamEvent.Message.ID
			state.streamModel = streamEvent.Message.Model

			// Update response with stored values
			resp.ID = state.streamID
			resp.Model = state.streamModel

			if streamEvent.Message.Usage != nil {
				state.streamUsage = lo.ToPtr(convertUsage(*streamEvent.Message.Usage))
				resp.Usage = state.streamUsage
			}

			resp.Created = 0
			if streamEvent.Message.Usage != nil {
				resp.ServiceTier = streamEvent.Message.Usage.ServiceTier
			}
		}
		// For message_start, we return an empty choice to indicate the start
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
				},
			},
		}

	case "content_block_start":
		// Only process tool_use content blocks, skip text content blocks
		if streamEvent.ContentBlock != nil && streamEvent.ContentBlock.Type == "tool_use" {
			// Initialize a new tool call
			state.toolIndex++
			toolCall := llm.ToolCall{
				Index: state.toolIndex,
				ID:    streamEvent.ContentBlock.ID,
				Type:  "function",
				Function: llm.FunctionCall{
					Name:      *streamEvent.ContentBlock.Name,
					Arguments: "",
				},
			}
			state.toolCalls[state.toolIndex] = &toolCall

			choice := llm.Choice{
				Index: 0,
				Delta: &llm.Message{
					Role:      "assistant",
					ToolCalls: []llm.ToolCall{toolCall},
				},
			}
			resp.Choices = []llm.Choice{choice}
		} else {
			//nolint:nilnil // It is expected.
			return nil, nil
		}

	case "content_block_delta":
		if streamEvent.Delta != nil {
			// Handle tool use deltas (input_json_delta)
			if streamEvent.Delta.PartialJSON != nil {
				choice := llm.Choice{
					Index: 0,
					Delta: &llm.Message{
						Role: "assistant",
						ToolCalls: []llm.ToolCall{
							{
								Index: state.toolIndex,
								ID:    state.toolCalls[state.toolIndex].ID,
								Type:  "function",
								Function: llm.FunctionCall{
									Arguments: *streamEvent.Delta.PartialJSON,
								},
							},
						},
					},
				}
				resp.Choices = []llm.Choice{choice}

				return resp, nil
			}

			choice := llm.Choice{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
				},
			}

			// Handle text content
			if streamEvent.Delta.Text != nil {
				choice.Delta.Content = llm.MessageContent{
					Content: streamEvent.Delta.Text,
				}
			}

			// Handle thinking content - map to reasoning_content
			if streamEvent.Delta.Thinking != nil {
				choice.Delta.ReasoningContent = streamEvent.Delta.Thinking
			}

			// Skip signature deltas as they're not part of the content
			if streamEvent.Delta.Signature != nil {
				//nolint:nilnil // It is expected.
				return nil, nil
			}

			resp.Choices = []llm.Choice{choice}
		}

	case "message_delta":
		// Update stored usage if available (final usage information)
		if streamEvent.Usage != nil {
			usage := convertUsage(*streamEvent.Usage)
			if state.streamUsage != nil {
				usage.PromptTokens = state.streamUsage.PromptTokens
				if state.streamUsage.PromptTokensDetails != nil {
					usage.PromptTokensDetails = state.streamUsage.PromptTokensDetails
				}
				// Recalculate total tokens
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}

			state.streamUsage = &usage
		}

		resp.Usage = state.streamUsage

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
				},
			}
		}

	case "message_stop":
		// Final event - return empty response to indicate completion
		resp.Choices = []llm.Choice{}
		// Include final usage information
		if state.streamUsage != nil {
			resp.Usage = state.streamUsage
		}

	default:
		// This should not happen due to filtering, but handle gracefully
		return nil, fmt.Errorf("unexpected stream event type: %s", streamEvent.Type)
	}

	return resp, nil
}

func (s *outboundStream) Current() *llm.Response {
	return s.current
}

func (s *outboundStream) Err() error {
	if s.err != nil {
		return s.err
	}

	return s.stream.Err()
}

func (s *outboundStream) Close() error {
	return s.stream.Close()
}
