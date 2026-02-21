package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/looplj/axonhub/axon/agent"
)

type ToolWrapped[I any] interface {
	Definition() agent.ToolDefinition
	Execute(ctx context.Context, input I) agent.ToolResult
}

type AgentTool[I any] struct {
	tool     ToolWrapped[I]
	schema   jsonschema.Schema
	resolved *jsonschema.Resolved
}

func NewAgentTool[I any](tool ToolWrapped[I]) *AgentTool[I] {
	schema := tool.Definition().Parameters

	resolved, err := schema.Resolve(nil)
	if err != nil {
		panic(err)
	}

	return &AgentTool[I]{
		tool:     tool,
		schema:   schema,
		resolved: resolved,
	}
}

func (w *AgentTool[I]) Definition() agent.ToolDefinition {
	return w.tool.Definition()
}

func (w *AgentTool[I]) Execute(ctx context.Context, arguments json.RawMessage) agent.ToolResult {
	var obj map[string]any
	if err := json.Unmarshal(arguments, &obj); err != nil {
		return ErrorResult(fmt.Errorf("%w: invalid arguments", err))
	}

	if obj == nil {
		obj = map[string]any{}
	}

	if err := w.resolved.Validate(obj); err != nil {
		return ErrorResult(fmt.Errorf("%w: caused by %w", ErrInvalidArguments, err))
	}

	var input I
	if err := json.Unmarshal(arguments, &input); err != nil {
		return ErrorResult(fmt.Errorf("%w: caused by %w", ErrInvalidArguments, err))
	}

	return w.tool.Execute(ctx, input)
}
