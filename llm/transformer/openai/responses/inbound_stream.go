package responses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

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
	hasReasoningTextPart    bool
	hasContentPartStarted   bool
	hasFinished             bool
	responseCompleted       bool
	terminalFinishReason    string
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

	// Tool call tracking
	toolCalls           map[int]*llm.ToolCall
	currentToolCallIdx  int
	toolCallItemStarted map[int]bool
	toolCallOutputIndex map[int]int    // Maps tool call index to output index
	toolCallItemIDs     map[int]string // Stable Responses output item ids per tool-call index

	// Response accumulation using streamAggregator
	usage               *llm.Usage
	aggregator          *streamAggregator
	transformerMetadata map[string]any

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
	if s.responseCompleted {
		return false
	}

	// Try to get the next chunk from source
	if !s.source.Next() {
		if s.err == nil && !s.errorEventEmitted && s.hasFinished {
			if err := s.enqueueTerminalResponse(); err != nil {
				s.err = fmt.Errorf("failed to enqueue terminal response event: %w", err)
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

	if s.enqueueRawResponsesStreamEvents(chunk) {
		return s.Next()
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
			if err := s.handleReasoningContent(choice.Delta.ReasoningContent); err != nil {
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
			s.terminalFinishReason = *choice.FinishReason

			// Close any open content parts
			if err := s.closeCurrentContentPart(); err != nil {
				s.err = err
				return false
			}

			// Close any open output items
			if err := s.closeCurrentOutputItem(); err != nil {
				s.err = err
				return false
			}
		}
	}

	// Complete when finish has been observed. Prefer waiting for a later usage
	// chunk when one is expected, but do not require usage: complete as soon as
	// finish is known and either usage is already present or this chunk itself
	// carries usage. Usage-less completion is finalized at stream end below.
	if s.hasFinished && !s.responseCompleted && s.usage != nil {
		if err := s.enqueueTerminalResponse(); err != nil {
			s.err = fmt.Errorf("failed to enqueue terminal response event: %w", err)
			return false
		}
	}

	// Continue to the next event
	return s.Next()
}

func (s *responsesInboundStream) enqueueTerminalResponse() error {
	s.responseCompleted = true
	eventType, status := responsesTerminalEvent(s.terminalFinishReason)
	s.aggregator.status = status
	response := s.aggregator.buildResponse()
	if s.usage != nil {
		response.Usage = ConvertLLMUsageToResponsesUsage(s.usage)
	}
	if calls := getResponseWebSearchCallsFromMetadata(s.transformerMetadata); len(calls) > 0 {
		response.Output = append(append([]Item(nil), calls...), response.Output...)
	}

	return s.enqueueEvent(&StreamEvent{Type: eventType, Response: response})
}

func responsesTerminalEvent(finishReason string) (StreamEventType, string) {
	switch finishReason {
	case "length":
		return StreamEventTypeResponseIncomplete, "incomplete"
	case "error":
		return StreamEventTypeResponseFailed, "failed"
	case "cancelled", "canceled":
		return StreamEventTypeResponseCancelled, "canceled"
	default:
		return StreamEventTypeResponseCompleted, "completed"
	}
}

// enqueueRawResponsesStreamEvents replays only Responses-native events that
// could not be represented by the canonical chunk model. They deliberately do
// not enter the synthesized event aggregator or any cross-protocol adapter.
func (s *responsesInboundStream) enqueueRawResponsesStreamEvents(chunk *llm.Response) bool {
	if chunk == nil || chunk.ProviderExtensions == nil || chunk.ProviderExtensions.OpenAIResponses == nil ||
		chunk.ProviderExtensions.OpenAIResponses.Response == nil {
		return false
	}
	events := chunk.ProviderExtensions.OpenAIResponses.Response.RawStreamEvents
	if len(events) == 0 {
		return false
	}
	for _, event := range events {
		if len(event.Raw) == 0 || event.Type == "" {
			continue
		}
		s.eventQueue = append(s.eventQueue, &httpclient.StreamEvent{
			Type: event.Type,
			Data: append([]byte(nil), event.Raw...),
		})
	}
	return len(s.eventQueue) > 0
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
	// Pass through the namespace tool map (established once during
	// inbound request, does not need merging).
	if raw, ok := metadata[responsesNamespaceToolMapTransformerMetadataKey]; ok && raw != nil {
		s.transformerMetadata[responsesNamespaceToolMapTransformerMetadataKey] = raw
	}
	// Reasoning text stream preference / original content sidecar.
	for _, key := range []string{
		responsesReasoningPreferTextStreamTransformerMetadataKey,
		responsesReasoningTextContentTransformerMetadataKey,
		responsesReasoningSummaryContentTransformerMetadataKey,
	} {
		if raw, ok := metadata[key]; ok && raw != nil {
			s.transformerMetadata[key] = raw
		}
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

func (s *responsesInboundStream) handleReasoningContent(content *string) error {
	if err := s.ensureReasoningItemStarted(""); err != nil {
		return err
	}

	s.accumulatedReasoning.WriteString(*content)

	emitReasoningText := false
	if s.transformerMetadata != nil {
		if _, ok := s.transformerMetadata[responsesReasoningTextContentTransformerMetadataKey]; ok {
			emitReasoningText = true
		}
		if v, ok := s.transformerMetadata[responsesReasoningPreferTextStreamTransformerMetadataKey].(bool); ok && v {
			emitReasoningText = true
		}
	}

	if emitReasoningText {
		if !s.hasReasoningTextPart {
			s.hasReasoningTextPart = true
		}
		err := s.enqueueEvent(&StreamEvent{
			Type:         StreamEventTypeReasoningTextDelta,
			ItemID:       &s.currentItemID,
			OutputIndex:  s.outputIndex,
			ContentIndex: lo.ToPtr(0),
			Delta:        *content,
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue reasoning_text.delta event: %w", err)
		}
		// When protocol-native text stream is requested, skip summary-compat events.
		return nil
	}

	// Default common path: emit reasoning_summary_* for existing fixtures/clients.
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
	if item, ok := getResponsesReasoningItemMetadata(metadata); ok {
		sourceID = item.ID
	}

	if err := s.ensureReasoningItemStarted(sourceID); err != nil {
		return err
	}

	s.accumulatedReasoningSignature.WriteString(*delta.ReasoningSignature)

	if item, ok := getResponsesReasoningItemMetadata(metadata); ok && item.Done {
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

func (s *responsesInboundStream) handleToolCalls(toolCalls []llm.ToolCall) error {
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

	for _, tc := range toolCalls {
		toolCallIndex := tc.Index

		// Initialize tool call tracking if needed
		if _, ok := s.toolCalls[toolCallIndex]; !ok {
			if err := s.initToolCall(tc); err != nil {
				return err
			}
		}

		// Process delta based on tool type. Chat custom calls use
		// OpenAIChatCustomToolCall; bridge them onto the Responses custom path.
		switch {
		case tc.ResponseCustomToolCall != nil || tc.OpenAIChatCustomToolCall != nil:
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

func (s *responsesInboundStream) initToolCall(tc llm.ToolCall) error {
	toolCallIndex := tc.Index

	if err := s.closeCurrentContentPart(); err != nil {
		return err
	}

	if err := s.closeCurrentOutputItem(); err != nil {
		return err
	}

	// Bridge Chat-native custom calls into the Responses custom carrier used by
	// stream state. Preserve ResponseCustomToolCall when already present.
	var customCall *llm.ResponseCustomToolCall
	if tc.ResponseCustomToolCall != nil {
		customCall = &llm.ResponseCustomToolCall{
			CallID:    tc.ResponseCustomToolCall.CallID,
			Name:      tc.ResponseCustomToolCall.Name,
			Namespace: tc.ResponseCustomToolCall.Namespace,
		}
	} else if tc.OpenAIChatCustomToolCall != nil {
		customCall = &llm.ResponseCustomToolCall{
			CallID: tc.ID,
			Name:   tc.OpenAIChatCustomToolCall.Name,
			Input:  "",
		}
	}

	s.toolCalls[toolCallIndex] = &llm.ToolCall{
		Index:                    toolCallIndex,
		ID:                       tc.ID,
		Type:                     tc.Type,
		ResponseItemID:           tc.ResponseItemID,
		ResponseCustomToolCall:   customCall,
		OpenAIChatCustomToolCall: tc.OpenAIChatCustomToolCall,
		Function: llm.FunctionCall{
			Name:      tc.Function.Name,
			Namespace: tc.Function.Namespace,
			Arguments: "",
		},
	}

	itemID := s.resolveToolCallItemID(toolCallIndex, tc)

	switch {
	case customCall != nil:
		item := &Item{
			ID:        itemID,
			Type:      "custom_tool_call",
			Status:    lo.ToPtr("in_progress"),
			CallID:    customCall.CallID,
			Name:      customCall.Name,
			Namespace: customCall.Namespace,
			Input:     lo.ToPtr(""),
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
		// Restore namespace group identity via table lookup (never string
		// splitting — group names may contain "__").
		fcName, fcNamespace := resolveNamespaceFromMetadata(s.transformerMetadata, tc.Function.Name)
		item := &Item{
			ID:        itemID,
			Type:      "function_call",
			Status:    lo.ToPtr("in_progress"),
			CallID:    tc.ID,
			Name:      fcName,
			Namespace: fcNamespace,
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
	s.currentItemID = itemID
	s.outputIndex++

	return nil
}

// resolveToolCallItemID returns the stable Responses output item id for a tool
// call. When the source has no ResponseItemID, allocate once via generateItemID
// and reuse it for later delta/done events. Never alias item id to call_id.
func (s *responsesInboundStream) resolveToolCallItemID(toolCallIndex int, tc llm.ToolCall) string {
	if s.toolCallItemIDs == nil {
		s.toolCallItemIDs = make(map[int]string)
	}
	if itemID, ok := s.toolCallItemIDs[toolCallIndex]; ok && itemID != "" {
		return itemID
	}
	itemID := tc.ResponseItemID
	if itemID == "" {
		itemID = generateItemID()
	}
	s.toolCallItemIDs[toolCallIndex] = itemID
	return itemID
}

// toolCallItemID looks up the stable item id for an already-initialized tool call.
func (s *responsesInboundStream) toolCallItemID(toolCallIndex int, tc *llm.ToolCall) string {
	if s.toolCallItemIDs != nil {
		if itemID, ok := s.toolCallItemIDs[toolCallIndex]; ok && itemID != "" {
			return itemID
		}
	}
	if tc != nil && tc.ResponseItemID != "" {
		return tc.ResponseItemID
	}
	if s.currentItemID != "" {
		return s.currentItemID
	}
	return generateItemID()
}

func (s *responsesInboundStream) handleFunctionCallDelta(tc llm.ToolCall) error {
	toolCallIndex := tc.Index
	s.toolCalls[toolCallIndex].Function.Arguments += tc.Function.Arguments

	if tc.Function.Arguments != "" {
		itemID := s.toolCallItemID(toolCallIndex, s.toolCalls[toolCallIndex])

		err := s.enqueueEvent(&StreamEvent{
			Type:         StreamEventTypeFunctionCallArgumentsDelta,
			ItemID:       &itemID,
			OutputIndex:  s.toolCallOutputIndex[toolCallIndex],
			ContentIndex: lo.ToPtr(0),
			Delta:        tc.Function.Arguments,
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue function_call_arguments.delta event: %w", err)
		}
	}

	return nil
}

func (s *responsesInboundStream) handleCustomToolCallDelta(tc llm.ToolCall) error {
	toolCallIndex := tc.Index
	tracked := s.toolCalls[toolCallIndex]
	if tracked.ResponseCustomToolCall == nil {
		// Defensive: initToolCall should have bridged Chat custom calls already.
		callID := tracked.ID
		name := ""
		if tc.OpenAIChatCustomToolCall != nil {
			name = tc.OpenAIChatCustomToolCall.Name
		}
		tracked.ResponseCustomToolCall = &llm.ResponseCustomToolCall{
			CallID: callID,
			Name:   name,
		}
	}

	deltaInput := ""
	switch {
	case tc.ResponseCustomToolCall != nil:
		deltaInput = tc.ResponseCustomToolCall.Input
	case tc.OpenAIChatCustomToolCall != nil:
		deltaInput = tc.OpenAIChatCustomToolCall.Input
	}
	tracked.ResponseCustomToolCall.Input += deltaInput

	if deltaInput != "" {
		itemID := s.toolCallItemID(toolCallIndex, tracked)

		err := s.enqueueEvent(&StreamEvent{
			Type:        StreamEventTypeCustomToolCallInputDelta,
			ItemID:      &itemID,
			OutputIndex: s.toolCallOutputIndex[toolCallIndex],
			Delta:       deltaInput,
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue custom_tool_call_input.delta event: %w", err)
		}
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
	hadTextPart := s.hasReasoningTextPart

	if hadTextPart {
		err := s.enqueueEvent(&StreamEvent{
			Type:         StreamEventTypeReasoningTextDone,
			ItemID:       &s.currentItemID,
			OutputIndex:  s.outputIndex,
			ContentIndex: lo.ToPtr(0),
			Text:         fullReasoning,
		})
		if err != nil {
			return fmt.Errorf("failed to enqueue reasoning_text.done event: %w", err)
		}
	}

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
	s.hasReasoningTextPart = false

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
	if hadTextPart && fullReasoning != "" {
		item.ReasoningContent = []ReasoningContent{{
			Type: "reasoning_text",
			Text: fullReasoning,
		}}
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

	// Close any open tool call items
	for idx, tc := range s.toolCalls {
		if !s.toolCallItemStarted[idx] {
			continue
		}

		itemID := s.toolCallItemID(idx, tc)

		switch {
		case tc.ResponseCustomToolCall != nil:
			// Custom tool call - emit custom_tool_call_input.done then output_item.done
			fullInput := tc.ResponseCustomToolCall.Input

			err := s.enqueueEvent(&StreamEvent{
				Type:        StreamEventTypeCustomToolCallInputDone,
				ItemID:      &itemID,
				OutputIndex: s.toolCallOutputIndex[idx],
				Input:       fullInput,
			})
			if err != nil {
				return fmt.Errorf("failed to enqueue custom_tool_call_input.done event: %w", err)
			}

			ctcDoneStatus := tc.Status
			if ctcDoneStatus == "" {
				ctcDoneStatus = "completed"
			}
			item := Item{
				ID:        itemID,
				Type:      "custom_tool_call",
				Status:    lo.ToPtr(ctcDoneStatus),
				CallID:    tc.ResponseCustomToolCall.CallID,
				Name:      tc.ResponseCustomToolCall.Name,
				Namespace: tc.ResponseCustomToolCall.Namespace,
				Input:     lo.ToPtr(fullInput),
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
				Arguments:   tc.Function.Arguments,
			})
			if err != nil {
				return fmt.Errorf("failed to enqueue function_call_arguments.done event: %w", err)
			}

			fcDoneStatus := tc.Status
			if fcDoneStatus == "" {
				fcDoneStatus = "completed"
			}
			// Restore namespace group identity (consistent with initToolCall).
			fcDoneName, fcDoneNamespace := resolveNamespaceFromMetadata(s.transformerMetadata, tc.Function.Name)
			item := Item{
				ID:        itemID,
				Type:      "function_call",
				Status:    lo.ToPtr(fcDoneStatus),
				CallID:    tc.ID,
				Name:      fcDoneName,
				Namespace: fcDoneNamespace,
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
	if s.responseCompleted {
		return nil
	}

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
