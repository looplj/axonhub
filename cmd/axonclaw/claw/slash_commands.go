package claw

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/subagent"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
	"github.com/looplj/axonhub/cmd/axonclaw/prompts"
)

type SlashCommand struct {
	Name        string
	Description string
	Execute     func(ctx context.Context, r *Runner, args []string) (string, error)
}

type SlashCommandRegistry struct {
	commands map[string]*SlashCommand
}

func NewSlashCommandRegistry() *SlashCommandRegistry {
	return &SlashCommandRegistry{
		commands: make(map[string]*SlashCommand),
	}
}

func (r *SlashCommandRegistry) Register(cmd *SlashCommand) {
	r.commands[cmd.Name] = cmd
}

func (r *SlashCommandRegistry) Get(name string) (*SlashCommand, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

func (r *SlashCommandRegistry) List() []*SlashCommand {
	result := make([]*SlashCommand, 0, len(r.commands))
	for _, cmd := range r.commands {
		result = append(result, cmd)
	}

	return result
}

func (r *SlashCommandRegistry) Match(input string) (*SlashCommand, []string, bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return nil, nil, false
	}

	fields := strings.Fields(input)
	if len(fields) == 0 {
		return nil, nil, false
	}

	cmdName := fields[0]
	args := fields[1:]

	cmd, ok := r.commands[cmdName]
	if !ok {
		return nil, nil, false
	}

	return cmd, args, true
}

func NewDefaultSlashCommands(client graphql.Client) *SlashCommandRegistry {
	registry := NewSlashCommandRegistry()

	registry.Register(&SlashCommand{
		Name:        "/reset",
		Description: "Refresh agent configuration and reset context",
		Execute:     executeReset,
	})

	registry.Register(&SlashCommand{
		Name:        "/help",
		Description: "Show available slash commands",
		Execute:     executeHelp,
	})

	registry.Register(&SlashCommand{
		Name:        "/clear",
		Description: "Clear agent conversation history",
		Execute:     executeClear,
	})

	registry.Register(&SlashCommand{
		Name:        "/subagent",
		Description: "Spawn a subagent: /subagent <agent_type> <task>",
		Execute:     executeSubagent,
	})

	registry.Register(&SlashCommand{
		Name:        "/stop",
		Description: "Stop the agent's current processing immediately",
		Execute:     executeStop,
	})

	return registry
}

func executeReset(ctx context.Context, r *Runner, _ []string) (string, error) {
	r.stopProcessing()
	r.Agent.ClearMessages()

	newBoot, err := bootstrap.Do(ctx, r.Client, bootstrap.Params{
		Workspace:  r.Workspace,
		SkillsRoot: r.Boot.SkillsRoot,
		ConfigDir:  r.Boot.ConfigDir,
	})
	if err != nil {
		return "", fmt.Errorf("reset bootstrap failed: %w", err)
	}

	threadID := r.Boot.ThreadID
	*r.Boot = *newBoot
	r.Boot.ThreadID = threadID

	env := buildPromptEnv(newBoot, r.Workspace)

	systemPrompts := prompts.BuildSystemPrompts(env, newBoot.Prompts)

	r.Agent.UpdateConfig(func(cfg agent.Config) agent.Config {
		cfg.Model = newBoot.Model
		cfg.SystemPrompts = systemPrompts

		return cfg
	})

	return fmt.Sprintf("Reset completed successfully.\n- Agent: %s (%s)\n- Model: %s",
		r.Boot.AgentName, r.Boot.AgentID, r.Boot.Model), nil
}

func executeHelp(_ context.Context, r *Runner, _ []string) (string, error) {
	var sb strings.Builder
	sb.WriteString("Available slash commands:\n")

	for _, cmd := range r.slashCommands.List() {
		fmt.Fprintf(&sb, "  %-12s %s\n", cmd.Name, cmd.Description)
	}

	sb.WriteString("\nUsage: Type /command in your message to execute a slash command.")

	return sb.String(), nil
}

