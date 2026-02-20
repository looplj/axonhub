package objects

// AgentBuiltinTool defines a built-in tool configuration for an agent.
// It is stored as JSON in the agents.agent_builtin_tools field.
type AgentBuiltinTool struct {
	Name    string          `json:"name"`
	Enabled bool            `json:"enabled"`
	Order   int             `json:"order"`
	Config  *JSONRawMessage `json:"config,omitempty"`
}

// AgentSkillsPolicy defines the skills install/add policy for an agent.
// It is stored as JSON in the agents.skills_policy field.
type AgentSkillsPolicy struct {
	Add string `json:"add"`
}
