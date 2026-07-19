package responses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// ErrStreamIncomplete is returned when the stream ends without a terminal event
// (response.completed, response.failed, response.cancelled, or response.incomplete).
var ErrStreamIncomplete = errors.New("stream ended without terminal event")

// TransformStream transforms OpenAI Responses API SSE events to unified llm.Response stream.
func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	req *httpclient.Request,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	// Append the DONE event to the stream
	doneEvent := lo.ToPtr(llm.DoneStreamEvent)
	streamWithDone := streams.AppendStream(stream, doneEvent)

	return streams.NoNil(newResponsesOutboundStream(streamWithDone)), nil
}

// responsesOutboundStream wraps a stream and maintains state during processing.
type responsesOutboundStream struct {
	stream     streams.Stream[*httpclient.StreamEvent]
	state      *outboundStreamState
	aggregator *streamAggregator

	// Event queue
	eventQueue []*llm.Response
	queueIndex int
	err        error

	// Track whether the response completed successfully
	responseCompleted bool
}

// outboundStreamState holds the state for a streaming session.
type outboundStreamState struct {
	responseID         string
	responseModel      string
	previousResponseID *string
	usage              *llm.Usage
	created            int64

	// Reasoning signature tracking
	pendingReasoningEncryptedContent map[string]*string

	// Transformer metadata tracking
	transformerMetadata        map[string]any
	transformerMetadataEmitted bool
}

