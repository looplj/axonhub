package responses

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	openAIResponsesReplayModeRaw           = "raw"
	openAIResponsesReplayModeMergeOnly     = "merge_only"
	openAIResponsesReplayModeSyntheticOnly = "synthetic_only"
)

// OpenAIResponsesStreamReplayState centralizes sequence allocation and terminal
// dedupe for mixed raw replay and synthetic Responses SSE events.
type OpenAIResponsesStreamReplayState struct {
	NextSequenceNumber        int
	Started                   bool
	TerminalEmitted           bool
	RawCompletedReplayed      bool
	SyntheticCompletedEmitted bool
	RawOnlyEventSeen          bool
	OutputItemsByID           map[string]OpenAIResponsesOutputItemState
	RawEventsBySequence       map[int]llm.OpenAIResponsesRawEvent
	SemanticEventsByPath      map[string]struct{}
	LastStructuredResponse    *llm.Response
}

type OpenAIResponsesOutputItemState struct {
	ID          string
	OutputIndex int
	ItemType    string
}

func newOpenAIResponsesStreamReplayState() *OpenAIResponsesStreamReplayState {
	return &OpenAIResponsesStreamReplayState{
		OutputItemsByID:      map[string]OpenAIResponsesOutputItemState{},
		RawEventsBySequence:  map[int]llm.OpenAIResponsesRawEvent{},
		SemanticEventsByPath: map[string]struct{}{},
	}
}

func (s *OpenAIResponsesStreamReplayState) nextSequenceNumber() int {
	if s == nil {
		return 0
	}

	sequence := s.NextSequenceNumber
	s.NextSequenceNumber++

	return sequence
}

func buildOpenAIResponsesRawEvent(
	event *httpclient.StreamEvent,
	streamEvent *StreamEvent,
	replayMode string,
	semanticPath string,
) *llm.OpenAIResponsesRawEvent {
	if event == nil {
		return nil
	}

	raw := cloneRaw(event.Data)
	eventType := event.Type
	var extra map[string]json.RawMessage
	var sequenceNumber *int

	if streamEvent != nil {
		raw = cloneRaw(streamEvent.Raw)
		if streamEvent.Type != "" {
			eventType = string(streamEvent.Type)
		}
		extra = cloneRawMap(streamEvent.Extra)
		sequenceNumber = streamEventSequenceNumber(streamEvent.Raw)
	}

	if eventType == "" {
		eventType = rawEventType(raw)
	}

	return &llm.OpenAIResponsesRawEvent{
		Type:           eventType,
		SSEType:        event.Type,
		LastEventID:    event.LastEventID,
		SequenceNumber: sequenceNumber,
		DataRaw:        cloneRaw(event.Data),
		Raw:            raw,
		Extra:          extra,
		SemanticPath:   semanticPath,
		ReplayMode:     replayMode,
	}
}

func attachOpenAIResponsesRawStreamEvent(resp *llm.Response, rawEvent *llm.OpenAIResponsesRawEvent) {
	if resp == nil || rawEvent == nil {
		return
	}

	providerExt := llm.EnsureOpenAIResponsesResponseProviderExtensions(resp)
	if providerExt == nil {
		return
	}

	providerExt.Stream = &llm.OpenAIResponsesStreamExtensions{
		RawEvent: cloneRawStreamEvent(rawEvent),
	}
}

func streamRawEvent(resp *llm.Response) *llm.OpenAIResponsesRawEvent {
	if resp == nil || resp.ProviderExtensions == nil ||
		resp.ProviderExtensions.OpenAIResponses == nil ||
		resp.ProviderExtensions.OpenAIResponses.Stream == nil {
		return nil
	}

	return resp.ProviderExtensions.OpenAIResponses.Stream.RawEvent
}

func cloneRawStreamEvent(src *llm.OpenAIResponsesRawEvent) *llm.OpenAIResponsesRawEvent {
	if src == nil {
		return nil
	}

	cloned := *src
	if src.SequenceNumber != nil {
		sequenceNumber := *src.SequenceNumber
		cloned.SequenceNumber = &sequenceNumber
	}
	cloned.DataRaw = cloneRaw(src.DataRaw)
	cloned.Raw = cloneRaw(src.Raw)
	cloned.Extra = cloneRawMap(src.Extra)

	return &cloned
}