func executeClear(_ context.Context, r *Runner, _ []string) (string, error) {
	r.stopProcessing()
	r.Agent.ClearMessages()
	return "Conversation history cleared.", nil
}

func executeStop(_ context.Context, r *Runner, _ []string) (string, error) {
	if !r.stopProcessing() {
		return "Agent is not currently processing.", nil
	}

	return "Agent stopped.", nil
}

func executeSubagent(ctx context.Context, r *Runner, args []string) (string, error) {
	if r.subagentMgr == nil {
		return "", fmt.Errorf("subagent manager not configured")
	}

	if len(args) < 2 {
		visibleAgents := r.subagentMgr.ListVisible()
		if len(visibleAgents) == 0 {
			return "No subagents available.", nil
		}

		var sb strings.Builder
		sb.WriteString("Available subagents:\n")

		for _, a := range visibleAgents {
			fmt.Fprintf(&sb, "  - %s\n", a.Name)
		}

		sb.WriteString("\nUsage: /subagent <agent_type> <task>")

		return sb.String(), nil
	}

	agentType := args[0]
	task := strings.Join(args[1:], " ")

	def, ok := r.subagentMgr.Get(agentType)
	if !ok {
		visibleAgents := r.subagentMgr.ListVisible()

		availableTypes := make([]string, 0, len(visibleAgents))
		for _, a := range visibleAgents {
			availableTypes = append(availableTypes, a.Name)
		}

		return "", fmt.Errorf("agent type %q not found. Available types: %v", agentType, availableTypes)
	}

	model := def.Model
	if model == "" {
		model = r.Boot.Model
	}

	allowedTools, deniedTools := buildSubAgentTools(def.Tools)

	r.Logger.Info("slash command: spawning subagent",
		"agent_type", agentType,
		"model", model,
		"task_len", len(task),
	)

	go func() {
		result, err := subagent.Run(ctx, subagent.Config{
			Model:         model,
			SystemPrompts: []string{def.Description},
			AllowedTools:  allowedTools,
			DeniedTools:   deniedTools,
			Provider:      r.Provider,
			Middlewares:   r.Agent.Middlewares(),
			Logger:        r.Logger.With("component", "slash_subagent"),
		}, task, r.toolSource)
		if err != nil {
			r.Logger.Warn("subagent failed", "agent_type", agentType, "error", err)
			return
		}

		if result.Output != "" {
			r.Agent.FollowUp(agent.Message{
				Role:    agent.RoleUser,
				Content: &agent.Content{Text: &result.Output},
			})
		}
	}()

	return fmt.Sprintf("Subagent %q spawned in background.", agentType), nil
}

func buildSubAgentTools(tools map[string]bool) (allowed []string, denied []string) {
	if tools == nil {
		return nil, nil
	}

	if len(tools) == 0 {
		return []string{}, nil
	}

	if defaultAllow, ok := tools["*"]; ok {
		if defaultAllow {
			for toolName, enabled := range tools {
				if toolName == "*" {
					continue
				}

				if !enabled {
					denied = append(denied, toolName)
				}
			}

			return nil, denied
		}

		for toolName, enabled := range tools {
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

	for toolName, enabled := range tools {
		if enabled {
			allowed = append(allowed, toolName)
		}
	}

	if allowed == nil {
		allowed = []string{}
	}

	return allowed, nil
}

func (r *Runner) sendSlashCommandResult(ctx context.Context, text string, msgID string) {
	var replyToID *string
	if msgID != "" {
		replyToID = &msgID
	}

	_, err := api.ReplyMessage(ctx, r.Client, &api.ReplyMessageInput{
		Text:             text,
		ReplyToMessageID: replyToID,
		Type:             lo.ToPtr(api.AgentMessageTypeChat),
	})
	if err != nil {
		r.Logger.Warn("failed to send slash command result", "error", err)
	}
}
