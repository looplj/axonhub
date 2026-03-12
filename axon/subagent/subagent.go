package subagent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/looplj/axonhub/axon/agent"
)

const (
	defaultMaxIterations = 30
	// SpawnAgentToolName is the tool name that must be excluded from subagent
	// tool registrations to prevent infinite nesting.
	SpawnAgentToolName = "SpawnAgent"
)

// Config configures a subagent run.
type Config struct {
	// Model is the LLM model identifier for the subagent.
	Model string
	// SystemPrompt is the system prompt for the subagent. Required.
	SystemPrompt string
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

	if cfg.SystemPrompt == "" {
		return nil, fmt.Errorf("subagent: system prompt is required")
	}

	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = defaultMaxIterations
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Create the ephemeral agent.
	opts := []agent.Option{agent.WithLogger(logger)}

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
		SystemPrompts: []string{cfg.SystemPrompt},
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

// buildToolSet converts a slice of tool names to a set for O(1) lookup.
// Returns nil if the input is nil (meaning no filtering is applied).
func buildToolSet(names []string) map[string]struct{} {
	if names == nil {
		return nil
	}

	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}

	return set
}
