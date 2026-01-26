package antigravity

import "github.com/google/uuid"

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

	// RequestType indicates the type of request (e.g., "agent")
	RequestType string `json:"requestType,omitempty"`

	// UserAgent identifies the client making the request
	UserAgent string `json:"userAgent,omitempty"`

	// RequestID is a unique identifier for this request
	RequestID string `json:"requestId,omitempty"`
}

// NewAntigravityEnvelope creates a new envelope with default values
func NewAntigravityEnvelope(project, model string, request interface{}) AntigravityEnvelope {
	return AntigravityEnvelope{
		Project:     project,
		Model:       model,
		Request:     request,
		RequestType: "agent",
		UserAgent:   "antigravity",
		RequestID:   "agent-" + uuid.New().String(),
	}
}