func streamEventSequenceNumber(raw json.RawMessage) *int {
	value := rawField(raw, "sequence_number")
	if len(value) == 0 {
		return nil
	}

	var sequenceNumber int
	if err := json.Unmarshal(value, &sequenceNumber); err != nil {
		return nil
	}

	return &sequenceNumber
}

func rawEventType(raw json.RawMessage) string {
	value := rawField(raw, "type")
	if len(value) == 0 {
		return ""
	}

	var eventType string
	if err := json.Unmarshal(value, &eventType); err != nil {
		return ""
	}

	return eventType
}

func isResponsesTerminalEvent(eventType StreamEventType) bool {
	switch eventType {
	case StreamEventTypeResponseCompleted, StreamEventTypeResponseFailed, StreamEventTypeResponseIncomplete:
		return true
	default:
		return false
	}
}

func hasRawOnlyResponseOutput(resp *Response) bool {
	if resp == nil {
		return false
	}

	for _, item := range resp.Output {
		if !isKnownOutputItemType(item.Type) {
			return true
		}
	}

	return false
}

func (s *responsesInboundStream) enqueueRawReplayEvent(
	rawEvent *llm.OpenAIResponsesRawEvent,
	chunk *llm.Response,
) error {
	if rawEvent == nil || rawEvent.ReplayMode != openAIResponsesReplayModeRaw {
		return nil
	}

	eventType := StreamEventType(rawEvent.Type)
	if isResponsesTerminalEvent(eventType) {
		if s.replayState != nil && s.replayState.TerminalEmitted {
			return nil
		}
	}

	data, err := s.rawReplayEventData(rawEvent, chunk)
	if err != nil {
		return err
	}

	if s.replayState != nil {
		s.replayState.RawOnlyEventSeen = true
		if rawEvent.SequenceNumber != nil {
			s.replayState.RawEventsBySequence[*rawEvent.SequenceNumber] = *cloneRawStreamEvent(rawEvent)
		}
		if isResponsesTerminalEvent(eventType) {
			s.replayState.TerminalEmitted = true
			if eventType == StreamEventTypeResponseCompleted {
				s.replayState.RawCompletedReplayed = true
			}
		}
	}

	streamType := rawEvent.SSEType
	if streamType == "" {
		streamType = rawEvent.Type
	}

	s.eventQueue = append(s.eventQueue, &httpclient.StreamEvent{
		LastEventID: rawEvent.LastEventID,
		Type:        streamType,
		Data:        data,
	})

	if s.aggregator == nil {
		s.aggregator = newStreamAggregator()
	}

	var ev StreamEvent
	if err := json.Unmarshal(data, &ev); err == nil {
		s.aggregator.processEvent(&ev)
	}

	if isResponsesTerminalEvent(eventType) {
		s.responseCompleted = true
	}

	return nil
}

func (s *responsesInboundStream) rawReplayEventData(
	rawEvent *llm.OpenAIResponsesRawEvent,
	chunk *llm.Response,
) ([]byte, error) {
	raw := rawEvent.Raw
	if len(raw) == 0 {
		raw = rawEvent.DataRaw
	}

	obj, err := decodeRawObject(raw)
	if err != nil {
		return nil, err
	}

	sequenceNumber, err := json.Marshal(s.nextSequenceNumber())
	if err != nil {
		return nil, err
	}
	obj["sequence_number"] = sequenceNumber

	if rawEvent.Type != "" {
		eventType, err := json.Marshal(rawEvent.Type)
		if err != nil {
			return nil, err
		}
		obj["type"] = eventType
	}

	s.overlayRawResponseEvent(obj, rawEvent, chunk)

	return json.Marshal(obj)
}

