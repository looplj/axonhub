package objects

type PromptProtectionAction string

const (
	PromptProtectionActionMask   PromptProtectionAction = "mask"
	PromptProtectionActionReject PromptProtectionAction = "reject"
)

type PromptProtectionScope string

const (
	PromptProtectionScopeSystem    PromptProtectionScope = "system"
	PromptProtectionScopeDeveloper PromptProtectionScope = "developer"
	PromptProtectionScopeUser      PromptProtectionScope = "user"
	PromptProtectionScopeAssistant PromptProtectionScope = "assistant"
	PromptProtectionScopeTool      PromptProtectionScope = "tool"
)

type PromptProtectionSettings struct {
	Action      PromptProtectionAction  `json:"action"`
	Replacement string                  `json:"replacement,omitempty"`
	Scopes      []PromptProtectionScope `json:"scopes,omitempty"`
}
