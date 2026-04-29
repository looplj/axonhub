package llm

import "encoding/json"

// ProtocolExtensions carries protocol-specific lossless data across transformers.
// It must stay protocol-neutral enough to avoid importing transformer packages from llm.
type ProtocolExtensions struct {
	OpenAIResponses *OpenAIResponsesExtensions `json:"openai_responses,omitempty"`
}

// OpenAIResponsesExtensions stores lossless OpenAI Responses data alongside the semantic llm view.
type OpenAIResponsesExtensions struct {
	RequestExtra map[string]json.RawMessage `json:"request_extra,omitempty"`
	InputItems   []OpenAIResponsesRawItem   `json:"input_items,omitempty"`
	Tools        []OpenAIResponsesRawItem   `json:"tools,omitempty"`

	ResponseRaw      json.RawMessage            `json:"response_raw,omitempty"`
	ResponseExtra    map[string]json.RawMessage `json:"response_extra,omitempty"`
	ResponseMetadata map[string]string          `json:"response_metadata,omitempty"`
	OutputItems      []OpenAIResponsesRawItem   `json:"output_items,omitempty"`

	RawEvent *OpenAIResponsesRawEvent `json:"raw_event,omitempty"`

	// Dirty is the legacy broad invalidation flag; the specific flags below keep safe raw data reusable.
	Dirty               bool `json:"dirty,omitempty"`
	InputDirty          bool `json:"input_dirty,omitempty"`
	ToolsDirty          bool `json:"tools_dirty,omitempty"`
	ResponseOutputDirty bool `json:"response_output_dirty,omitempty"`
	ResponseDirty       bool `json:"response_dirty,omitempty"`
	StreamDirty         bool `json:"stream_dirty,omitempty"`
}

// OpenAIResponsesRawItem stores an ordered raw Responses tool/input/output item.
type OpenAIResponsesRawItem struct {
	Type string          `json:"type,omitempty"`
	ID   string          `json:"id,omitempty"`
	Raw  json.RawMessage `json:"raw,omitempty"`
}

// OpenAIResponsesRawEvent stores an original Responses SSE event.
type OpenAIResponsesRawEvent struct {
	Type           string          `json:"type,omitempty"`
	SequenceNumber *int            `json:"sequence_number,omitempty"`
	Raw            json.RawMessage `json:"raw,omitempty"`
}

// CloneProtocolExtensions returns a detached copy for request variants.
func CloneProtocolExtensions(ext *ProtocolExtensions) *ProtocolExtensions {
	if ext == nil {
		return nil
	}

	return &ProtocolExtensions{
		OpenAIResponses: cloneOpenAIResponsesExtensions(ext.OpenAIResponses),
	}
}

func cloneOpenAIResponsesExtensions(ext *OpenAIResponsesExtensions) *OpenAIResponsesExtensions {
	if ext == nil {
		return nil
	}

	return &OpenAIResponsesExtensions{
		RequestExtra:        cloneRawMap(ext.RequestExtra),
		InputItems:          cloneOpenAIResponsesRawItems(ext.InputItems),
		Tools:               cloneOpenAIResponsesRawItems(ext.Tools),
		ResponseRaw:         cloneRawMessage(ext.ResponseRaw),
		ResponseExtra:       cloneRawMap(ext.ResponseExtra),
		ResponseMetadata:    cloneStringMap(ext.ResponseMetadata),
		OutputItems:         cloneOpenAIResponsesRawItems(ext.OutputItems),
		RawEvent:            cloneOpenAIResponsesRawEvent(ext.RawEvent),
		Dirty:               ext.Dirty,
		InputDirty:          ext.InputDirty,
		ToolsDirty:          ext.ToolsDirty,
		ResponseOutputDirty: ext.ResponseOutputDirty,
		ResponseDirty:       ext.ResponseDirty,
		StreamDirty:         ext.StreamDirty,
	}
}

func cloneOpenAIResponsesRawItems(items []OpenAIResponsesRawItem) []OpenAIResponsesRawItem {
	if len(items) == 0 {
		return nil
	}

	cloned := make([]OpenAIResponsesRawItem, len(items))
	for i, item := range items {
		cloned[i] = OpenAIResponsesRawItem{
			Type: item.Type,
			ID:   item.ID,
			Raw:  cloneRawMessage(item.Raw),
		}
	}

	return cloned
}

func cloneOpenAIResponsesRawEvent(event *OpenAIResponsesRawEvent) *OpenAIResponsesRawEvent {
	if event == nil {
		return nil
	}

	cloned := &OpenAIResponsesRawEvent{
		Type: event.Type,
		Raw:  cloneRawMessage(event.Raw),
	}
	if event.SequenceNumber != nil {
		seq := *event.SequenceNumber
		cloned.SequenceNumber = &seq
	}

	return cloned
}

func cloneRawMessage(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return nil
	}

	return append(json.RawMessage(nil), data...)
}

func cloneRawMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]json.RawMessage, len(src))
	for key, value := range src {
		dst[key] = cloneRawMessage(value)
	}

	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}

	return dst
}

// EnsureOpenAIResponsesExtensions creates the Responses extension holder when needed.
func EnsureOpenAIResponsesExtensions(ext *ProtocolExtensions) *OpenAIResponsesExtensions {
	if ext == nil {
		return nil
	}

	if ext.OpenAIResponses == nil {
		ext.OpenAIResponses = &OpenAIResponsesExtensions{}
	}

	return ext.OpenAIResponses
}

// MarkOpenAIResponsesInputDirty marks request input as semantically changed.
func MarkOpenAIResponsesInputDirty(req *Request) {
	if req == nil {
		return
	}

	if req.ProtocolExtensions == nil {
		req.ProtocolExtensions = &ProtocolExtensions{}
	}

	EnsureOpenAIResponsesExtensions(req.ProtocolExtensions).InputDirty = true
}

// MarkOpenAIResponsesToolsDirty marks request tools as semantically changed.
func MarkOpenAIResponsesToolsDirty(req *Request) {
	if req == nil {
		return
	}

	if req.ProtocolExtensions == nil {
		req.ProtocolExtensions = &ProtocolExtensions{}
	}

	EnsureOpenAIResponsesExtensions(req.ProtocolExtensions).ToolsDirty = true
}

// OpenAIResponsesInputDirty reports whether request input must be rebuilt semantically.
func OpenAIResponsesInputDirty(ext *ProtocolExtensions) bool {
	return ext != nil && ext.OpenAIResponses != nil &&
		(ext.OpenAIResponses.Dirty || ext.OpenAIResponses.InputDirty)
}

// OpenAIResponsesToolsDirty reports whether request tools must be rebuilt semantically.
func OpenAIResponsesToolsDirty(ext *ProtocolExtensions) bool {
	return ext != nil && ext.OpenAIResponses != nil &&
		(ext.OpenAIResponses.Dirty || ext.OpenAIResponses.ToolsDirty)
}

// OpenAIResponsesOutputDirty reports whether response output must be rebuilt semantically.
func OpenAIResponsesOutputDirty(ext *ProtocolExtensions) bool {
	return ext != nil && ext.OpenAIResponses != nil &&
		(ext.OpenAIResponses.Dirty || ext.OpenAIResponses.ResponseOutputDirty)
}
