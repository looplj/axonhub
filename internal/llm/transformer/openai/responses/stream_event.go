package responses

type StreamEventType string

const (
	StremEventTypeError StreamEventType = "error"
	// Response events.

	StreamEventTypeResponseCreated    StreamEventType = "response.created"
	StreamEventTypeResponseInProgress StreamEventType = "response.in_progress"
	StreamEventTypeResponseCompleted  StreamEventType = "response.completed"
	StreamEventTypeResponseQueued     StreamEventType = "response.queued"
	StreamEventTypeResponseFailed     StreamEventType = "response.failed"
	StreamEventTypeResponseIncomplete StreamEventType = "response.incomplete"

	// Item events.

	StreamEventTypeItemAdded StreamEventType = "response.output_item.added"
	StreamEventTypeItemDone  StreamEventType = "response.output_item.done"

	// Content part events.

	StreamEventTypeContentPartAdded StreamEventType = "response.output_item.content_part.added"
	StreamEventTypeContentPartDone  StreamEventType = "response.output_item.content_part.done"

	// Output text events.

	StreamEventTypeOutputTextDelta StreamEventType = "response.output_item.output_text.delta"
	StreamEventTypeOutputTextDone  StreamEventType = "response.output_item.output_text.done"

	// Image generation events.

	StreamEventTypeImageGenerationGenerating   StreamEventType = "response.image_generation_call.generating"
	StreamEventTypeImageGenerationInProgress   StreamEventType = "response.image_generation_call.in_progress"
	StreamEventTypeImageGenerationPartialImage StreamEventType = "response.image_generation_call.partial_image"
	StreamEventTypeImageGenerationCompleted    StreamEventType = "response.image_generation_call.completed"
)

type StreamEvent struct {
	Type           StreamEventType `json:"type"`
	SequenceNumber int             `json:"sequence_number"`
	Response       *Response       `json:"response,omitempty"`

	OutputIndex int    `json:"output_index,omitempty"`
	Item        *Input `json:"item,omitempty"`

	// For content_part, output_text, image_generation events
	ItemID *string `json:"item_id,omitempty"`
	// For content_part, output_text events.
	ContentIndex *int `json:"content_index,omitempty"`
	// For content_part events
	Part *StreamEventContentPart `json:"part,omitempty"`

	// For output_text delta events.
	Delta string `json:"delta,omitempty"`
	// For output_text done events.
	Text string `json:"text,omitempty"`

	// For image_generation partial_image events.
	PartialImageB64 string `json:"partial_image_b64,omitempty"`
	// For image_generation partial_image events.
	PartialImageIndex *int `json:"partial_image_index,omitempty"`

	// For error events
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Param   string `json:"param,omitempty"`
}

type StreamEventContentPart struct {
	// Any of "output_text", "reasoning_text", "refusal".
	Type string `json:"type"`
	// The text of the part, for output_text and reasoning_text.
	Text *string `json:"text,omitempty"`
	// The refusal reason, for refusal.
	Refusal *string `json:"refusal,omitempty"`
}