func (s *responsesInboundStream) overlayRawResponseEvent(
	obj map[string]json.RawMessage,
	rawEvent *llm.OpenAIResponsesRawEvent,
	chunk *llm.Response,
) {
	eventType := StreamEventType(rawEvent.Type)
	switch eventType {
	case StreamEventTypeResponseCreated, StreamEventTypeResponseInProgress,
		StreamEventTypeResponseQueued, StreamEventTypeResponseCompleted,
		StreamEventTypeResponseFailed, StreamEventTypeResponseIncomplete:
	default:
		return
	}

	responseObj := map[string]json.RawMessage{}
	if rawResponse, ok := obj["response"]; ok && len(rawResponse) > 0 {
		if decoded, err := decodeRawObject(rawResponse); err == nil {
			responseObj = decoded
		}
	}

	overlayString(responseObj, "object", "response")
	overlayString(responseObj, "id", firstNonEmpty(chunkString(chunk, func(resp *llm.Response) string { return resp.ID }), s.responseID))
	overlayString(responseObj, "model", firstNonEmpty(chunkString(chunk, func(resp *llm.Response) string { return resp.Model }), s.model))

	createdAt := s.createdAt
	if chunk != nil && chunk.Created != 0 {
		createdAt = chunk.Created
	}
	if createdAt != 0 {
		overlayJSON(responseObj, "created_at", createdAt)
	}

	if previousResponseID := firstResponseID(chunk, s); previousResponseID != nil {
		overlayJSON(responseObj, "previous_response_id", previousResponseID)
	}

	if status := statusForRawResponseEvent(eventType); status != "" {
		overlayString(responseObj, "status", status)
	}

	usage := s.usage
	if chunk != nil && chunk.Usage != nil {
		usage = chunk.Usage
	}
	if usage != nil {
		overlayJSON(responseObj, "usage", ConvertLLMUsageToResponsesUsage(usage))
	}

	if eventType == StreamEventTypeResponseCompleted && s.aggregator != nil {
		overlayJSON(responseObj, "output", s.completedResponseOutput(responseObj))
	}

	if encoded, err := json.Marshal(responseObj); err == nil {
		obj["response"] = encoded
	}
}

func (s *responsesInboundStream) completedResponseOutput(responseObj map[string]json.RawMessage) []Item {
	structuredOutput := s.aggregator.buildResponse().Output
	rawOutput := responseObj["output"]
	if len(rawOutput) == 0 {
		return structuredOutput
	}

	var rawItems []Item
	if err := json.Unmarshal(rawOutput, &rawItems); err != nil {
		return structuredOutput
	}

	knownByKey, contentExtraByKey, rawTopLevel := splitResponseRawOutputItems(buildOutputRawItems(rawItems))
	output := applyKnownOutputExtras(structuredOutput, knownByKey, contentExtraByKey)
	output = appendRawOnlyOutputItems(output, rawTopLevel)

	return dedupeOutputItemsByID(output)
}

func overlayString(obj map[string]json.RawMessage, key string, value string) {
	if value == "" {
		return
	}

	overlayJSON(obj, key, value)
}

func overlayJSON(obj map[string]json.RawMessage, key string, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}

	obj[key] = encoded
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func chunkString(chunk *llm.Response, getter func(*llm.Response) string) string {
	if chunk == nil {
		return ""
	}

	return getter(chunk)
}

func firstResponseID(chunk *llm.Response, s *responsesInboundStream) *string {
	if chunk != nil && chunk.PreviousResponseID != nil {
		return chunk.PreviousResponseID
	}
	if s != nil {
		return s.previousResponseID
	}

	return nil
}

func statusForRawResponseEvent(eventType StreamEventType) string {
	switch eventType {
	case StreamEventTypeResponseCreated, StreamEventTypeResponseInProgress:
		return "in_progress"
	case StreamEventTypeResponseQueued:
		return "queued"
	case StreamEventTypeResponseCompleted:
		return "completed"
	case StreamEventTypeResponseFailed:
		return "failed"
	case StreamEventTypeResponseIncomplete:
		return "incomplete"
	default:
		return ""
	}
}
