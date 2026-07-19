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
	stream streams.Stream[*httpclient.StreamEvent]
	state  *outboundStreamState

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

	// Content accumulation
	textContent      strings.Builder
	reasoningContent strings.Builder

	// Tool call tracking
	toolCalls                map[string]*llm.ToolCall // internal key -> tool call
	itemToToolCallKey        map[string]string        // item.id -> internal key
	callToToolCallKey        map[string]string        // call_id -> internal key
	outputIndexToToolCallKey map[int]string           // output_index -> internal key
	ambiguousToolCallIndexes map[int]bool             // output_index reused by multiple tool calls
	toolCallIdentityEmitted  map[string]bool          // internal key -> whether identity was emitted
	nextToolCallIndex        int

	// Reasoning signature tracking
	pendingReasoningEncryptedContent map[string]*string

	// Transformer metadata tracking
	transformerMetadata        map[string]any
	transformerMetadataEmitted bool
}

func newResponsesOutboundStream(stream streams.Stream[*httpclient.StreamEvent]) *responsesOutboundStream {
	return &responsesOutboundStream{
		stream: stream,
		state: &outboundStreamState{
			toolCalls:                        make(map[string]*llm.ToolCall),
			itemToToolCallKey:                make(map[string]string),
			callToToolCallKey:                make(map[string]string),
			outputIndexToToolCallKey:         make(map[int]string),
			ambiguousToolCallIndexes:         make(map[int]bool),
			toolCallIdentityEmitted:          make(map[string]bool),
			pendingReasoningEncryptedContent: make(map[string]*string),
			transformerMetadata:              make(map[string]any),
		},
	}
}

func toolCallStateKey(callID, itemID string) string {
	if callID != "" {
		return "call:" + callID
	}
	if itemID != "" {
		return "item:" + itemID
	}
	return ""
}

func toolCallMatchesItemType(tc *llm.ToolCall, itemType string) bool {
	if tc == nil || itemType == "" {
		return tc != nil
	}

	switch itemType {
	case "function_call":
		return tc.ResponseCustomToolCall == nil
	case "custom_tool_call":
		return tc.ResponseCustomToolCall != nil
	default:
		return false
	}
}

func (s *responsesOutboundStream) bindToolCallOutputIndex(outputIndex int, key string) {
	if key == "" || s.state.ambiguousToolCallIndexes[outputIndex] {
		return
	}

	existingKey := s.state.outputIndexToToolCallKey[outputIndex]
	if existingKey == "" {
		s.state.outputIndexToToolCallKey[outputIndex] = key
		return
	}
	if existingKey != key {
		delete(s.state.outputIndexToToolCallKey, outputIndex)
		s.state.ambiguousToolCallIndexes[outputIndex] = true
	}
}

func (s *responsesOutboundStream) uniqueToolCallKeyForOutputIndex(outputIndex int) (string, error) {
	if s.state.ambiguousToolCallIndexes[outputIndex] {
		return "", fmt.Errorf("ambiguous tool call output_index %d", outputIndex)
	}
	return s.state.outputIndexToToolCallKey[outputIndex], nil
}

