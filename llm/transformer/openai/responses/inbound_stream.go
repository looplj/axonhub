package responses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// ChatWrappedCustomMetadataKey marks Responses custom tool calls whose input
// was wrapped by the Responses-to-Chat tool adapter. The Chat adapter
// (producer) and the Responses inbound stream (consumer) must share this
// exported key so the wrapped metadata propagates and wrapping suppression
// stays consistent across both sides.
const ChatWrappedCustomMetadataKey = "openai_responses_chat_wrapped_custom"

// TransformStream transforms the unified llm.Response stream to OpenAI Responses API SSE events.
func (t *InboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return &responsesInboundStream{
		source:              stream,
		ctx:                 ctx,
		toolCalls:           make(map[int]*llm.ToolCall),
		transformerMetadata: make(map[string]any),
		pendingReasoning:    make(map[string][]string),
	}, nil
}

// responsesInboundStream implements the stateful stream transformation.
//
//nolint:containedctx // Checked.
type responsesInboundStream struct {
	source streams.Stream[*llm.Response]
	ctx    context.Context

	// State tracking
	hasStarted              bool
	hasResponseCreated      bool
	hasMessageItemStarted   bool
	hasReasoningItemStarted bool
	hasReasoningSummaryPart bool
	hasContentPartStarted   bool
	hasFinished             bool
	responseCompleted       bool
	finishReason            string
	pendingAnnotations      []llm.Annotation

	// Response metadata
	responseID string
	model      string
	createdAt  int64

	// Content tracking
	outputIndex    int
	contentIndex   int
	sequenceNumber int
	currentItemID  string

	// Content accumulation for items (used for emitting done events)
	accumulatedText               strings.Builder
	accumulatedReasoning          strings.Builder
	accumulatedReasoningSignature strings.Builder
	currentReasoningSourceID      string
	pendingReasoning              map[string][]string
	pendingReasoningOrder         []string

	// Tool call tracking
	toolCalls           map[int]*llm.ToolCall
	currentToolCallIdx  int
	toolCallItemStarted map[int]bool
	toolCallOutputIndex map[int]int // Maps tool call index to output index
	toolCallItemID      map[int]string
	// skipToolCallClosure suppresses closing open tool call items while
	// closeCurrentNonToolOutputItem closes only message/reasoning items.
	skipToolCallClosure bool

	// Response accumulation using streamAggregator
	usage               *llm.Usage
	aggregator          *streamAggregator
	transformerMetadata map[string]any
	echoResponse        *Response

	// Event queue
	eventQueue []*httpclient.StreamEvent
	queueIndex int
	err        error

	// Error event tracking - when true, we've already emitted an error event
	// to the client so Err() should return nil to avoid double error emission
	errorEventEmitted bool
}

func (s *responsesInboundStream) enqueueEvent(ev *StreamEvent) error {
	ev.SequenceNumber = s.sequenceNumber
	s.sequenceNumber++

	eventData, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	streamEvent := &httpclient.StreamEvent{
		Type: string(ev.Type),
		Data: eventData,
	}

	s.eventQueue = append(s.eventQueue, streamEvent)

	// Use aggregator to accumulate state for response.completed
	if s.aggregator == nil {
		s.aggregator = newStreamAggregator()
	}

	s.aggregator.processEvent(ev)

	return nil
}

