package llm

type TransformOptions struct {
	// ArrayInstructions specifies whether the system instructions is an array.
	ArrayInstructions *bool `json:"array_instructions,omitempty"`

	// ArrayInputs specifies whether the inputs is an array.
	ArrayInputs *bool `json:"array_inputs,omitempty"`

	// DisableResponsesChatCompat disables the beta Responses-to-Chat protocol
	// conversion for this request, falling back to the legacy generic conversion.
	// Set by the orchestrator when the selected channel has not enabled
	// EnableResponsesChatCompat in its transform options. The zero value leaves
	// the beta conversion available, so request paths that must respect the
	// channel setting have to be stamped by the orchestrator.
	DisableResponsesChatCompat bool `json:"disable_responses_chat_compat,omitempty"`
}