func (s *responsesOutboundStream) ensureToolCallState(
	item *Item,
	outputIndex int,
	fromAddedEvent bool,
) (string, *llm.ToolCall, error) {
	if item == nil {
		return "", nil, nil
	}

	key := ""
	if item.CallID != "" {
		key = s.state.callToToolCallKey[item.CallID]
	}
	if key == "" && item.ID != "" {
		key = s.state.itemToToolCallKey[item.ID]
	}
	if key == "" && !fromAddedEvent {
		candidateKey, err := s.uniqueToolCallKeyForOutputIndex(outputIndex)
		if err != nil {
			return "", nil, err
		}
		candidate := s.state.toolCalls[candidateKey]
		if toolCallMatchesItemType(candidate, item.Type) &&
			(item.CallID == "" || candidate.ID == "" || candidate.ID == item.CallID) &&
			(item.ID == "" || candidate.ResponseItemID == "" || candidate.ResponseItemID == item.ID) {
			key = candidateKey
		}
	}
	if key == "" {
		key = toolCallStateKey(item.CallID, item.ID)
	}
	if key == "" {
		return "", nil, nil
	}

	tc, exists := s.state.toolCalls[key]
	if !exists {
		tc = &llm.ToolCall{
			Index: s.state.nextToolCallIndex,
			Type:  "function",
		}
		s.state.nextToolCallIndex++
		s.state.toolCalls[key] = tc
	}
	s.bindToolCallOutputIndex(outputIndex, key)

	if item.CallID != "" {
		tc.ID = item.CallID
		s.state.callToToolCallKey[item.CallID] = key
	}
	if item.ID != "" {
		// A Responses stream can omit the item id in output_item.added and
		// provide it only in a later terminal snapshot. Once the canonical
		// tool identity has been emitted, changing the item id would split
		// one tool call into two identities for downstream stream consumers.
		if !s.state.toolCallIdentityEmitted[key] {
			tc.ResponseItemID = item.ID
		}
		s.state.itemToToolCallKey[item.ID] = key
	}

	if item.Type == "custom_tool_call" {
		tc.Type = llm.ToolTypeResponsesCustomTool
		if tc.ResponseCustomToolCall == nil {
			tc.ResponseCustomToolCall = &llm.ResponseCustomToolCall{}
		}
		if item.CallID != "" {
			tc.ResponseCustomToolCall.CallID = item.CallID
		}
	}

	if fromAddedEvent && !exists {
		switch item.Type {
		case "function_call":
			tc.Function.Name = item.Name
			tc.Function.Namespace = item.Namespace
			tc.Function.Arguments = item.Arguments
		case "custom_tool_call":
			tc.ResponseCustomToolCall.Name = item.Name
			tc.ResponseCustomToolCall.Namespace = item.Namespace
			if item.Input != nil {
				tc.ResponseCustomToolCall.Input = *item.Input
			}
		}
	}

	return key, tc, nil
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

		case "function_call":
			key, tc, err := s.ensureToolCallState(item, streamEvent.OutputIndex, true)
			if err != nil {
				return err
			}
			if key == "" || tc.ID == "" || s.state.toolCallIdentityEmitted[key] {
				return nil
			}
			s.state.toolCallIdentityEmitted[key] = true

			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						ToolCalls: []llm.ToolCall{
							{
								ID:             tc.ID,
								ResponseItemID: tc.ResponseItemID,
								Type:           "function",
								Index:          tc.Index,
								Function:       tc.Function,
							},
						},
					},
				},
			}

		case "custom_tool_call":
			key, tc, err := s.ensureToolCallState(item, streamEvent.OutputIndex, true)
			if err != nil {
				return err
			}
			if key == "" || tc.ID == "" || s.state.toolCallIdentityEmitted[key] {
				return nil
			}
			s.state.toolCallIdentityEmitted[key] = true
			customToolCall := *tc.ResponseCustomToolCall

			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						ToolCalls: []llm.ToolCall{
							{
								ID:                     tc.ID,
								ResponseItemID:         tc.ResponseItemID,
								Type:                   llm.ToolTypeResponsesCustomTool,
								Index:                  tc.Index,
								ResponseCustomToolCall: &customToolCall,
							},
						},
					},
				},
			}

		default:
			// For other item types (e.g., message), skip - no meaningful content to emit
			return nil // Intentionally skip this event
		}

	case StreamEventTypeFunctionCallArgumentsDelta:
		// Function call arguments delta
		key, err := s.toolCallKeyForStreamEvent(streamEvent)
		if err != nil {
			return err
		}
		if tc, ok := s.state.toolCalls[key]; ok {
			tc.Function.Arguments += streamEvent.Delta
			if !s.state.toolCallIdentityEmitted[key] {
				return nil
			}

			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						ToolCalls: []llm.ToolCall{
							{
								Index: tc.Index,
								Function: llm.FunctionCall{
									Arguments: streamEvent.Delta,
								},
							},
						},
					},
				},
			}
		}

	case StreamEventTypeFunctionCallArgumentsDone:
		key, err := s.toolCallKeyForStreamEvent(streamEvent)
		if err != nil {
			return err
		}
		emitted, err := s.reconcileFunctionCall(resp, key, streamEvent.Name, streamEvent.Namespace, streamEvent.Arguments, false)
		if err != nil {
			return err
		}
		if !emitted {
			return nil
		}

	case StreamEventTypeCustomToolCallInputDelta:
		// Custom tool call input delta - accumulate and emit as tool call delta
		key, err := s.toolCallKeyForStreamEvent(streamEvent)
		if err != nil {
			return err
		}
		if tc, ok := s.state.toolCalls[key]; ok && tc.ResponseCustomToolCall != nil {
			tc.ResponseCustomToolCall.Input += streamEvent.Delta
			if !s.state.toolCallIdentityEmitted[key] {
				return nil
			}

			resp.Choices = []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						ToolCalls: []llm.ToolCall{
							{
								Index: tc.Index,
								Type:  llm.ToolTypeResponsesCustomTool,
								ResponseCustomToolCall: &llm.ResponseCustomToolCall{
									CallID:    tc.ID,
									Name:      tc.ResponseCustomToolCall.Name,
									Namespace: tc.ResponseCustomToolCall.Namespace,
									Input:     streamEvent.Delta,
								},
							},
						},
					},
				},
			}
		}

	case StreamEventTypeCustomToolCallInputDone:
		key, err := s.toolCallKeyForStreamEvent(streamEvent)
		if err != nil {
			return err
		}
		emitted, err := s.reconcileCustomToolCall(resp, key, "", "", streamEvent.Input, false)
		if err != nil {
			return err
		}
		if !emitted {
			return nil
		}

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

	case StreamEventTypeOutputItemDone:
		if streamEvent.Item == nil {
			return nil // Intentionally skip this event
		}
		if streamEvent.Item.Type == "function_call" || streamEvent.Item.Type == "custom_tool_call" {
			emitted, err := s.reconcileFinalToolItem(resp, streamEvent.OutputIndex, streamEvent.Item)
			if err != nil {
				return err
			}
			if !emitted {
				return nil
			}
			break
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
			for i := range streamEvent.Response.Output {
				item := &streamEvent.Response.Output[i]
				if item.Type != "function_call" && item.Type != "custom_tool_call" {
					continue
				}
				deltaResp := s.newResponseChunk()
				emitted, err := s.reconcileFinalToolItem(deltaResp, i, item)
				if err != nil {
					return err
				}
				if emitted {
					s.enqueue(deltaResp)
				}
			}
			s.state.previousResponseID = streamEvent.Response.PreviousResponseID
			resp.PreviousResponseID = s.state.previousResponseID
		}
		if len(s.state.transformerMetadata) > 0 && !s.state.transformerMetadataEmitted {
			resp.TransformerMetadata = s.state.transformerMetadata
			s.state.transformerMetadataEmitted = true
		}

		finishReason := "stop"
		if s.hasEmittedToolCall() {
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

func (s *responsesOutboundStream) toolCallKeyForStreamEvent(event StreamEvent) (string, error) {
	key := ""
	if event.CallID != "" {
		key = s.state.callToToolCallKey[event.CallID]
	}
	if key == "" && event.ItemID != nil {
		key = s.state.itemToToolCallKey[*event.ItemID]
		if key == "" {
			key = s.state.callToToolCallKey[*event.ItemID]
		}
	}
	if key == "" {
		var err error
		key, err = s.uniqueToolCallKeyForOutputIndex(event.OutputIndex)
		if err != nil {
			return "", err
		}
	}
	if key == "" {
		return "", nil
	}

	if event.CallID != "" {
		s.state.callToToolCallKey[event.CallID] = key
	}
	if event.ItemID != nil && *event.ItemID != "" {
		s.state.itemToToolCallKey[*event.ItemID] = key
	}

	return key, nil
}

func (s *responsesOutboundStream) hasEmittedToolCall() bool {
	for _, emitted := range s.state.toolCallIdentityEmitted {
		if emitted {
			return true
		}
	}
	return false
}

func (s *responsesOutboundStream) beginTerminalEvent() bool {
	if s.responseCompleted {
		return false
	}
	s.responseCompleted = true
	return true
}

func finalStreamDelta(current, final string) (string, error) {
	if final == "" || final == current {
		return "", nil
	}
	if strings.HasPrefix(final, current) {
		return final[len(current):], nil
	}
	return "", fmt.Errorf("final value does not extend streamed value")
}

func (s *responsesOutboundStream) reconcileFinalToolItem(
	resp *llm.Response,
	outputIndex int,
	item *Item,
) (bool, error) {
	key, tc, err := s.ensureToolCallState(item, outputIndex, false)
	if err != nil {
		return false, err
	}
	if key == "" || tc == nil {
		return false, nil
	}
	if tc.ID == "" {
		return false, fmt.Errorf("final %s item %q is missing call_id", item.Type, item.ID)
	}
	includeIdentity := !s.state.toolCallIdentityEmitted[key]

	switch item.Type {
	case "function_call":
		return s.reconcileFunctionCall(resp, key, item.Name, item.Namespace, item.Arguments, includeIdentity)
	case "custom_tool_call":
		finalInput := ""
		if item.Input != nil {
			finalInput = *item.Input
		}
		return s.reconcileCustomToolCall(resp, key, item.Name, item.Namespace, finalInput, includeIdentity)
	default:
		return false, nil
	}
}

func (s *responsesOutboundStream) reconcileFunctionCall(
	resp *llm.Response,
	key, name, namespace, finalArguments string,
	includeIdentity bool,
) (bool, error) {
	tc, ok := s.state.toolCalls[key]
	if !ok {
		return false, nil
	}

	nameChanged := name != "" && name != tc.Function.Name
	namespaceChanged := namespace != "" && namespace != tc.Function.Namespace
	if nameChanged {
		tc.Function.Name = name
	}
	if namespaceChanged {
		tc.Function.Namespace = namespace
	}

	delta, err := finalStreamDelta(tc.Function.Arguments, finalArguments)
	if err != nil {
		return false, fmt.Errorf("invalid final arguments for function call %q: %w", tc.ID, err)
	}
	if finalArguments != "" {
		tc.Function.Arguments = finalArguments
	}
	if !s.state.toolCallIdentityEmitted[key] && !includeIdentity {
		return false, nil
	}
	if delta == "" && !nameChanged && !namespaceChanged && !includeIdentity {
		return false, nil
	}

	arguments := delta
	if includeIdentity {
		arguments = tc.Function.Arguments
	}
	functionDelta := llm.FunctionCall{Arguments: arguments}
	if includeIdentity || nameChanged {
		functionDelta.Name = tc.Function.Name
	}
	if includeIdentity || namespaceChanged {
		functionDelta.Namespace = tc.Function.Namespace
	}

	toolCallDelta := llm.ToolCall{
		Index:    tc.Index,
		Function: functionDelta,
	}
	if includeIdentity {
		toolCallDelta.ID = tc.ID
		toolCallDelta.ResponseItemID = tc.ResponseItemID
		toolCallDelta.Type = "function"
	}

	resp.Choices = []llm.Choice{
		{
			Index: 0,
			Delta: &llm.Message{
				ToolCalls: []llm.ToolCall{toolCallDelta},
			},
		},
	}
	if includeIdentity {
		s.state.toolCallIdentityEmitted[key] = true
	}
	return true, nil
}

func (s *responsesOutboundStream) reconcileCustomToolCall(
	resp *llm.Response,
	key, name, namespace, finalInput string,
	includeIdentity bool,
) (bool, error) {
	tc, ok := s.state.toolCalls[key]
	if !ok || tc.ResponseCustomToolCall == nil {
		return false, nil
	}

	nameChanged := name != "" && name != tc.ResponseCustomToolCall.Name
	namespaceChanged := namespace != "" && namespace != tc.ResponseCustomToolCall.Namespace
	if nameChanged {
		tc.ResponseCustomToolCall.Name = name
	}
	if namespaceChanged {
		tc.ResponseCustomToolCall.Namespace = namespace
	}
	delta, err := finalStreamDelta(tc.ResponseCustomToolCall.Input, finalInput)
	if err != nil {
		return false, fmt.Errorf("invalid final input for custom tool call %q: %w", tc.ID, err)
	}
	if finalInput != "" {
		tc.ResponseCustomToolCall.Input = finalInput
	}
	if !s.state.toolCallIdentityEmitted[key] && !includeIdentity {
		return false, nil
	}
	if delta == "" && !nameChanged && !namespaceChanged && !includeIdentity {
		return false, nil
	}

	input := delta
	if includeIdentity {
		input = tc.ResponseCustomToolCall.Input
	}
	customToolDelta := &llm.ResponseCustomToolCall{
		CallID: tc.ID,
		Input:  input,
	}
	if includeIdentity || nameChanged {
		customToolDelta.Name = tc.ResponseCustomToolCall.Name
	}
	if includeIdentity || namespaceChanged {
		customToolDelta.Namespace = tc.ResponseCustomToolCall.Namespace
	}

	toolCallDelta := llm.ToolCall{
		Index:                  tc.Index,
		Type:                   llm.ToolTypeResponsesCustomTool,
		ResponseCustomToolCall: customToolDelta,
	}
	if includeIdentity {
		toolCallDelta.ID = tc.ID
		toolCallDelta.ResponseItemID = tc.ResponseItemID
	}

	resp.Choices = []llm.Choice{
		{
			Index: 0,
			Delta: &llm.Message{
				ToolCalls: []llm.ToolCall{toolCallDelta},
			},
		},
	}
	if includeIdentity {
		s.state.toolCallIdentityEmitted[key] = true
	}
	return true, nil
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
