package llm

type TransformOptions struct {
	// ArrayInstructions specifies whether the system instructions is an array.
	ArrayInstructions *bool `json:"array_instructions,omitempty"`

	// ArrayInputs specifies whether the inputs is an array.
	ArrayInputs *bool `json:"array_inputs,omitempty"`

	// DowngradeMidConversationSystem specifies whether mid-conversation system messages
	// (e.g. Claude Code reminders) are downgraded to user. OpenAI-compatible upstreams
	// hoist all system messages to the front of the prompt, so a newly injected reminder
	// rewrites the whole system prefix and defeats prompt caching. Downgrading them keeps
	// the system prefix stable across turns. true = enabled, nil/false = disabled (default).
	DowngradeMidConversationSystem *bool `json:"downgrade_mid_conversation_system,omitempty"`
}
