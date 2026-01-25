package antigravity

// AntigravityEnvelope acts as a project-aware gateway wrapper.
// All standard LLM payloads must be wrapped in this envelope.
type AntigravityEnvelope struct {
	// Project is the resolved Google Cloud Project ID.
	Project string `json:"project"`

	// Model is the Antigravity model ID.
	// The provider is inferred from this ID (e.g., "claude-..." implies Anthropic, "gemini-..." implies Google).
	Model string `json:"model"`

	// Request is the provider-specific payload (e.g., Gemini Format for Gemini/Claude models via Antigravity).
	Request interface{} `json:"request"`
}
