package claw

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"
	"github.com/looplj/axonhub/axon/subagent"
	"github.com/looplj/axonhub/axon/tools"
)

const (
	// SummarizerAgentName is the bundled subagent name used by the summarizer.
	// Users can override it by placing a "summarizer.md" file in the subagents
	// directory.
	SummarizerAgentName = "axonclaw_summarizer"

	summarizerSystemPrompt = `You are a context summarization assistant. Your task is to analyze the conversation history and create a concise summary that preserves critical information for future context.

## Instructions

1. Analyze the conversation and identify:
   - Key decisions made and their rationale
   - File changes (created, modified, deleted) with brief descriptions
   - User preferences and coding style discovered
   - Important entities (people, projects, concepts, tools)
   - Task progress and current status

2. Use the Skill tool to load the "memory-management" skill, then follow its instructions to persist important long-term information:
   - Store user preferences, key decisions, and important entities that should be remembered across sessions
   - Organize memories by category (e.g. "longterm/preferences", "longterm/decisions", "daily/YYYY-MM-DD")

3. Return a concise plain-text summary (NOT JSON) that covers:
   - What was discussed and accomplished
   - Key decisions and their reasons
   - File changes made
   - Current task status and any pending items

The summary will be injected into the conversation context to help the assistant maintain continuity. Keep it focused and concise — omit routine tool interactions and focus on what matters for continuing the work.`
)

// SummarizerDefinition returns the bundled subagent Definition for the
// summarizer. Register it on a Manager via RegisterBundled before Load so
// that users can override the prompt / tools / model via a .md file.
func SummarizerDefinition() *subagent.Definition {
	return &subagent.Definition{
		Name:        SummarizerAgentName,
		Hidden:      true,
		Description: summarizerSystemPrompt,
		Tools: map[string]bool{
			"*":     true,
			"Skill": true,
			"Bash":  true,
		},
	}
}

type SmartSummarizer struct {
	manager     *subagent.Manager
	provider    agent.Provider
	model       string
	skillMgr    *tools.SkillManager
	workspace   string
	bus         bus.EventBus
	middlewares []agent.Middleware
	logger      *slog.Logger
}

type SmartSummarizerOptions struct {
	Manager     *subagent.Manager
	Provider    agent.Provider
	Model       string
	SkillMgr    *tools.SkillManager
	Workspace   string
	Bus         bus.EventBus
	Middlewares []agent.Middleware
	Logger      *slog.Logger
}

func NewSmartSummarizer(opts SmartSummarizerOptions) *SmartSummarizer {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &SmartSummarizer{
		manager:     opts.Manager,
		provider:    opts.Provider,
		model:       opts.Model,
		skillMgr:    opts.SkillMgr,
		workspace:   opts.Workspace,
		bus:         opts.Bus,
		middlewares: opts.Middlewares,
		logger:      logger,
	}
}

func (s *SmartSummarizer) Summarize(ctx context.Context, messages []agent.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	conversationText := RenderMessages(messages, MessageRenderOptions{Separator: "\n\n"})

	def, ok := s.manager.Get(SummarizerAgentName)
	if !ok {
		return "", fmt.Errorf("summarizer subagent definition %q not found", SummarizerAgentName)
	}

	model := def.Model
	if model == "" {
		model = s.model
	}

	allowedTools, deniedTools := subagent.BuildToolFiltersFromDefinition(def.Tools)

	task := fmt.Sprintf("Please summarize the following conversation:\n\n%s", conversationText)

	toolSource := s.buildToolSource()

	result, err := subagent.Run(ctx, subagent.Config{
		Model:         model,
		SystemPrompts: []string{def.Description},
		AllowedTools:  allowedTools,
		DeniedTools:   deniedTools,
		Provider:      s.provider,
		Bus:           s.bus,
		Middlewares:   s.middlewares,
		Logger:        s.logger.With("component", "summarizer_subagent"),
	}, task, toolSource)
	if err != nil {
		return "", fmt.Errorf("summarizer subagent failed: %w", err)
	}

	if result.Output == "" {
		return "", fmt.Errorf("summarizer subagent returned empty output")
	}

	return result.Output, nil
}

func (s *SmartSummarizer) buildToolSource() subagent.ToolSource {
	agentTools := []agent.Tool{}

	agentTools = append(agentTools, tools.NewAgentTool(tools.NewReadTool(s.workspace, true)))
	agentTools = append(agentTools, tools.NewAgentTool(tools.NewWriteTool(s.workspace, true)))
	agentTools = append(agentTools, tools.NewAgentTool(tools.NewBashTool(s.workspace, true, true)))
	agentTools = append(agentTools, tools.NewAgentTool(tools.NewGrepTool(s.workspace, true)))
	agentTools = append(agentTools, tools.NewAgentTool(tools.NewGlobTool(s.workspace, true)))

	if s.skillMgr != nil {
		agentTools = append(agentTools, tools.NewAgentTool(tools.NewSkillTool(s.skillMgr)))
	}

	return &staticToolSource{tools: agentTools}
}

type staticToolSource struct {
	tools []agent.Tool
}

func (s *staticToolSource) AvailableTools() []agent.Tool    { return s.tools }
func (s *staticToolSource) Middlewares() []agent.Middleware { return nil }
