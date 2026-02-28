package runner

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/thread"
	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
)

type ReloadTool struct {
	client    graphql.Client
	agent     *agent.Agent
	threadMgr *thread.Manager
	threadID  string
	workspace string
	boot      *bootstrap.Result
	logger    interface{ Info(msg string, args ...any) }
}

type ReloadToolOptions struct {
	Client    graphql.Client
	Agent     *agent.Agent
	ThreadMgr *thread.Manager
	ThreadID  string
	Workspace string
	Boot      *bootstrap.Result
	Logger    interface{ Info(msg string, args ...any) }
}

func NewReloadTool(opts ReloadToolOptions) *ReloadTool {
	return &ReloadTool{
		client:    opts.Client,
		agent:     opts.Agent,
		threadMgr: opts.ThreadMgr,
		threadID:  opts.ThreadID,
		workspace: opts.Workspace,
		boot:      opts.Boot,
		logger:    opts.Logger,
	}
}

func (t *ReloadTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Reload",
		Description: "Reload bootstrap configuration and clear thread history. Use this when the user has modified prompts or configuration and wants to apply changes immediately without restarting the agent instance.",
		Parameters: jsonschema.Schema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
			Properties: map[string]*jsonschema.Schema{
				"clear_thread": {
					Type:        "boolean",
					Description: "Whether to clear the thread message history (default: true)",
				},
			},
		},
	}
}

type reloadInput struct {
	ClearThread bool `json:"clear_thread"`
}

func (t *ReloadTool) Execute(ctx context.Context, input reloadInput) agent.ToolResult {
	newBoot, err := bootstrap.Do(ctx, t.client, bootstrap.SystemPromptData{
		Workspace: t.workspace,
	})
	if err != nil {
		return agent.ToolResult{Error: fmt.Errorf("reload bootstrap failed: %w", err)}
	}

	t.boot.AgentID = newBoot.AgentID
	t.boot.AgentName = newBoot.AgentName
	t.boot.Model = newBoot.Model
	t.boot.SystemPrompt = newBoot.SystemPrompt
	t.boot.Tools = newBoot.Tools
	t.boot.Skills = newBoot.Skills
	t.boot.BuiltinTools = newBoot.BuiltinTools
	t.boot.AxonClawPath = newBoot.AxonClawPath

	localPrompt := buildLocalSystemPrompt(t.boot.AxonClawPath)

	t.agent.UpdateConfig(func(cfg agent.Config) agent.Config {
		cfg.SystemPrompts = []string{newBoot.SystemPrompt, localPrompt}
		return cfg
	})

	clearThread := input.ClearThread
	if !clearThread {
		clearThread = true
	}

	if clearThread {
		t.agent.ClearMessages()
		if t.threadMgr != nil && t.threadID != "" {
			if err := t.threadMgr.Delete(t.threadID); err != nil {
				if t.logger != nil {
					t.logger.Info("reload: failed to delete thread", "error", err)
				}
			}
		}
	}

	result := fmt.Sprintf("Reload completed successfully.\n- Agent: %s (%s)\n- Model: %s\n- Thread cleared: %v",
		t.boot.AgentName, t.boot.AgentID, t.boot.Model, clearThread)

	return agent.ToolResult{Content: agent.Content{Text: &result}}
}
