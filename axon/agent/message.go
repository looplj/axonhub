package agent

// Role represents a message sender role.
type Role string

const (
	// RoleSystem represents a system-level message.
	RoleSystem Role = "system"
	// RoleUser represents a user message.
	RoleUser Role = "user"
	// RoleAssistant represents an assistant message.
	RoleAssistant Role = "assistant"
	// RoleTool represents a tool result message.
	RoleTool Role = "tool"
)

// Message represents a conversation message.
type Message struct {
	Role Role `json:"role"`

	// For normal message.
	Content *Content `json:"content,omitempty"`

	// For tool result
	ToolUseID *string `json:"tool_call_id,omitempty"`
	IsError   *bool   `json:"is_error,omitempty"`

	// For tool use message
	// One tool use per message, it is helpful to handle parallel tool use.
	ToolCall *ToolCall `json:"tool_call,omitempty"`

	// RoundIndex groups messages from the same LLM call round.
	// Messages with the same RoundIndex should be aggregated into a single API message.
	RoundIndex int `json:"round_index"`
}

// ToolCall represents an AI tool invocation.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"arguments"` // JSON arguments (valid JSON only when complete)
}
