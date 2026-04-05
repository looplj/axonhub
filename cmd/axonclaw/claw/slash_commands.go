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
		Description: "Reset agent state: /reset [all|prompts|tasks|soul|identity|user|system|memory|heartbeat]",
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

func executeReset(ctx context.Context, r *Runner, args []string) (string, error) {
	r.stopProcessing()
	r.Agent.ClearMessages()

	resetType := "all"
	if len(args) > 0 {
		resetType = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch resetType {
	case "all":
		return executeResetAll(ctx, r)
	case "prompts":
		return executeResetPrompts(ctx, r, prompts.ResetOptions{
			Soul:     true,
			Identity: true,
			User:     true,
			System:   true,
			// Memory need to be reset separately.
			Memory: false,
			// Heartbeat need to be reset separately.¬
			Heartbeat: false,
		})
	case "tasks":
		return executeResetTasks(ctx, r)
	case "soul":
		return executeResetPrompts(ctx, r, prompts.ResetOptions{Soul: true})
	case "identity":
		return executeResetPrompts(ctx, r, prompts.ResetOptions{Identity: true})
	case "user":
		return executeResetPrompts(ctx, r, prompts.ResetOptions{User: true})
	case "system", "agents":
		return executeResetPrompts(ctx, r, prompts.ResetOptions{System: true})
	case "memory":
		return executeResetPrompts(ctx, r, prompts.ResetOptions{Memory: true})
	case "heartbeat":
		return executeResetPrompts(ctx, r, prompts.ResetOptions{Heartbeat: true})
	default:
		return "", fmt.Errorf("unknown reset target: %q. Available: all, prompts, tasks, soul, identity, user, system, memory, heartbeat", resetType)
	}
}

func executeResetAll(ctx context.Context, r *Runner) (string, error) {
	newBoot, err := bootstrap.Do(ctx, r.Client, bootstrap.Params{
		Workspace:  r.Workspace,
		SkillsRoot: r.Boot.SkillsRoot,
		PromptDir:  r.Boot.PromptDir,
		RuntimeDir: r.Boot.RuntimeDir,
	})
	if err != nil {
		return "", fmt.Errorf("reset bootstrap failed: %w", err)
	}

	env := buildPromptEnv(newBoot, r.Workspace)

	if err := prompts.ResetToDefaults(newBoot.PromptDir, prompts.ResetOptions{
		Soul:      true,
		Identity:  true,
		User:      true,
		System:    true,
		Memory:    true,
		Heartbeat: true,
	}, env, newBoot.ServerSystemPrompt); err != nil {
		return "", fmt.Errorf("reset prompt files: %w", err)
	}

	if r.TaskStore != nil {
		if err := r.TaskStore.Reset(); err != nil {
			return "", fmt.Errorf("reset tasks: %w", err)
		}

		if err := EnsureDefaultTasks(r.TaskStore); err != nil {
			return "", fmt.Errorf("reinit default tasks: %w", err)
		}
	}

	threadID := r.Boot.ThreadID
	*r.Boot = *newBoot
	r.Boot.ThreadID = threadID

	systemPrompts := prompts.BuildSystemPrompts(env, newBoot.Prompts)

	r.Agent.UpdateConfig(func(cfg agent.Config) agent.Config {
		cfg.Model = newBoot.Model
		cfg.SystemPrompts = systemPrompts

		return cfg
	})

	return fmt.Sprintf("Reset completed successfully.\n- Agent: %s (%s)\n- Model: %s\n- Prompts: reset to defaults\n- Tasks: reset to defaults",
		r.Boot.AgentName, r.Boot.AgentID, r.Boot.Model), nil
}

func executeResetPrompts(ctx context.Context, r *Runner, opts prompts.ResetOptions) (string, error) {
	newBoot, err := bootstrap.Do(ctx, r.Client, bootstrap.Params{
		Workspace:  r.Workspace,
		SkillsRoot: r.Boot.SkillsRoot,
		PromptDir:  r.Boot.PromptDir,
		RuntimeDir: r.Boot.RuntimeDir,
	})
	if err != nil {
		return "", fmt.Errorf("reset bootstrap failed: %w", err)
	}

	env := buildPromptEnv(newBoot, r.Workspace)

	if err := prompts.ResetToDefaults(newBoot.PromptDir, opts, env, newBoot.ServerSystemPrompt); err != nil {
		return "", fmt.Errorf("reset prompt files: %w", err)
	}

	threadID := r.Boot.ThreadID
	*r.Boot = *newBoot
	r.Boot.ThreadID = threadID

	systemPrompts := prompts.BuildSystemPrompts(env, newBoot.Prompts)

	r.Agent.UpdateConfig(func(cfg agent.Config) agent.Config {
		cfg.Model = newBoot.Model
		cfg.SystemPrompts = systemPrompts

		return cfg
	})

	resetItems := []string{}
	if opts.Soul {
		resetItems = append(resetItems, "soul")
	}

	if opts.Identity {
		resetItems = append(resetItems, "identity")
	}

	if opts.User {
		resetItems = append(resetItems, "user")
	}

	if opts.System {
		resetItems = append(resetItems, "system")
	}

	if opts.Memory {
		resetItems = append(resetItems, "memory")
	}

	if opts.Heartbeat {
		resetItems = append(resetItems, "heartbeat")
	}

	return fmt.Sprintf("Reset completed successfully.\n- Agent: %s (%s)\n- Model: %s\n- Prompts reset: %s",
		r.Boot.AgentName, r.Boot.AgentID, r.Boot.Model, strings.Join(resetItems, ", ")), nil
}

func executeResetTasks(ctx context.Context, r *Runner) (string, error) {
	if r.TaskStore == nil {
		return "", fmt.Errorf("task store not available")
	}

	if err := r.TaskStore.Reset(); err != nil {
		return "", fmt.Errorf("reset tasks: %w", err)
	}

	if err := EnsureDefaultTasks(r.TaskStore); err != nil {
		return "", fmt.Errorf("reinit default tasks: %w", err)
	}

	return "Tasks reset completed successfully.", nil
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
