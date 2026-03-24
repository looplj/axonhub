package subagent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"
)

const (
	defaultMaxIterations = 30
)

// Config configures a subagent run.
type Config struct {
	// Model is the LLM model identifier for the subagent.
	Model string
	// SystemPrompts are the system prompts for the subagent. At least one
	// non-empty prompt is required.
	SystemPrompts []string
	// AllowedTools is a whitelist of tool names the subagent may use.
	// If nil, all tools from the ToolSource are available (except SpawnAgent itself).
	AllowedTools []string
	// DeniedTools is a blacklist of tool names the subagent may NOT use.
	// This is applied after AllowedTools filtering.
	DeniedTools []string
	// MaxIterations limits the tool-call loop (0 means defaultMaxIterations).
	MaxIterations int
	// Provider is the LLM provider to use.
	Provider agent.Provider
	// ContextManager is an optional isolated context manager for the subagent.
	// If nil, a new SimpleContextManager is created automatically, ensuring
	// the subagent never shares context with the parent agent.
	ContextManager agent.ContextManager
	// Bus is an optional event bus shared with the parent agent. When provided,
	// the subagent publishes lifecycle events (e.g. EventMessageAdded) to this
	// bus so external consumers (archive writer, tracing) can observe them.
	// If nil, the subagent creates its own isolated bus and events are not
	// propagated to the parent.
	Bus bus.EventBus
	// Middlewares are optional tool-execution middlewares inherited from the
	// parent agent (e.g. permission evaluation). If omitted, the subagent will
	// run tools without any middleware enforcement.
	Middlewares []agent.Middleware
	// Logger is optional; if nil slog.Default() is used.
	Logger *slog.Logger
}

// ToolSource provides tools that can be registered on a subagent.
type ToolSource interface {
	// AvailableTools returns all tools that the subagent may use.
	// SpawnAgent is always excluded by the subagent runner (no nesting).
	AvailableTools() []agent.Tool
	// Middlewares returns middlewares that should be applied to the subagent tool
	// execution (e.g. permission/approval checks). Returning nil is allowed.
	Middlewares() []agent.Middleware
}

// Run creates a temporary agent with an isolated context window, registers
// the allowed tools, executes the given prompt, and returns the final
// assistant text. The subagent is ephemeral — it is discarded after Run returns.
func Run(ctx context.Context, cfg Config, prompt string, tools ToolSource) (*agent.Result, error) {
	if cfg.Provider == nil {
		return nil, fmt.Errorf("subagent: provider is required")
	}

	if cfg.Model == "" {
		return nil, fmt.Errorf("subagent: model is required")
	}

	if err := validateSystemPrompts(cfg.SystemPrompts); err != nil {
		return nil, err
	}

	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = defaultMaxIterations
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Create the ephemeral agent with an isolated context manager.
	// Each subagent MUST have its own context manager to prevent message
	// history from leaking between parent and child agents.
	cm := cfg.ContextManager
	if cm == nil {
		cm = agent.NewSimpleContextManager(nil)
	}

	opts := []agent.Option{
		agent.WithLogger(logger),
		agent.WithContextManager(cm),
	}

	if cfg.Bus != nil {
		opts = append(opts, agent.WithBus(cfg.Bus))
	}

	middlewares := cfg.Middlewares
	if middlewares == nil && tools != nil {
		middlewares = tools.Middlewares()
	}

	if len(middlewares) > 0 {
		opts = append(opts, agent.WithMiddlewares(middlewares...))
	}

	a := agent.New(agent.Config{
		Model:         cfg.Model,
		MaxIterations: maxIter,
		SystemPrompts: cfg.SystemPrompts,
	}, cfg.Provider, opts...)

	// Register tools, filtering by AllowedTools whitelist.
	if tools != nil {
		allowed := buildToolSet(cfg.AllowedTools)
		denied := buildToolSet(cfg.DeniedTools)

		for _, tool := range tools.AvailableTools() {
			name := tool.Definition().Name
			// Never register the SpawnAgent tool on a subagent (prevent nesting).
			if name == SpawnAgentToolName {
				continue
			}

			if allowed != nil {
				if _, ok := allowed[name]; !ok {
					continue
				}
			}

			if denied != nil {
				if _, ok := denied[name]; ok {
					continue
				}
			}

			a.RegisterTool(tool)
		}
	}

	// Execute the prompt synchronously.
	result, err := a.Process(ctx, agent.Content{Text: &prompt})
	if err != nil {
		return nil, fmt.Errorf("subagent: process failed: %w", err)
	}

	return result, nil
}

func validateSystemPrompts(prompts []string) error {
	if len(prompts) == 0 {
		return fmt.Errorf("subagent: at least one system prompt is required")
	}

	for _, prompt := range prompts {
		if strings.TrimSpace(prompt) != "" {
			return nil
		}
	}

	return fmt.Errorf("subagent: at least one non-empty system prompt is required")
}

// buildToolSet converts a slice of tool names to a set for O(1) lookup.
func buildToolSet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}

	return set
}
