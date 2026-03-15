package subagent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/tools"
)

const (
	// SpawnAgentToolName is the tool name that must be excluded from subagent
	// tool registrations to prevent infinite nesting.
	SpawnAgentToolName = "SpawnAgent"
)

type Tool struct {
	manager     *Manager
	provider    agent.Provider
	toolSource  ToolSource
	model       string
	middlewares []agent.Middleware
	logger      *slog.Logger
}

type ToolOptions struct {
	Manager     *Manager
	Provider    agent.Provider
	ToolSource  ToolSource
	Model       string
	Middlewares []agent.Middleware
	Logger      *slog.Logger
}

func NewTool(opts ToolOptions) *Tool {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Tool{
		manager:     opts.Manager,
		provider:    opts.Provider,
		toolSource:  opts.ToolSource,
		model:       opts.Model,
		middlewares: opts.Middlewares,
		logger:      logger,
	}
}

type toolInput struct {
	AgentType string `json:"agent_type"`
	Task      string `json:"task"`
}

var toolParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"agent_type": {
			Type:        "string",
			Description: "The type of agent to spawn. Each type has its own system prompt and tool configuration defined by markdown files in `.agent/subagents/`.",
		},
		"task": {
			Type:        "string",
			Description: "The task description / prompt for the spawned agent. Be specific about what you need it to do and what output you expect.",
		},
	},
	Required: []string{"agent_type", "task"},
}

func (t *Tool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        SpawnAgentToolName,
		Description: "Spawn an agent with its own isolated context window to handle a specific task. The spawned agent works independently, uses tools as needed, and returns its final output.\n\nWhen to use:\n- Large exploration: scanning many files, searching across a codebase — keeps intermediate output out of your context\n- Independent sub-tasks: tasks that can be fully completed in isolation (e.g. write a test, review a file)\n- Cost optimization: use a cheaper/faster model for simple tasks (exploration, search, formatting)\n\nParameters:\n- agent_type (required): the type of agent to spawn, defined by markdown files under `.agent/subagents/`\n- task (required): clear, self-contained description of what to do and what output to return\n\nConstraints:\n- Spawned agents CANNOT communicate with the user — only you can via SendMessage\n- Spawned agents CANNOT spawn further agents (no nesting)\n- Each spawned agent has its own isolated context window — it does NOT share your conversation history",
		Parameters:  toolParameters,
	}
}

func (t *Tool) Execute(ctx context.Context, input toolInput) agent.ToolResult {
	if t.manager == nil {
		return tools.ErrorResult(fmt.Errorf("subagent manager not configured"))
	}

	def, ok := t.manager.Get(input.AgentType)
	if !ok {
		visibleAgents := t.manager.ListVisible()

		availableTypes := make([]string, 0, len(visibleAgents))
		for _, a := range visibleAgents {
			availableTypes = append(availableTypes, a.Name)
		}

		return tools.ErrorResult(fmt.Errorf("agent type %q not found. Available types: %v", input.AgentType, availableTypes))
	}

	model := def.Model
	if model == "" {
		model = t.model
	}

	allowedTools, deniedTools := buildToolFiltersFromDefinition(def.Tools)

	t.logger.Info("spawn agent starting",
		"agent_type", input.AgentType,
		"model", model,
		"task_len", len(input.Task),
		"allowed_tools", allowedTools,
		"denied_tools", deniedTools,
	)

	result, err := Run(ctx, Config{
		Model:         model,
		SystemPrompts: []string{def.Description},
		AllowedTools:  allowedTools,
		DeniedTools:   deniedTools,
		Provider:      t.provider,
		Middlewares:   t.middlewares,
		Logger:        t.logger.With("component", "spawn_agent"),
	}, input.Task, t.toolSource)
	if err != nil {
		return tools.ErrorResult(fmt.Errorf("spawned agent failed: %w", err))
	}

	t.logger.Info("spawn agent completed",
		"agent_type", input.AgentType,
		"model", model,
		"output_len", len(result.Output),
		"input_tokens", result.Usage.InputTokens,
		"output_tokens", result.Usage.OutputTokens,
	)

	if result.Output == "" {
		return tools.TextResult("Spawned agent completed but produced no output.")
	}

	return tools.TextResult(result.Output)
}

func buildToolFiltersFromDefinition(toolsConfig map[string]bool) (allowed []string, denied []string) {
	// Nil toolsConfig means "no configuration provided" => no filtering
	// (allow all tools, except SpawnAgent which is always excluded).
	if toolsConfig == nil {
		return nil, nil
	}

	// Non-nil but empty toolsConfig means: explicitly configured but nothing
	// enabled => allow none.
	if len(toolsConfig) == 0 {
		return []string{}, nil
	}

	// If the config explicitly contains "*", treat it as a default policy:
	// - "*": true  => allow all tools by default; "tool": false entries deny.
	// - "*": false => deny all tools by default; "tool": true entries allow.
	if defaultAllow, ok := toolsConfig["*"]; ok {
		if defaultAllow {
			for toolName, enabled := range toolsConfig {
				if toolName == "*" {
					continue
				}

				if !enabled {
					denied = append(denied, toolName)
				}
			}

			return nil, denied
		}

		for toolName, enabled := range toolsConfig {
			if toolName == "*" {
				continue
			}

			if enabled {
				allowed = append(allowed, toolName)
			}
		}

		if allowed == nil {
			allowed = []string{}
		}

		return allowed, nil
	}

	// No default marker: treat it as an allowlist.
	for toolName, enabled := range toolsConfig {
		if enabled {
			allowed = append(allowed, toolName)
		}
	}

	// Important: return a non-nil empty slice if nothing is enabled, so the
	// caller can distinguish it from nil ("allow all").
	if allowed == nil {
		allowed = []string{}
	}

	return allowed, nil
}
