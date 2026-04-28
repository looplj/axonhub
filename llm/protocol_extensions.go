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
	OutputItems  []OpenAIResponsesRawItem   `json:"output_items,omitempty"`
	RawEvent     *OpenAIResponsesRawEvent   `json:"raw_event,omitempty"`
	Dirty        bool                       `json:"dirty,omitempty"`
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