//nolint:maintidx,gocognit // It is complex and hard to split.
func (s *responsesInboundStream) Next() bool {
	// If we have events in the queue, return them first
	if s.queueIndex < len(s.eventQueue) {
		return true
	}

	// Clear the queue and reset index for new events
	s.eventQueue = nil
	s.queueIndex = 0

	// Try to get the next chunk from source
	if !s.source.Next() {
		if s.err == nil && !s.errorEventEmitted && s.source.Err() == nil && !s.responseCompleted {
			if !s.hasFinished {
				// The upstream ended cleanly without a finish chunk. Close any
				// open output items and synthesize the terminal event so clients
				// do not see an abruptly truncated SSE stream.
				if !s.hasResponseCreated {
					return s.emitStreamErrorEvent(errors.New("upstream stream ended without producing any response event")) == nil
				}
				if err := s.flushPendingReasoning(); err != nil {
					s.err = err
					return false
				}
				if err := s.closeCurrentContentPart(); err != nil {
					s.err = err
					return false
				}
				if err := s.closeCurrentOutputItem(); err != nil {
					s.err = err
					return false
				}
				s.hasFinished = true
			}

			if err := s.enqueueTerminalResponse(); err != nil {
				s.err = err
				return false
			}

			return s.Next()
		}

		// Source stream ended - check if we need to emit an error event
		if s.err == nil && !s.errorEventEmitted && s.source.Err() != nil {
			sourceErr := s.source.Err()
			// Don't emit error event for client cancellation
			if errors.Is(sourceErr, context.Canceled) {
				slog.DebugContext(s.ctx, "stream canceled by client")
				return false
			}
			if errors.Is(sourceErr, context.DeadlineExceeded) {
				slog.DebugContext(s.ctx, "stream deadline exceeded")
				return false
			}
			// Emit an error event for upstream failures
			if err := s.emitStreamErrorEvent(sourceErr); err != nil {
				s.err = fmt.Errorf("failed to enqueue stream error event: %w", err)
				return false
			}

			return s.Next()
		}

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

	// Initialize response metadata from first chunk
	if s.responseID == "" && chunk.ID != "" {
		s.responseID = chunk.ID
	}

	if s.model == "" && chunk.Model != "" {
		s.model = chunk.Model
	}

	// Track createdAt
	if s.createdAt == 0 && chunk.Created != 0 {
		s.createdAt = chunk.Created
	}

	// Track usage
	if chunk.Usage != nil {
		s.usage = chunk.Usage
	}

	if len(chunk.TransformerMetadata) > 0 {
		s.mergeTransformerMetadata(chunk.TransformerMetadata)
		s.captureEchoFields(chunk.TransformerMetadata)
	}

	// Generate response.created event if this is the first chunk
	if !s.hasResponseCreated {
		s.hasResponseCreated = true

		response := &Response{
			Object:    "response",
			ID:        s.responseID,
			Model:     s.model,
			CreatedAt: s.createdAt,
			Status:    lo.ToPtr("in_progress"),
			Output:    []Item{},
		}
		applyEchoFields(response, s.echoResponse)

		if s.usage != nil {
			response.Usage = ConvertLLMUsageToResponsesUsage(s.usage)
		}
		err := s.enqueueEvent(&StreamEvent{
			Type:     StreamEventTypeResponseCreated,
			Response: response,
		})
		if err != nil {
			s.err = fmt.Errorf("failed to enqueue response.created event: %w", err)
			return false
		}

		// Also emit response.in_progress
		err = s.enqueueEvent(&StreamEvent{
			Type:     StreamEventTypeResponseInProgress,
			Response: response,
		})
		if err != nil {
			s.err = fmt.Errorf("failed to enqueue response.in_progress event: %w", err)
			return false
		}
	}

	// Process choices
	if len(chunk.Choices) > 0 {
		choice := chunk.Choices[0]

		// Handle reasoning content (thinking) delta
		if choice.Delta != nil && choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			if err := s.handleReasoningContent(choice.Delta.ReasoningContent, chunk.TransformerMetadata); err != nil {
				s.err = err
				return false
			}
		}

		// Handle encrypted reasoning content delta (stored in ReasoningSignature)
		if choice.Delta != nil && choice.Delta.ReasoningSignature != nil && *choice.Delta.ReasoningSignature != "" {
			if err := s.handleReasoningSignature(choice.Delta, chunk.TransformerMetadata); err != nil {
				s.err = err
				return false
			}
		}

		if choice.Message != nil && len(choice.Message.Annotations) > 0 {
			s.pendingAnnotations = append(s.pendingAnnotations, choice.Message.Annotations...)
		}
		if choice.Delta != nil && len(choice.Delta.Annotations) > 0 {
			s.pendingAnnotations = append(s.pendingAnnotations, choice.Delta.Annotations...)
		}

		// Handle text content delta
		if choice.Delta != nil && choice.Delta.Content.Content != nil && *choice.Delta.Content.Content != "" {
			if err := s.handleTextContent(choice.Delta.Content.Content); err != nil {
				s.err = err
				return false
			}
		}
		if choice.Delta != nil && len(choice.Delta.Content.MultipleContent) > 0 {
			if err := s.handleCompactionContent(choice.Delta.Content.MultipleContent); err != nil {
				s.err = err
				return false
			}
		}

		// Handle tool calls
		if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
			if err := s.handleToolCalls(choice.Delta.ToolCalls); err != nil {
				s.err = err
				return false
			}
		}

		// Handle finish reason
		if choice.FinishReason != nil && !s.hasFinished {
			s.hasFinished = true
			s.finishReason = *choice.FinishReason
			abnormalFinish := isAbnormalResponsesFinishReason(s.finishReason)

			// Map the Chat Completions finish_reason onto the Responses status so
			// the final response.completed event reports abnormal termination
			// (truncation, content rejection, failure) instead of always claiming success.
			switch *choice.FinishReason {
			case "length":
				s.aggregator.status = "incomplete"
				s.aggregator.incompleteDetails = &ResponseIncompleteDetails{Reason: "max_output_tokens"}
			case "content_filter":
				s.aggregator.status = "incomplete"
				s.aggregator.incompleteDetails = &ResponseIncompleteDetails{Reason: "content_filter"}
			case "error":
				s.aggregator.status = "failed"
			case "cancelled", "canceled":
				s.aggregator.status = "cancelled"
			}

			if err := s.flushPendingReasoning(); err != nil {
				s.err = err
				return false
			}

			// Close any open content parts
			if err := s.closeCurrentContentPart(); err != nil {
				s.err = err
				return false
			}

			var closeErr error
			if abnormalFinish {
				closeErr = s.closeCurrentNonToolOutputItem()
			} else {
				closeErr = s.closeCurrentOutputItem()
			}
			if closeErr != nil {
				s.err = closeErr
				return false
			}
		}
	}

	// Handle final usage chunk and complete response
	if chunk.Usage != nil && s.hasFinished && !s.responseCompleted {
		s.usage = chunk.Usage

		if err := s.enqueueTerminalResponse(); err != nil {
			s.err = err
			return false
		}
	}

	// Continue to the next event
	return s.Next()
}

