package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
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
	stream  streams.Stream[*httpclient.StreamEvent]
	state   *outboundStreamState
	current *llm.Response
	err     error
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
	toolCalls map[string]*llm.ToolCall // callID -> tool call
}

func newResponsesOutboundStream(stream streams.Stream[*httpclient.StreamEvent]) *responsesOutboundStream {
	return &responsesOutboundStream{
		stream: stream,
		state: &outboundStreamState{
			toolCalls: make(map[string]*llm.ToolCall),
		},
	}
}

func (s *responsesOutboundStream) Next() bool {
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

// transformStreamChunk transforms a single OpenAI Responses API streaming chunk to unified llm.Response.
//
//nolint:maintidx,gocognit // It is complex and hard to split.
func (s *responsesOutboundStream) transformStreamChunk(event *httpclient.StreamEvent) (*llm.Response, error) {
	if event == nil || len(event.Data) == 0 {
		return nil, nil
	}

	// Handle [DONE] marker
	if string(event.Data) == "[DONE]" {
		return llm.DoneResponse, nil
	}

	// Parse the streaming event
	var streamEvent StreamEvent

	err := json.Unmarshal(event.Data, &streamEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal responses api stream event: %w", err)
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
	case StreamEventTypeResponseCreated, StreamEventTypeResponseInProgress:
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

	case StreamEventTypeOutputItemAdded:
		// Output item added - check type to determine how to handle
		if streamEvent.Item != nil {
			item := streamEvent.Item
			switch item.Type {
			case "function_call":
				// Initialize tool call tracking
				s.state.toolCalls[item.CallID] = &llm.ToolCall{
					ID:   item.CallID,
					Type: "function",
					Function: llm.FunctionCall{
						Name:      item.Name,
						Arguments: "",
					},
				}

				resp.Choices = []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							ToolCalls: []llm.ToolCall{
								{
									ID:   item.CallID,
									Type: "function",
									Function: llm.FunctionCall{
										Name: item.Name,
									},
								},
							},
						},
					},
				}
			default:
				// For other item types, just emit an empty delta
				resp.Choices = []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
						},
					},
				}
			}
		} else {
			// No item data, emit empty delta
			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						Role: "assistant",
					},
				},
			}
		}

	case StreamEventTypeContentPartAdded:
		// Content part added - emit empty delta
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
				},
			},
		}

	case StreamEventTypeOutputTextDelta:
		// Text content delta
		s.state.textContent.WriteString(streamEvent.Delta)

		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &streamEvent.Delta,
					},
				},
			},
		}

	case StreamEventTypeFunctionCallArgumentsDelta:
		// Function call arguments delta
		if streamEvent.ItemID != nil {
			// Try to find the tool call by item_id (which may be call_id)
			for callID, tc := range s.state.toolCalls {
				if callID == *streamEvent.ItemID || tc.ID == *streamEvent.ItemID {
					tc.Function.Arguments += streamEvent.Delta

					resp.Choices = []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Role: "assistant",
								ToolCalls: []llm.ToolCall{
									{
										ID:   tc.ID,
										Type: "function",
										Function: llm.FunctionCall{
											Arguments: streamEvent.Delta,
										},
									},
								},
							},
						},
					}

					break
				}
			}
		}

	case StreamEventTypeFunctionCallArgumentsDone:
		// Function call completed - update with final arguments
		if streamEvent.CallID != "" {
			if tc, ok := s.state.toolCalls[streamEvent.CallID]; ok {
				tc.Function.Name = streamEvent.Name
				tc.Function.Arguments = streamEvent.Arguments
			}
		}

		// Emit the complete tool call
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
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
					Role:             "assistant",
					ReasoningContent: &streamEvent.Delta,
				},
			},
		}

	case StreamEventTypeOutputTextDone:
		// Text content completed
		text := streamEvent.Text
		if text == "" {
			text = s.state.textContent.String()
		}

		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &text,
					},
				},
			},
		}

	case StreamEventTypeReasoningSummaryTextDone:
		// Reasoning content completed
		text := streamEvent.Text
		if text == "" {
			text = s.state.reasoningContent.String()
		}

		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role:             "assistant",
					ReasoningContent: &text,
				},
			},
		}

	case StreamEventTypeOutputItemDone, StreamEventTypeContentPartDone,
		StreamEventTypeReasoningSummaryPartAdded, StreamEventTypeReasoningSummaryPartDone:
		// These events don't need special handling for the unified format
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
				},
			},
		}

	case StreamEventTypeResponseCompleted:
		// Response completed
		if streamEvent.Response != nil {
			if streamEvent.Response.Usage != nil {
				s.state.usage = streamEvent.Response.Usage.ToUsage()
				resp.Usage = s.state.usage
			}
		}

		// Determine finish reason
		finishReason := "stop"
		if len(s.state.toolCalls) > 0 {
			finishReason = "tool_calls"
		}

		resp.Choices = []llm.Choice{
			{
				Index:        0,
				FinishReason: &finishReason,
			},
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
		return nil, &llm.ResponseError{
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
						Role: "assistant",
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
					Delta: &llm.Message{
						Role: "assistant",
					},
				},
			}
		}

	default:
		// Unknown event type - emit empty delta
		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
				},
			},
		}
	}

	return resp, nil
}

func (s *responsesOutboundStream) Current() *llm.Response {
	return s.current
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
