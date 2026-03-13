package runner

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/looplj/axonhub/axon/agent"

	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
	"github.com/looplj/axonhub/cmd/axonclaw/prompts"
)

type ResetTool struct {
	client    graphql.Client
	agent     *agent.Agent
	workspace string
	boot      *bootstrap.Result
	logger    interface{ Info(msg string, args ...any) }
}

type ResetToolOptions struct {
	Client    graphql.Client
	Agent     *agent.Agent
	Workspace string
	Boot      *bootstrap.Result
	Logger    interface{ Info(msg string, args ...any) }
}

func NewResetTool(opts ResetToolOptions) *ResetTool {
	return &ResetTool{
		client:    opts.Client,
		agent:     opts.Agent,
		workspace: opts.Workspace,
		boot:      opts.Boot,
		logger:    opts.Logger,
	}
}

func (t *ResetTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Reset",
		Description: "Refresh agent configuration (including system prompts, tools, and skills) from the server and reset the agent context. This clears in-memory messages without restarting the agent instance.",
		Parameters: jsonschema.Schema{
			Schema:               "https://json-schema.org/draft/2020-12/schema",
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			Properties:           map[string]*jsonschema.Schema{},
		},
	}
}

func (t *ResetTool) Execute(ctx context.Context, _ map[string]any) agent.ToolResult {
	t.agent.ClearMessages()

	newBoot, err := bootstrap.Do(ctx, t.client, bootstrap.Params{
		Workspace:  t.workspace,
		SkillsRoot: t.boot.SkillsRoot,
		ConfigDir:  t.boot.ConfigDir,
	})
	if err != nil {
		return agent.ToolResult{Error: fmt.Errorf("reset bootstrap failed: %w", err)}
	}

	threadID := t.boot.ThreadID
	*t.boot = *newBoot
	t.boot.ThreadID = threadID

	env := buildPromptEnv(newBoot, t.workspace)

	systemPrompts := prompts.BuildSystemPrompts(env, newBoot.Prompts, newBoot.Skills)

	t.agent.UpdateConfig(func(cfg agent.Config) agent.Config {
		cfg.Model = newBoot.Model
		cfg.SystemPrompts = systemPrompts
		return cfg
	})

	result := fmt.Sprintf("Reset completed successfully.\n- Agent: %s (%s)\n- Model: %s",
		t.boot.AgentName, t.boot.AgentID, t.boot.Model)

	return agent.ToolResult{Content: agent.Content{Text: &result}}
}
