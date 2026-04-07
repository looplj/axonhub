package subagent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"
	axoncontext "github.com/looplj/axonhub/axon/context"
	"github.com/looplj/axonhub/axon/tools"
)

const ForkAgentToolName = "ForkAgent"

type ForkTool struct {
	parentAgent *agent.Agent
	model       string
	bus         bus.EventBus
	logger      *slog.Logger
}

type ForkToolOptions struct {
	ParentAgent *agent.Agent
	Model       string
	Bus         bus.EventBus
	Logger      *slog.Logger
}

func NewForkTool(opts ForkToolOptions) *ForkTool {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &ForkTool{
		parentAgent: opts.ParentAgent,
		model:       opts.Model,
		bus:         opts.Bus,
		logger:      logger,
	}
}

type forkToolInput struct {
	TaskName string `json:"task_name"`
	Task     string `json:"task"`
}

var forkToolParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"task_name": {
			Type:        "string",
			Description: "A short, descriptive name for this forked task (e.g. 'refactor-auth', 'debug-api-error'). Used to label the fork's work.",
		},
		"task": {
			Type:        "string",
			Description: "The task description / prompt for the forked agent. The forked agent inherits the full conversation history, so you can reference prior context. Be specific about what you need it to do and what output you expect.",
		},
	},
	Required: []string{"task_name", "task"},
}

func (t *ForkTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name: ForkAgentToolName,
		Description: `Fork the current agent to handle a task with full conversation context.

Unlike SpawnAgent (which starts fresh), ForkAgent creates a copy of this agent with the entire conversation history. The forked agent can reference all prior context, decisions, and file changes discussed so far.

When to use:
- Tasks that need full conversation context to proceed correctly
- Offloading a sub-task that depends on prior discussion (e.g. "apply the same pattern we discussed to another file")
- Parallel work on related tasks that share the same context

When NOT to use:
- Independent tasks that don't need conversation history — use SpawnAgent instead
- Simple tool calls that you can do directly

Constraints:
- The forked agent CANNOT communicate with the user — only the parent can via SendMessage
- The forked agent CANNOT fork further (no nesting)
- Changes to messages in either the parent or the fork do not affect each other after the fork point`,
		Parameters: forkToolParameters,
	}
}

func (t *ForkTool) Execute(ctx context.Context, input forkToolInput) agent.ToolResult {
	ctx = axoncontext.WithSource(ctx, input.TaskName)

	t.logger.Info("fork agent starting",
		"task_name", input.TaskName,
		"task_len", len(input.Task),
	)

	var forkOpts []agent.Option

	if t.bus != nil {
		forkOpts = append(forkOpts, agent.WithBus(t.bus))
	}

	forkOpts = append(forkOpts, agent.WithLogger(t.logger.With("component", "fork_agent")))

	forked := t.parentAgent.Fork(agent.ForkConfig{
		Model: t.model,
		ExcludeTools: map[string]struct{}{
			ForkAgentToolName:  {},
			SpawnAgentToolName: {},
		},
	}, forkOpts...)

	result, err := forked.Process(ctx, agent.Content{Text: &input.Task})
	if err != nil {
		return tools.ErrorResult(fmt.Errorf("forked agent failed: %w", err))
	}

	t.logger.Info("fork agent completed",
		"task_name", input.TaskName,
		"output_len", len(result.Output),
		"input_tokens", result.Usage.InputTokens,
		"output_tokens", result.Usage.OutputTokens,
	)

	if result.Output == "" {
		return tools.TextResult("Forked agent completed but produced no output.")
	}

	return tools.TextResult(result.Output)
}
