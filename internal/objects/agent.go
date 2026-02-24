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

// AgentInstanceDeployment holds deployment-specific details for an agent instance.
type AgentInstanceDeployment struct {
	Directory           string `json:"directory,omitempty"`
	DockerContainerName string `json:"docker_container_name,omitempty"`
}

// DeployAxonclawInput is the input for deploying axonclaw to a runtime.
type DeployAxonclawInput struct {
	AgentID   GUID   `json:"agentID"`
	RuntimeID GUID   `json:"runtimeID"`
	Name      string `json:"name"`
	Directory string `json:"directory,omitempty"`
}
