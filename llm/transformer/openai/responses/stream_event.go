package responses

import "encoding/json"

// StreamEventType defines the type of streaming events for the OpenAI Responses API.
type StreamEventType string

const (
	StreamEventTypeError StreamEventType = "error"

	// Response lifecycle events.

	StreamEventTypeResponseCreated    StreamEventType = "response.created"
	StreamEventTypeResponseInProgress StreamEventType = "response.in_progress"
	StreamEventTypeResponseCompleted  StreamEventType = "response.completed"
	StreamEventTypeResponseQueued     StreamEventType = "response.queued"
	StreamEventTypeResponseFailed     StreamEventType = "response.failed"
	StreamEventTypeResponseIncomplete StreamEventType = "response.incomplete"

	// Output item events.

	StreamEventTypeOutputItemAdded StreamEventType = "response.output_item.added"
	StreamEventTypeOutputItemDone  StreamEventType = "response.output_item.done"

	// Content part events.

	StreamEventTypeContentPartAdded StreamEventType = "response.content_part.added"
	StreamEventTypeContentPartDone  StreamEventType = "response.content_part.done"

	// Output text events.

	StreamEventTypeOutputTextDelta StreamEventType = "response.output_text.delta"
	StreamEventTypeOutputTextDone  StreamEventType = "response.output_text.done"

	// Function call events.

	StreamEventTypeFunctionCallArgumentsDelta StreamEventType = "response.function_call_arguments.delta"
	StreamEventTypeFunctionCallArgumentsDone  StreamEventType = "response.function_call_arguments.done"

	// Reasoning events.

	StreamEventTypeReasoningSummaryPartAdded StreamEventType = "response.reasoning_summary_part.added"
	StreamEventTypeReasoningSummaryPartDone  StreamEventType = "response.reasoning_summary_part.done"
	StreamEventTypeReasoningSummaryTextDelta StreamEventType = "response.reasoning_summary_text.delta"
	StreamEventTypeReasoningSummaryTextDone  StreamEventType = "response.reasoning_summary_text.done"

	// Image generation events.

	StreamEventTypeImageGenerationGenerating   StreamEventType = "response.image_generation_call.generating"
	StreamEventTypeImageGenerationInProgress   StreamEventType = "response.image_generation_call.in_progress"
	StreamEventTypeImageGenerationPartialImage StreamEventType = "response.image_generation_call.partial_image"
	StreamEventTypeImageGenerationCompleted    StreamEventType = "response.image_generation_call.completed"
)

// StreamEvent represents a streaming event from the OpenAI Responses API.
// Reference: https://platform.openai.com/docs/api-reference/responses-streaming
type StreamEvent struct {
	// Common fields
	Type           StreamEventType `json:"type"`
	SequenceNumber int             `json:"sequence_number"`

	// For response.* events
	Response *Response `json:"response,omitempty"`

	// For output_item.* events
	OutputIndex int   `json:"output_index"`
	Item        *Item `json:"item,omitempty"`

	// For content_part.*, output_text.*, function_call_arguments.* events
	ItemID       *string `json:"item_id,omitempty"`
	ContentIndex *int    `json:"content_index,omitempty"`

	// For content_part.added/done events
	Part *StreamEventContentPart `json:"part,omitempty"`

	// For output_text.delta and function_call_arguments.delta events
	Delta string `json:"delta,omitempty"`

	// For output_text.done events
	Text string `json:"text,omitempty"`

	// For function_call_arguments.done events
	Name      string `json:"name,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Arguments string `json:"arguments,omitempty"`

	// For reasoning_summary_* events
	SummaryIndex *int `json:"summary_index,omitempty"`

	// For image_generation_call.partial_image events
	PartialImageB64   string `json:"partial_image_b64,omitempty"`
	PartialImageIndex *int   `json:"partial_image_index,omitempty"`

	// For error events
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Param   string `json:"param,omitempty"`
}

// StreamEventContentPart represents a content part in streaming events.
type StreamEventContentPart struct {
	// Any of "output_text", "reasoning", "refusal".
	Type string `json:"type"`
	// The text of the part, for output_text.
	Text *string `json:"text,omitempty"`
	// The refusal reason, for refusal.
	Refusal *string `json:"refusal,omitempty"`
}

// MarshalStreamEvent marshals a StreamEvent to JSON bytes suitable for SSE.
func MarshalStreamEvent(ev *StreamEvent) ([]byte, error) {
	return json.Marshal(ev)
}
