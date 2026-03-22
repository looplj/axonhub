package claw

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/subagent"

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
		Name:        "/skill",
		Description: "Execute a skill: /skill <skill_name> [args]",
		Execute:     executeSkill,
	})

	return registry
}

func executeReset(ctx context.Context, r *Runner, _ []string) (string, error) {
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
	r.Agent.ClearMessages()
	return "Conversation history cleared.", nil
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
		return "", fmt.Errorf("subagent failed: %w", err)
	}

	if result.Output == "" {
		return "Subagent completed but produced no output.", nil
	}

	return result.Output, nil
}

func executeSkill(_ context.Context, r *Runner, args []string) (string, error) {
	if r.skillMgr == nil {
		return "", fmt.Errorf("skill manager not configured")
	}

	if len(args) == 0 {
		skills, err := r.skillMgr.List()
		if err != nil {
			return "", fmt.Errorf("failed to list skills: %w", err)
		}

		if len(skills) == 0 {
			return "No skills available.", nil
		}

		var sb strings.Builder
		sb.WriteString("Available skills:\n")

		for _, s := range skills {
			fmt.Fprintf(&sb, "  - %s\n", s.Name)
		}

		sb.WriteString("\nUsage: /skill <skill_name> [args]")

		return sb.String(), nil
	}

	skillName := args[0]

	result, err := r.skillMgr.Get(skillName)
	if err != nil {
		return "", fmt.Errorf("skill %q not found: %w", skillName, err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Skill: %s\n", result.Skill.Name)
	fmt.Fprintf(&sb, "Directory: %s\n", result.Skill.Dir)
	sb.WriteString("\nContent:\n")
	sb.WriteString(result.Skill.Content)

	if len(args) > 1 {
		fmt.Fprintf(&sb, "\n\nArguments: %s", strings.Join(args[1:], " "))
	}

	return sb.String(), nil
}

func buildSubAgentTools(tools map[string]bool) (allowed []string, denied []string) {
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

func (r *Runner) sendSlashCommandResult(text string) {
	r.Agent.FollowUp(agent.Message{
		Role:    agent.RoleAssistant,
		Content: &agent.Content{Text: &text},
	})
}