func newResponsesOutboundStream(stream streams.Stream[*httpclient.StreamEvent]) *responsesOutboundStream {
	return &responsesOutboundStream{
		stream:     stream,
		aggregator: newStreamAggregator(),
		state: &outboundStreamState{
			pendingReasoningEncryptedContent: make(map[string]*string),
			transformerMetadata:              make(map[string]any),
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
		// Stream ended - check if we received a terminal event
		// If not, this is an incomplete stream (e.g., upstream EOF)
		if s.err == nil && !s.responseCompleted && s.stream.Err() == nil {
			// Only set this error if we had started receiving response data
			// This distinguishes between "no response" and "incomplete response"
			if s.state.responseID != "" {
				s.err = ErrStreamIncomplete
			}
		}
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

	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.DebugContext(context.Background(), "received response stream event", slog.Any("event", streamEvent))
	}
	if s.responseCompleted {
		return nil
	}
	s.aggregator.processEvent(&streamEvent)
	if s.aggregator.err != nil {
		return s.aggregator.err
	}

	// Build base response
	resp := &llm.Response{
		Object:             "chat.completion.chunk",
		ID:                 s.state.responseID,
		Model:              s.state.responseModel,
		Created:            s.state.created,
		PreviousResponseID: s.state.previousResponseID,
	}

	//nolint:exhaustive //Only process events we care about.
	switch streamEvent.Type {
	case StreamEventTypeResponseCreated:
		if streamEvent.Response != nil {
			s.state.responseID = streamEvent.Response.ID
			s.state.responseModel = streamEvent.Response.Model
			s.state.created = streamEvent.Response.CreatedAt
			s.state.previousResponseID = streamEvent.Response.PreviousResponseID

			resp.ID = s.state.responseID
			resp.Model = s.state.responseModel
			resp.Created = s.state.created
			resp.PreviousResponseID = s.state.previousResponseID

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
			s.state.previousResponseID = streamEvent.Response.PreviousResponseID

			if streamEvent.Response.Usage != nil {
				s.state.usage = streamEvent.Response.Usage.ToUsage()
			}
		}

		return nil // Intentionally skip this event
	case StreamEventTypeOutputItemAdded:
		// Output item added - check type to determine how to handle
		if streamEvent.Item == nil {
			// No item data, skip
			return nil // Intentionally skip this event
		}

		item := streamEvent.Item
		switch item.Type {
		case "reasoning":
			if item.ID == "" || item.EncryptedContent == nil || *item.EncryptedContent == "" {
				return nil // Intentionally skip this event
			}

			// Responses streams may send a provisional encrypted_content on item.added
			// and the final blob on item.done. Hold the value until item.done so the
			// final blob replaces the provisional one instead of being concatenated.
			s.state.pendingReasoningEncryptedContent[item.ID] = shared.EncodeOpenAIEncryptedContent(item.EncryptedContent)
			return nil

		case "function_call", "custom_tool_call":
			return nil

		default:
			// For other item types (e.g., message), skip - no meaningful content to emit
			return nil // Intentionally skip this event
		}

	case StreamEventTypeFunctionCallArgumentsDelta,
		StreamEventTypeFunctionCallArgumentsDone,
		StreamEventTypeCustomToolCallInputDelta,
		StreamEventTypeCustomToolCallInputDone:
		// streamAggregator owns tool identity and terminal payload reconciliation.
		return nil

	case StreamEventTypeContentPartAdded:
		// Content part added - skip, no meaningful content to emit
		return nil // Intentionally skip this event

	case StreamEventTypeOutputTextDelta:
		// Text content delta
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

	case StreamEventTypeOutputItemDone:
		if streamEvent.Item == nil {
			return nil // Intentionally skip this event
		}
		if streamEvent.Item.Type == "function_call" || streamEvent.Item.Type == "custom_tool_call" {
			return nil
		}
		if streamEvent.Item.Type == "web_search_call" {
			appendResponseWebSearchCallMetadata(s.state.transformerMetadata, *streamEvent.Item)
			return nil // Intentionally skip this event
		}
		if streamEvent.Item.Type == "reasoning" {
			if streamEvent.Item.ID == "" {
				return nil // Intentionally skip this event
			}

			encryptedContent := shared.EncodeOpenAIEncryptedContent(streamEvent.Item.EncryptedContent)
			if encryptedContent == nil || *encryptedContent == "" {
				encryptedContent = s.state.pendingReasoningEncryptedContent[streamEvent.Item.ID]
			}
			delete(s.state.pendingReasoningEncryptedContent, streamEvent.Item.ID)
			if encryptedContent == nil || *encryptedContent == "" {
				return nil // Intentionally skip this event
			}

			resp.TransformerMetadata = map[string]any{
				responsesReasoningItemTransformerMetadataKey: map[string]any{
					"id":   streamEvent.Item.ID,
					"done": true,
				},
			}
			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						ReasoningSignature: encryptedContent,
					},
				},
			}
			break
		}
		if streamEvent.Item.Type != "message" {
			return nil // Intentionally skip this event
		}

		msg := convertOutputToMessage([]Item{*streamEvent.Item}, s.state.transformerMetadata)
		if len(msg.Annotations) == 0 {
			return nil // Intentionally skip this event
		}
		if len(s.state.transformerMetadata) > 0 {
			resp.TransformerMetadata = s.state.transformerMetadata
			s.state.transformerMetadataEmitted = true
		}

		resp.Choices = []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Annotations: msg.Annotations,
				},
			},
		}

	case StreamEventTypeContentPartDone,
		StreamEventTypeReasoningSummaryPartAdded, StreamEventTypeReasoningSummaryPartDone:
		// These events don't need special handling - skip
		return nil // Intentionally skip this event

	case StreamEventTypeResponseCompleted:
		// Response completed - emit two events: one with finish_reason, one with usage
		if !s.beginTerminalEvent() {
			return nil
		}
		if streamEvent.Response != nil {
			s.state.previousResponseID = streamEvent.Response.PreviousResponseID
			resp.PreviousResponseID = s.state.previousResponseID
		}
		toolCallCount := s.enqueueAggregatedToolCalls()
		if len(s.state.transformerMetadata) > 0 && !s.state.transformerMetadataEmitted {
			resp.TransformerMetadata = s.state.transformerMetadata
			s.state.transformerMetadataEmitted = true
		}

		finishReason := "stop"
		if toolCallCount > 0 {
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
				Object:             "chat.completion.chunk",
				ID:                 s.state.responseID,
				Model:              s.state.responseModel,
				Created:            s.state.created,
				PreviousResponseID: s.state.previousResponseID,
				Choices:            []llm.Choice{},
				Usage:              s.state.usage,
			}

			s.enqueue(resp)
			s.enqueue(usageResp)

			return nil
		}

	case StreamEventTypeResponseFailed:
		// Response failed
		if !s.beginTerminalEvent() {
			return nil
		}
		finishReason := "error"
		resp.Choices = []llm.Choice{
			{
				Index:        0,
				FinishReason: &finishReason,
			},
		}

	case StreamEventTypeResponseIncomplete:
		// Response incomplete (e.g., max tokens)
		if !s.beginTerminalEvent() {
			return nil
		}
		finishReason := "length"
		resp.Choices = []llm.Choice{
			{
				Index:        0,
				FinishReason: &finishReason,
			},
		}

	case StreamEventTypeResponseCancelled:
		// Response cancelled
		if !s.beginTerminalEvent() {
			return nil
		}
		finishReason := "cancelled"
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
				Param:   lo.FromPtr(streamEvent.Param),
			},
		}

	case StreamEventTypeImageGenerationPartialImage,
		StreamEventTypeImageGenerationGenerating,
		StreamEventTypeImageGenerationInProgress,
		StreamEventTypeImageGenerationCompleted:
		// Handle image generation events
		if streamEvent.PartialImageB64 != "" {
			imageURL := xurl.BuildDataURL("image/png", streamEvent.PartialImageB64, true)
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

func (s *responsesOutboundStream) beginTerminalEvent() bool {
	if s.responseCompleted {
		return false
	}
	s.responseCompleted = true
	return true
}

func (s *responsesOutboundStream) enqueueAggregatedToolCalls() int {
	toolCalls := s.aggregator.completedToolCalls()
	for index, aggregated := range toolCalls {
		message := convertOutputToMessage([]Item{aggregated.Item}, nil)
		if len(message.ToolCalls) != 1 {
			continue
		}

		toolCall := message.ToolCalls[0]
		toolCall.Index = index
		finalPayload := toolCall.Function.Arguments
		if toolCall.ResponseCustomToolCall != nil {
			finalPayload = toolCall.ResponseCustomToolCall.Input
		}

		deltas := reconcileToolPayload(aggregated.Deltas, finalPayload)
		identity := toolCall
		if len(aggregated.Deltas) > 0 {
			if identity.ResponseCustomToolCall != nil {
				customCall := *identity.ResponseCustomToolCall
				customCall.Input = ""
				identity.ResponseCustomToolCall = &customCall
			} else {
				identity.Function.Arguments = ""
			}
		}
		s.enqueueToolCallChunk(identity)

		for _, delta := range deltas {
			toolCallDelta := llm.ToolCall{Index: index}
			if toolCall.ResponseCustomToolCall != nil {
				toolCallDelta.Type = llm.ToolTypeResponsesCustomTool
				toolCallDelta.ResponseCustomToolCall = &llm.ResponseCustomToolCall{
					CallID:    toolCall.ID,
					Name:      toolCall.ResponseCustomToolCall.Name,
					Namespace: toolCall.ResponseCustomToolCall.Namespace,
					Input:     delta,
				}
			} else {
				toolCallDelta.Function.Arguments = delta
			}
			s.enqueueToolCallChunk(toolCallDelta)
		}
	}

	return len(toolCalls)
}

func reconcileToolPayload(deltas []string, finalPayload string) []string {
	if len(deltas) == 0 {
		return nil
	}

	streamedPayload := strings.Join(deltas, "")
	switch {
	case finalPayload == streamedPayload:
		return deltas
	case strings.HasPrefix(finalPayload, streamedPayload):
		if suffix := finalPayload[len(streamedPayload):]; suffix != "" {
			return append(append([]string(nil), deltas...), suffix)
		}
		return deltas
	default:
		return []string{finalPayload}
	}
}

func (s *responsesOutboundStream) enqueueToolCallChunk(toolCall llm.ToolCall) {
	resp := s.newResponseChunk()
	resp.Choices = []llm.Choice{{
		Index: 0,
		Delta: &llm.Message{ToolCalls: []llm.ToolCall{toolCall}},
	}}
	s.enqueue(resp)
}

func (s *responsesOutboundStream) newResponseChunk() *llm.Response {
	return &llm.Response{
		Object:             "chat.completion.chunk",
		ID:                 s.state.responseID,
		Model:              s.state.responseModel,
		Created:            s.state.created,
		PreviousResponseID: s.state.previousResponseID,
	}
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
	ctx context.Context, _ *httpclient.Request,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}