func isAbnormalResponsesFinishReason(reason string) bool {
	switch reason {
	case "length", "content_filter", "error", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

// enqueueTerminalResponse maps Chat finish reasons onto the terminal Responses
// stream events. The Responses API terminates streams with dedicated terminal
// event types: response.completed for normal finishes and response.incomplete,
// response.failed, or response.cancelled for truncated, failed, or cancelled
// runs.
func (s *responsesInboundStream) enqueueTerminalResponse() error {
	status := "completed"
	eventType := StreamEventTypeResponseCompleted
	switch s.finishReason {
	case "length", "content_filter":
		status = "incomplete"
		eventType = StreamEventTypeResponseIncomplete
	case "error":
		status = "failed"
		eventType = StreamEventTypeResponseFailed
	case "cancelled", "canceled":
		status = "cancelled"
		eventType = StreamEventTypeResponseCancelled
	}

	s.responseCompleted = true
	if s.aggregator.status == "" || s.aggregator.status == "in_progress" {
		s.aggregator.status = status
	}
	response := s.aggregator.buildResponse()
	applyEchoFields(response, s.echoResponse)
	if status == "incomplete" && response.IncompleteDetails == nil {
		reason := "max_output_tokens"
		if s.finishReason == "content_filter" {
			reason = "content_filter"
		}
		response.IncompleteDetails = &ResponseIncompleteDetails{Reason: reason}
	}
	if s.usage != nil {
		response.Usage = ConvertLLMUsageToResponsesUsage(s.usage)
	}
	if calls := getResponseWebSearchCallsFromMetadata(s.transformerMetadata); len(calls) > 0 {
		response.Output = append(append([]Item(nil), calls...), response.Output...)
	}
	if err := s.enqueueEvent(&StreamEvent{Type: eventType, Response: response}); err != nil {
		return fmt.Errorf("failed to enqueue %s event: %w", eventType, err)
	}
	return nil
}

func (s *responsesInboundStream) mergeTransformerMetadata(metadata map[string]any) {
	if len(metadata) == 0 {
		return
	}

	if calls := getResponseWebSearchCallsFromMetadata(metadata); len(calls) > 0 {
		existingCalls := getResponseWebSearchCallsFromMetadata(s.transformerMetadata)
		mergedCalls := append(existingCalls, calls...)
		s.transformerMetadata[responsesWebSearchCallsTransformerMetadataKey] = mergedCalls
	}
}

// captureEchoFields extracts the upstream Response echo fields from
// TransformerMetadata so they can be restored on rebuilt response events.
func (s *responsesInboundStream) captureEchoFields(metadata map[string]any) {
	if len(metadata) == 0 {
		return
	}
	raw, ok := metadata[responsesEchoFieldsTransformerMetadataKey]
	if !ok || raw == nil {
		return
	}
	if echo, ok := raw.(*Response); ok {
		s.echoResponse = echo
	}
}

func getResponsesReasoningItemMetadata(metadata map[string]any) (responsesReasoningItemMetadata, bool) {
	if len(metadata) == 0 {
		return responsesReasoningItemMetadata{}, false
	}

	raw, ok := metadata[responsesReasoningItemTransformerMetadataKey]
	if !ok || raw == nil {
		return responsesReasoningItemMetadata{}, false
	}

	if item, ok := raw.(responsesReasoningItemMetadata); ok {
		return item, item.ID != ""
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return responsesReasoningItemMetadata{}, false
	}

	var item responsesReasoningItemMetadata
	if err := json.Unmarshal(data, &item); err != nil {
		return responsesReasoningItemMetadata{}, false
	}

	return item, item.ID != ""
}

func (s *responsesInboundStream) handleReasoningContent(content *string, metadata map[string]any) error {
	sourceID := ""
	if item, ok := getResponsesReasoningItemMetadata(metadata); ok {
		sourceID = item.ID
	}
	if sourceID != "" {
		if _, exists := s.pendingReasoning[sourceID]; !exists {
			s.pendingReasoningOrder = append(s.pendingReasoningOrder, sourceID)
		}
		s.pendingReasoning[sourceID] = append(s.pendingReasoning[sourceID], *content)
		return nil
	}

	return s.emitReasoningContent(content, sourceID)
}

func (s *responsesInboundStream) emitReasoningContent(content *string, sourceID string) error {

	if err := s.ensureReasoningItemStarted(sourceID); err != nil {
		return err
	}

	// Start reasoning summary part only when we actually have summary text.
	if !s.hasReasoningSummaryPart {
		s.hasReasoningSummaryPart = true

		err := s.enqueueEvent(&StreamEvent{
			Type:         StreamEventTypeReasoningSummaryPartAdded,
			ItemID:       &s.currentItemID,
			OutputIndex:  s.outputIndex,
			SummaryIndex: lo.ToPtr(0),
			Part:         &StreamEventContentPart{Type: "summary_text"},
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue reasoning_summary_part.added event: %w", err)
		}
	}

	// Accumulate reasoning content
	s.accumulatedReasoning.WriteString(*content)

	// Emit reasoning_summary_text.delta
	err := s.enqueueEvent(&StreamEvent{
		Type:         StreamEventTypeReasoningSummaryTextDelta,
		ItemID:       &s.currentItemID,
		OutputIndex:  s.outputIndex,
		SummaryIndex: lo.ToPtr(0),
		Delta:        *content,
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue reasoning_summary_text.delta event: %w", err)
	}

	return nil
}

func (s *responsesInboundStream) handleReasoningSignature(delta *llm.Message, metadata map[string]any) error {
	sourceID := delta.ID
	itemMetadata, itemScoped := getResponsesReasoningItemMetadata(metadata)
	if itemScoped {
		sourceID = itemMetadata.ID
	}
	if sourceID != "" {
		if contents, ok := s.pendingReasoning[sourceID]; ok {
			for _, content := range contents {
				if err := s.emitReasoningContent(lo.ToPtr(content), sourceID); err != nil {
					return err
				}
			}
			delete(s.pendingReasoning, sourceID)
			s.removePendingReasoningID(sourceID)
		}
	}

	if err := s.ensureReasoningItemStarted(sourceID); err != nil {
		return err
	}

	if itemScoped {
		// Responses encrypted_content is an opaque item-level value. Repeated
		// values for the same item are provisional/final replacements, not deltas.
		s.accumulatedReasoningSignature.Reset()
	}
	s.accumulatedReasoningSignature.WriteString(*delta.ReasoningSignature)

	if itemScoped && itemMetadata.Done {
		return s.closeReasoningItem()
	}

	// OpenAI Responses encrypted_content is emitted as an opaque item-level blob,
	// not as a text-style delta. Close the item immediately so consecutive
	// reasoning blobs from separate upstream items are not concatenated.
	if sourceID == "" && delta.ReasoningContent == nil {
		return s.closeReasoningItem()
	}

	return nil
}

func (s *responsesInboundStream) removePendingReasoningID(sourceID string) {
	for i, id := range s.pendingReasoningOrder {
		if id == sourceID {
			s.pendingReasoningOrder = append(s.pendingReasoningOrder[:i], s.pendingReasoningOrder[i+1:]...)
			return
		}
	}
}

func (s *responsesInboundStream) flushPendingReasoning() error {
	for len(s.pendingReasoningOrder) > 0 {
		sourceID := s.pendingReasoningOrder[0]
		contents, ok := s.pendingReasoning[sourceID]
		if !ok {
			s.removePendingReasoningID(sourceID)
			continue
		}
		for _, content := range contents {
			if err := s.emitReasoningContent(lo.ToPtr(content), sourceID); err != nil {
				return err
			}
		}
		delete(s.pendingReasoning, sourceID)
		s.removePendingReasoningID(sourceID)
		if err := s.closeReasoningItem(); err != nil {
			return err
		}
	}
	return nil
}

func (s *responsesInboundStream) ensureReasoningItemStarted(sourceID string) error {
	// Start reasoning output item if not started.
	if s.hasReasoningItemStarted {
		if sourceID == "" || s.currentReasoningSourceID == "" || s.currentReasoningSourceID == sourceID {
			return nil
		}

		if err := s.closeReasoningItem(); err != nil {
			return err
		}
	}

	// Close any previous output item.
	if err := s.closeCurrentOutputItem(); err != nil {
		return err
	}

	s.hasReasoningItemStarted = true
	s.hasReasoningSummaryPart = false
	s.currentReasoningSourceID = sourceID

	s.currentItemID = sourceID
	if s.currentItemID == "" {
		s.currentItemID = generateItemID()
	}
	item := &Item{
		ID:      s.currentItemID,
		Type:    "reasoning",
		Status:  lo.ToPtr("in_progress"),
		Summary: []ReasoningSummary{},
	}

	err := s.enqueueEvent(&StreamEvent{
		Type:        StreamEventTypeOutputItemAdded,
		OutputIndex: s.outputIndex,
		Item:        item,
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue output_item.added event: %w", err)
	}

	return nil
}

func (s *responsesInboundStream) handleTextContent(content *string) error {
	if err := s.flushPendingReasoning(); err != nil {
		return err
	}

	// Close reasoning item if it was started
	if s.hasReasoningItemStarted {
		if err := s.closeReasoningItem(); err != nil {
			return err
		}
	}

	// Start message output item if not started
	if !s.hasMessageItemStarted {
		s.hasMessageItemStarted = true

		s.currentItemID = generateItemID()

		err := s.enqueueEvent(&StreamEvent{
			Type:        StreamEventTypeOutputItemAdded,
			OutputIndex: s.outputIndex,
			Item: &Item{
				ID:      s.currentItemID,
				Type:    "message",
				Status:  lo.ToPtr("in_progress"),
				Role:    "assistant",
				Content: &Input{Items: []Item{}},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue output_item.added event: %w", err)
		}
	}

	// Start content part if not started
	if !s.hasContentPartStarted {
		s.hasContentPartStarted = true

		textPartItems, _ := attachAnnotationsToFirstTextItem([]Item{{
			Type:        "output_text",
			Annotations: []Annotation{},
		}}, s.pendingAnnotations)

		err := s.enqueueEvent(&StreamEvent{
			Type:         StreamEventTypeContentPartAdded,
			ItemID:       &s.currentItemID,
			OutputIndex:  s.outputIndex,
			ContentIndex: &s.contentIndex,
			Part: &StreamEventContentPart{
				Type:        "output_text",
				Text:        "",
				Annotations: textPartItems[0].Annotations,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue content_part.added event: %w", err)
		}
		// Keep pendingAnnotations until output_item.done so the final message item preserves them.
	}

	// Accumulate text content
	s.accumulatedText.WriteString(*content)

	// Emit output_text.delta
	err := s.enqueueEvent(&StreamEvent{
		Type:         StreamEventTypeOutputTextDelta,
		ItemID:       &s.currentItemID,
		OutputIndex:  s.outputIndex,
		ContentIndex: &s.contentIndex,
		Delta:        *content,
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue output_text.delta event: %w", err)
	}

	return nil
}

func (s *responsesInboundStream) handleCompactionContent(parts []llm.MessageContentPart) error {
	for _, part := range parts {
		if (part.Type != "compaction" && part.Type != "compaction_summary") || part.Compact == nil {
			continue
		}

		if err := s.closeCurrentOutputItem(); err != nil {
			return err
		}

		item := compactionItemFromPart(part, part.Type)
		if item.ID == "" {
			item.ID = generateItemID()
		}

		for _, eventType := range []StreamEventType{StreamEventTypeOutputItemAdded, StreamEventTypeOutputItemDone} {
			if err := s.enqueueEvent(&StreamEvent{
				Type:        eventType,
				OutputIndex: s.outputIndex,
				Item:        &item,
			}); err != nil {
				return fmt.Errorf("failed to enqueue compaction %s event: %w", eventType, err)
			}
		}

		s.outputIndex++
	}

	return nil
}

func (s *responsesInboundStream) handleToolCalls(toolCalls []llm.ToolCall) error {
	if err := s.flushPendingReasoning(); err != nil {
		return err
	}

	// Close message item if it was started
	if s.hasMessageItemStarted {
		if err := s.closeMessageItem(); err != nil {
			return err
		}
	}

	// Close reasoning item if it was started
	if s.hasReasoningItemStarted {
		if err := s.closeReasoningItem(); err != nil {
			return err
		}
	}

	if s.toolCallItemStarted == nil {
		s.toolCallItemStarted = make(map[int]bool)
	}

	if s.toolCallOutputIndex == nil {
		s.toolCallOutputIndex = make(map[int]int)
	}
	if s.toolCallItemID == nil {
		s.toolCallItemID = make(map[int]string)
	}

	for _, tc := range toolCalls {
		toolCallIndex := tc.Index

		// Initialize tool call tracking if needed
		if _, ok := s.toolCalls[toolCallIndex]; !ok {
			if err := s.initToolCall(tc); err != nil {
				return err
			}
		}
		state := s.toolCalls[toolCallIndex]
		stateKind := responsesToolCallKind(state)
		deltaKind := responsesToolCallKind(&tc)
		if stateKind != deltaKind {
			return fmt.Errorf("tool call index %d changed type from %s to %s", toolCallIndex, stateKind, deltaKind)
		}
		if state.ID == "" && tc.ID != "" {
			state.ID = tc.ID
			if state.ResponseCustomToolCall != nil {
				state.ResponseCustomToolCall.CallID = tc.ID
			}
			if state.ResponseToolSearchCall != nil {
				state.ResponseToolSearchCall.CallID = tc.ID
			}
		}
		if state.ResponseCustomToolCall != nil && state.ResponseCustomToolCall.Name == "" && tc.ResponseCustomToolCall != nil {
			state.ResponseCustomToolCall.Name = tc.ResponseCustomToolCall.Name
		}

		// Process delta based on tool type
		switch {
		case tc.ResponseToolSearchCall != nil:
			if err := s.handleToolSearchCallDelta(tc); err != nil {
				return err
			}
		case tc.ResponseCustomToolCall != nil:
			if err := s.handleCustomToolCallDelta(tc); err != nil {
				return err
			}
		default:
			if err := s.handleFunctionCallDelta(tc); err != nil {
				return err
			}
		}
	}

	return nil
}

// responsesToolCallKind returns the lifecycle kind represented by one unified tool call.
func responsesToolCallKind(call *llm.ToolCall) string {
	switch {
	case call.ResponseToolSearchCall != nil:
		return "tool_search"
	case call.ResponseCustomToolCall != nil:
		return "custom"
	default:
		return "function"
	}
}

func (s *responsesInboundStream) initToolCall(tc llm.ToolCall) error {
	toolCallIndex := tc.Index

	if err := s.closeCurrentContentPart(); err != nil {
		return err
	}

	var customCall *llm.ResponseCustomToolCall
	if tc.ResponseCustomToolCall != nil {
		customCall = &llm.ResponseCustomToolCall{CallID: tc.ResponseCustomToolCall.CallID, Name: tc.ResponseCustomToolCall.Name}
	}
	var toolSearchCall *llm.ResponseToolSearchCall
	if tc.ResponseToolSearchCall != nil {
		toolSearchCall = &llm.ResponseToolSearchCall{CallID: tc.ResponseToolSearchCall.CallID, Execution: tc.ResponseToolSearchCall.Execution}
	}

	s.toolCalls[toolCallIndex] = &llm.ToolCall{
		Index:                  toolCallIndex,
		ID:                     tc.ID,
		Type:                   tc.Type,
		ResponseCustomToolCall: customCall,
		ResponseToolSearchCall: toolSearchCall,
		TransformerMetadata:    tc.TransformerMetadata,
		Function: llm.FunctionCall{
			Name:      tc.Function.Name,
			Namespace: tc.Function.Namespace,
			Arguments: "",
		},
	}

	// A Responses function_call must include its name in output_item.added for
	// clients to route it. Some upstreams provide that identity only in a later
	// arguments delta or done event, so retain the call until it is known.
	// Custom and tool-search calls carry their identity from the first chunk.
	if tc.ResponseCustomToolCall == nil && tc.ResponseToolSearchCall == nil && tc.Function.Name == "" {
		return nil
	}

	return s.startToolCallItem(toolCallIndex)
}

func (s *responsesInboundStream) startToolCallItem(toolCallIndex int) error {
	if s.toolCallItemStarted[toolCallIndex] {
		return nil
	}

	tc := s.toolCalls[toolCallIndex]

	itemID := tc.ID
	if itemID == "" {
		itemID = generateItemID()
	}

	switch {
	case tc.ResponseToolSearchCall != nil:
		item := &Item{
			ID: itemID, Type: "tool_search_call", Status: lo.ToPtr("in_progress"),
			CallID: tc.ResponseToolSearchCall.CallID, Execution: tc.ResponseToolSearchCall.Execution,
		}
		if err := s.enqueueEvent(&StreamEvent{Type: StreamEventTypeOutputItemAdded, OutputIndex: s.outputIndex, Item: item}); err != nil {
			return fmt.Errorf("failed to enqueue output_item.added event: %w", err)
		}
	case tc.ResponseCustomToolCall != nil:
		item := &Item{
			ID:     itemID,
			Type:   "custom_tool_call",
			Status: lo.ToPtr("in_progress"),
			CallID: tc.ResponseCustomToolCall.CallID,
			Name:   tc.ResponseCustomToolCall.Name,
			Input:  lo.ToPtr(""),
		}

		err := s.enqueueEvent(&StreamEvent{
			Type:        StreamEventTypeOutputItemAdded,
			OutputIndex: s.outputIndex,
			Item:        item,
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue output_item.added event: %w", err)
		}

	default:
		item := &Item{
			ID:        itemID,
			Type:      "function_call",
			Status:    lo.ToPtr("in_progress"),
			CallID:    tc.ID,
			Name:      tc.Function.Name,
			Namespace: tc.Function.Namespace,
		}

		err := s.enqueueEvent(&StreamEvent{
			Type:        StreamEventTypeOutputItemAdded,
			OutputIndex: s.outputIndex,
			Item:        item,
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue output_item.added event: %w", err)
		}
	}

	s.toolCallItemStarted[toolCallIndex] = true
	s.toolCallOutputIndex[toolCallIndex] = s.outputIndex
	s.toolCallItemID[toolCallIndex] = itemID
	s.currentItemID = itemID
	s.outputIndex++

	return nil
}

func (s *responsesInboundStream) handleFunctionCallDelta(tc llm.ToolCall) error {
	toolCallIndex := tc.Index
	storedToolCall := s.toolCalls[toolCallIndex]
	if tc.ID != "" {
		storedToolCall.ID = tc.ID
	}
	if tc.Type != "" {
		storedToolCall.Type = tc.Type
	}
	if tc.Function.Name != "" {
		storedToolCall.Function.Name = tc.Function.Name
	}
	if tc.Function.Namespace != "" {
		storedToolCall.Function.Namespace = tc.Function.Namespace
	}
	storedToolCall.Function.Arguments += tc.Function.Arguments

	argumentsToEmit := tc.Function.Arguments
	if !s.toolCallItemStarted[toolCallIndex] {
		if storedToolCall.Function.Name == "" {
			return nil
		}

		if err := s.startToolCallItem(toolCallIndex); err != nil {
			return err
		}

		// Arguments received before the identity became available have not been
		// emitted. Replay the complete buffered payload after output_item.added.
		argumentsToEmit = storedToolCall.Function.Arguments
	}

	if argumentsToEmit != "" {
		itemID := s.toolCallItemID[toolCallIndex]

		err := s.enqueueEvent(&StreamEvent{
			Type:         StreamEventTypeFunctionCallArgumentsDelta,
			ItemID:       &itemID,
			OutputIndex:  s.toolCallOutputIndex[toolCallIndex],
			ContentIndex: lo.ToPtr(0),
			Delta:        argumentsToEmit,
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue function_call_arguments.delta event: %w", err)
		}
	}

	return nil
}

func (s *responsesInboundStream) handleCustomToolCallDelta(tc llm.ToolCall) error {
	toolCallIndex := tc.Index
	state := s.toolCalls[toolCallIndex]
	state.ResponseCustomToolCall.Input += tc.ResponseCustomToolCall.Input
	if wrapped, _ := tc.TransformerMetadata[ChatWrappedCustomMetadataKey].(bool); wrapped {
		if state.TransformerMetadata == nil {
			state.TransformerMetadata = map[string]any{}
		}
		state.TransformerMetadata[ChatWrappedCustomMetadataKey] = true
	}
	if wrapped, _ := state.TransformerMetadata[ChatWrappedCustomMetadataKey].(bool); wrapped {
		return nil
	}

	if tc.ResponseCustomToolCall.Input != "" {
		itemID := s.toolCallItemID[toolCallIndex]

		err := s.enqueueEvent(&StreamEvent{
			Type:        StreamEventTypeCustomToolCallInputDelta,
			ItemID:      &itemID,
			OutputIndex: s.toolCallOutputIndex[toolCallIndex],
			Delta:       tc.ResponseCustomToolCall.Input,
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue custom_tool_call_input.delta event: %w", err)
		}
	}

	return nil
}

// handleToolSearchCallDelta emits lifecycle events for a client tool-search call.
func (s *responsesInboundStream) handleToolSearchCallDelta(tc llm.ToolCall) error {
	toolCallIndex := tc.Index
	state := s.toolCalls[toolCallIndex].ResponseToolSearchCall
	state.Arguments += tc.ResponseToolSearchCall.Arguments
	if tc.ResponseToolSearchCall.Arguments == "" {
		return nil
	}
	itemID := s.toolCallItemID[toolCallIndex]
	if err := s.enqueueEvent(&StreamEvent{
		Type: StreamEventTypeFunctionCallArgumentsDelta, ItemID: &itemID,
		OutputIndex: s.toolCallOutputIndex[toolCallIndex], ContentIndex: lo.ToPtr(0),
		Delta: tc.ResponseToolSearchCall.Arguments,
	}); err != nil {
		return fmt.Errorf("failed to enqueue tool_search_call arguments delta: %w", err)
	}
	return nil
}

func (s *responsesInboundStream) closeReasoningItem() error {
	if !s.hasReasoningItemStarted {
		return nil
	}

	s.hasReasoningItemStarted = false
	fullReasoning := s.accumulatedReasoning.String()
	hadSummaryPart := s.hasReasoningSummaryPart

	// Emit reasoning summary done events only if we started the summary part.
	if hadSummaryPart {
		// Emit reasoning_summary_text.done with accumulated text
		err := s.enqueueEvent(&StreamEvent{
			Type:         StreamEventTypeReasoningSummaryTextDone,
			ItemID:       &s.currentItemID,
			OutputIndex:  s.outputIndex,
			SummaryIndex: lo.ToPtr(0),
			Text:         fullReasoning,
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue reasoning_summary_text.done event: %w", err)
		}

		// Emit reasoning_summary_part.done
		err = s.enqueueEvent(&StreamEvent{
			Type:         StreamEventTypeReasoningSummaryPartDone,
			ItemID:       &s.currentItemID,
			OutputIndex:  s.outputIndex,
			SummaryIndex: lo.ToPtr(0),
			Part: &StreamEventContentPart{
				Type: "summary_text",
				Text: fullReasoning,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue reasoning_summary_part.done event: %w", err)
		}
	}

	s.hasReasoningSummaryPart = false

	// Emit output_item.done with complete reasoning item
	var encryptedContent *string

	if s.accumulatedReasoningSignature.Len() > 0 {
		encoded := s.accumulatedReasoningSignature.String()
		encryptedContent = lo.ToPtr(encoded)
	}

	var summary []ReasoningSummary
	if hadSummaryPart {
		summary = []ReasoningSummary{{
			Type: "summary_text",
			Text: fullReasoning,
		}}
	} else {
		summary = []ReasoningSummary{}
	}

	item := Item{
		ID:               s.currentItemID,
		Type:             "reasoning",
		Status:           lo.ToPtr("completed"),
		Summary:          summary,
		EncryptedContent: encryptedContent,
	}

	err := s.enqueueEvent(&StreamEvent{
		Type:        StreamEventTypeOutputItemDone,
		OutputIndex: s.outputIndex,
		Item:        &item,
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue output_item.done event: %w", err)
	}

	s.outputIndex++
	s.accumulatedReasoning.Reset()
	s.accumulatedReasoningSignature.Reset()
	s.currentReasoningSourceID = ""

	return nil
}

func (s *responsesInboundStream) closeMessageItem() error {
	if !s.hasMessageItemStarted {
		return nil
	}

	s.hasMessageItemStarted = false
	fullText := s.accumulatedText.String()

	// Close content part first
	if err := s.closeCurrentContentPart(); err != nil {
		return err
	}

	// Emit output_item.done with complete message content
	item := Item{
		ID:     s.currentItemID,
		Type:   "message",
		Status: lo.ToPtr("completed"),
		Role:   "assistant",
		Content: &Input{
			Items: []Item{{
				Type:        "output_text",
				Text:        &fullText,
				Annotations: []Annotation{},
			}},
		},
	}
	item.Content.Items, _ = attachAnnotationsToFirstTextItem(item.Content.Items, s.pendingAnnotations)
	s.pendingAnnotations = nil

	err := s.enqueueEvent(&StreamEvent{
		Type:        StreamEventTypeOutputItemDone,
		OutputIndex: s.outputIndex,
		Item:        &item,
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue output_item.done event: %w", err)
	}

	s.outputIndex++
	s.contentIndex = 0
	s.accumulatedText.Reset()

	return nil
}

func (s *responsesInboundStream) closeCurrentContentPart() error {
	if !s.hasContentPartStarted {
		return nil
	}

	s.hasContentPartStarted = false
	fullText := s.accumulatedText.String()

	// Emit output_text.done with accumulated text
	err := s.enqueueEvent(&StreamEvent{
		Type:         StreamEventTypeOutputTextDone,
		ItemID:       &s.currentItemID,
		OutputIndex:  s.outputIndex,
		ContentIndex: &s.contentIndex,
		Text:         fullText,
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue output_text.done event: %w", err)
	}

	// Emit content_part.done with full text
	contentPartItems, _ := attachAnnotationsToFirstTextItem([]Item{{
		Type:        "output_text",
		Text:        lo.ToPtr(fullText),
		Annotations: []Annotation{},
	}}, s.pendingAnnotations)

	err = s.enqueueEvent(&StreamEvent{
		Type:         StreamEventTypeContentPartDone,
		ItemID:       &s.currentItemID,
		OutputIndex:  s.outputIndex,
		ContentIndex: &s.contentIndex,
		Part: &StreamEventContentPart{
			Type:        "output_text",
			Text:        fullText,
			Annotations: contentPartItems[0].Annotations,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue content_part.done event: %w", err)
	}

	return nil
}

func (s *responsesInboundStream) closeCurrentOutputItem() error {
	// Close message item if open
	if s.hasMessageItemStarted {
		if err := s.closeMessageItem(); err != nil {
			return err
		}
	}

	// Close reasoning item if open
	if s.hasReasoningItemStarted {
		if err := s.closeReasoningItem(); err != nil {
			return err
		}
	}

	// Non-tool closures must not touch tool call state; skipping here also
	// avoids writing to a nil toolCallItemStarted map.
	if s.skipToolCallClosure {
		return nil
	}

	// Close any open tool call items
	toolCallIndexes := lo.Keys(s.toolCalls)
	sort.Ints(toolCallIndexes)
	for _, idx := range toolCallIndexes {
		tc := s.toolCalls[idx]
		if !s.toolCallItemStarted[idx] {
			continue
		}

		itemID := s.toolCallItemID[idx]
		if itemID == "" {
			itemID = tc.ID
		}
		if itemID == "" {
			itemID = s.currentItemID
		}

		switch {
		case tc.ResponseToolSearchCall != nil:
			arguments := tc.ResponseToolSearchCall.Arguments
			if err := s.enqueueEvent(&StreamEvent{
				Type: StreamEventTypeFunctionCallArgumentsDone, ItemID: &itemID,
				OutputIndex: s.toolCallOutputIndex[idx], Arguments: arguments,
			}); err != nil {
				return fmt.Errorf("failed to enqueue tool_search_call arguments done: %w", err)
			}
			item := Item{
				ID: itemID, Type: "tool_search_call", Status: lo.ToPtr("completed"),
				CallID: tc.ResponseToolSearchCall.CallID, Execution: tc.ResponseToolSearchCall.Execution,
				Arguments: arguments,
			}
			if err := s.enqueueEvent(&StreamEvent{Type: StreamEventTypeOutputItemDone, OutputIndex: s.toolCallOutputIndex[idx], Item: &item}); err != nil {
				return fmt.Errorf("failed to enqueue output_item.done event: %w", err)
			}
		case tc.ResponseCustomToolCall != nil:
			// Custom tool call - emit custom_tool_call_input.done then output_item.done
			fullInput := tc.ResponseCustomToolCall.Input
			if wrapped, _ := tc.TransformerMetadata[ChatWrappedCustomMetadataKey].(bool); wrapped {
				var input struct {
					Input *string `json:"input"`
				}
				if err := json.Unmarshal([]byte(fullInput), &input); err != nil {
					slog.WarnContext(s.ctx, "failed to unwrap Chat custom tool input",
						slog.String("call_id", tc.ResponseCustomToolCall.CallID),
						slog.Any("error", err))
					// Some compatible providers ignore the wrapper schema and emit raw
					// custom input. Preserve it, matching the non-streaming fallback.
				} else if input.Input == nil {
					slog.WarnContext(s.ctx, "failed to unwrap Chat custom tool input",
						slog.String("call_id", tc.ResponseCustomToolCall.CallID),
						slog.String("error", "missing input field"))
				} else {
					fullInput = *input.Input
				}
				if fullInput != "" {
					if err := s.enqueueEvent(&StreamEvent{
						Type: StreamEventTypeCustomToolCallInputDelta, ItemID: &itemID,
						OutputIndex: s.toolCallOutputIndex[idx], Delta: fullInput,
					}); err != nil {
						return fmt.Errorf("failed to enqueue custom_tool_call_input.delta event: %w", err)
					}
				}
			}

			err := s.enqueueEvent(&StreamEvent{
				Type:        StreamEventTypeCustomToolCallInputDone,
				ItemID:      &itemID,
				OutputIndex: s.toolCallOutputIndex[idx],
				Input:       fullInput,
			})
			if err != nil {
				return fmt.Errorf("failed to enqueue custom_tool_call_input.done event: %w", err)
			}

			item := Item{
				ID:     itemID,
				Type:   "custom_tool_call",
				Status: lo.ToPtr("completed"),
				CallID: tc.ResponseCustomToolCall.CallID,
				Name:   tc.ResponseCustomToolCall.Name,
				Input:  lo.ToPtr(fullInput),
			}

			err = s.enqueueEvent(&StreamEvent{
				Type:        StreamEventTypeOutputItemDone,
				OutputIndex: s.toolCallOutputIndex[idx],
				Item:        &item,
			})
			if err != nil {
				return fmt.Errorf("failed to enqueue output_item.done event: %w", err)
			}

		default:
			// Function call - emit function_call_arguments.done then output_item.done
			err := s.enqueueEvent(&StreamEvent{
				Type:        StreamEventTypeFunctionCallArgumentsDone,
				ItemID:      &itemID,
				OutputIndex: s.toolCallOutputIndex[idx],
				CallID:      tc.ID,
				Name:        tc.Function.Name,
				Namespace:   tc.Function.Namespace,
				Arguments:   tc.Function.Arguments,
			})
			if err != nil {
				return fmt.Errorf("failed to enqueue function_call_arguments.done event: %w", err)
			}

			item := Item{
				ID:        itemID,
				Type:      "function_call",
				Status:    lo.ToPtr("completed"),
				CallID:    tc.ID,
				Name:      tc.Function.Name,
				Namespace: tc.Function.Namespace,
				Arguments: tc.Function.Arguments,
			}

			err = s.enqueueEvent(&StreamEvent{
				Type:        StreamEventTypeOutputItemDone,
				OutputIndex: s.toolCallOutputIndex[idx],
				Item:        &item,
			})
			if err != nil {
				return fmt.Errorf("failed to enqueue output_item.done event: %w", err)
			}
		}

		s.toolCallItemStarted[idx] = false
	}

	return nil
}

func (s *responsesInboundStream) closeCurrentNonToolOutputItem() error {
	s.skipToolCallClosure = true
	defer func() { s.skipToolCallClosure = false }()

	return s.closeCurrentOutputItem()
}

func (s *responsesInboundStream) emitStreamErrorEvent(err error) error {
	code, message := classifyStreamError(err)

	if s.hasResponseCreated {
		response := s.buildFailedResponse(code, message)
		if err := s.enqueueEvent(&StreamEvent{
			Type:     StreamEventTypeResponseFailed,
			Response: response,
		}); err != nil {
			return err
		}
	} else {
		if err := s.enqueueEvent(&StreamEvent{
			Type:    StreamEventTypeError,
			Code:    code,
			Message: message,
		}); err != nil {
			return err
		}
	}

	s.errorEventEmitted = true

	return nil
}

func classifyStreamError(err error) (code, message string) {
	code = "stream_error"
	message = err.Error()

	if errors.Is(err, io.EOF) {
		code = "upstream_eof"
		message = "upstream connection closed unexpectedly"
		return code, message
	}

	if errors.Is(err, context.Canceled) {
		code = "client_cancel"
		message = "client disconnected"
		return code, message
	}

	if errors.Is(err, context.DeadlineExceeded) {
		code = "timeout"
		message = "request timeout"
		return code, message
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) {
		code = "api_error"
		message = string(httpErr.Body)
		if message == "" {
			message = httpErr.Status
		}
		return code, message
	}

	if errors.Is(err, ErrStreamIncomplete) {
		code = "incomplete_stream"
		message = "stream ended without terminal event"
		return code, message
	}

	return code, message
}

func (s *responsesInboundStream) buildFailedResponse(code, message string) *Response {
	response := &Response{
		Object:    "response",
		ID:        s.responseID,
		Model:     s.model,
		CreatedAt: s.createdAt,
		Status:    lo.ToPtr("failed"),
		Output:    []Item{},
		Error: &Error{
			Type:    "server_error",
			Code:    code,
			Message: message,
		},
	}
	applyEchoFields(response, s.echoResponse)

	if s.aggregator != nil {
		aggregated := s.aggregator.buildResponse()
		response.Output = aggregated.Output
	}

	return response
}

func (s *responsesInboundStream) Current() *httpclient.StreamEvent {
	if s.queueIndex < len(s.eventQueue) {
		event := s.eventQueue[s.queueIndex]
		s.queueIndex++

		return event
	}

	return nil
}

func (s *responsesInboundStream) Err() error {
	// If we've already emitted an error event to the client, return nil
	// to avoid double error emission by the SSE writer
	if s.errorEventEmitted {
		return nil
	}

	if s.err != nil {
		return s.err
	}

	return s.source.Err()
}

func (s *responsesInboundStream) Close() error {
	return s.source.Close()
}

// AggregateStreamChunks aggregates streaming chunks into a complete response body.
func (t *InboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}
