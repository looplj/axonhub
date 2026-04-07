package claw

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"
	axoncontext "github.com/looplj/axonhub/axon/context"
)

const compactInstruction = `

CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.
Tool calls will be REJECTED and will waste your only turn — you will fail the task.

---

## Context Compaction Task

The conversation history above has grown too large and needs to be compacted.

Your task: Create a concise summary that preserves critical information for continuing the work.

### Analysis Phase
First, analyze the conversation and identify:
- Key decisions made and their rationale
- File changes (created, modified, deleted) with brief descriptions
- User preferences and coding style discovered
- Important entities (people, projects, concepts, tools)
- Task progress and current status
- Errors encountered and how they were resolved

### Summary Output
Return a concise plain-text summary (NOT JSON) that covers:

1. **Primary Request**: What the user originally asked for
2. **Key Decisions**: Important choices made and why
3. **Files Changed**: List of files created/modified/deleted with brief descriptions
4. **Current State**: What is actively being worked on right now
5. **Pending Items**: Any unfinished tasks or next steps
6. **Important Context**: User preferences, constraints, or other critical info

The summary will be injected into the conversation context to help continue the work.
Keep it focused and concise — omit routine tool interactions and focus on what matters for continuing the work.

Do NOT call any tools. Respond with TEXT ONLY.`

type Summarizer interface {
	Summarize(ctx context.Context, messages []agent.Message) (string, error)
}

type ForkedCompactSummarizer struct {
	agent       *agent.Agent
	provider    agent.Provider
	model       string
	logger      *slog.Logger
	bus         bus.EventBus
	middlewares []agent.Middleware
}

type ForkedCompactSummarizerOptions struct {
	Agent       *agent.Agent
	Provider    agent.Provider
	Model       string
	Logger      *slog.Logger
	Bus         bus.EventBus
	Middlewares []agent.Middleware
}

func NewForkedCompactSummarizer(opts ForkedCompactSummarizerOptions) *ForkedCompactSummarizer {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &ForkedCompactSummarizer{
		agent:       opts.Agent,
		provider:    opts.Provider,
		model:       opts.Model,
		logger:      logger,
		bus:         opts.Bus,
		middlewares: opts.Middlewares,
	}
}

func (s *ForkedCompactSummarizer) Summarize(ctx context.Context, messages []agent.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	cfg := s.agent.Config()

	model := cfg.Model
	if s.model != "" {
		model = s.model
	}

	forkCfg := agent.Config{
		Model:         model,
		MaxIterations: 1,
		SystemPrompts: cfg.SystemPrompts,
	}

	forkOpts := []agent.Option{
		agent.WithLogger(s.logger.With("component", "compact_fork")),
		agent.WithMessages(messages),
	}

	if s.bus != nil {
		forkOpts = append(forkOpts, agent.WithBus(s.bus))
	}

	if len(s.middlewares) > 0 {
		forkOpts = append(forkOpts, agent.WithMiddlewares(s.middlewares...))
	}

	ctx = axoncontext.WithSource(ctx, "compaction")

	forkAgent := agent.New(forkCfg, s.provider, forkOpts...)

	compactPrompt := compactInstruction

	result, err := forkAgent.Process(ctx, agent.Content{Text: &compactPrompt})
	if err != nil {
		return "", fmt.Errorf("compact fork failed: %w", err)
	}

	if result.Output == "" {
		return "", fmt.Errorf("compact fork returned empty output")
	}

	return strings.TrimSpace(result.Output), nil
}
