package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
)

type toolCallBuilder struct {
	id        string
	name      string
	jsonParts []string
}

func (b *toolCallBuilder) buildJSON() string {
	var sb strings.Builder
	for _, part := range b.jsonParts {
		sb.WriteString(part)
	}
	return sb.String()
}

type Middleware interface {
	BeforeTool(ctx context.Context, req ToolRequest) error
	AfterTool(ctx context.Context, req ToolRequest, toolErr error) error
}

type ToolRequest struct {
	ThreadID   string
	Workspace  string
	ToolCallID string
	ToolName   string
	ToolInput  string
	StartedAt  time.Time
}

// Tool is the interface that tools must implement.
type Tool interface {
	// Definition returns the tool's schema definition.
	Definition() ToolDefinition

	// Execute runs the tool with the given JSON arguments.
	Execute(ctx context.Context, arguments json.RawMessage) ToolResult
}

// ToolDefinition describes a tool available to the agent.
type ToolDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  jsonschema.Schema `json:"parameters"`
}

// ToolResult represents the result of a tool execution.
// If Error is set, all other fields are ignored.
type ToolResult struct {
	Content Content
	Error   error
}

type toolResultJSON struct {
	Content Content `json:"content"`
	Error   string  `json:"error,omitempty"`
}

func (r ToolResult) MarshalJSON() ([]byte, error) {
	var errMsg string
	if r.Error != nil {
		errMsg = r.Error.Error()
	}
	return json.Marshal(toolResultJSON{
		Content: r.Content,
		Error:   errMsg,
	})
}

func (r *ToolResult) UnmarshalJSON(data []byte) error {
	var tmp toolResultJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Content = tmp.Content
	if tmp.Error != "" {
		r.Error = errors.New(tmp.Error)
	}
	return nil
}

// ToolRegistry manages available tools.
type ToolRegistry struct {
	tools     map[string]Tool
	order     []string
	validator map[string]ToolValidator
	mu        sync.RWMutex
}

type ToolValidator interface {
	Validate(input any) error
}

// NewToolRegistry creates a new empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:     make(map[string]Tool),
		order:     make([]string, 0),
		validator: make(map[string]ToolValidator),
	}
}

// Register adds a tool to the registry. If a tool with the same name
// already exists, it is replaced.
func (r *ToolRegistry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Definition().Name
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = tool

	def := tool.Definition()

	resolver, err := def.Parameters.Resolve(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to resolve parameters for tool %s: %v", def.Name, err))
	}
	r.validator[def.Name] = resolver
}

// Get returns a tool by name and a boolean indicating whether it was found.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tool names in order of registration.
func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, len(r.order))
	copy(result, r.order)
	return result
}

// Definitions returns all registered tool definitions in order of registration.
func (r *ToolRegistry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		if tool, ok := r.tools[name]; ok {
			result = append(result, tool.Definition())
		}
	}
	return result
}

// ValidateArguments validates the arguments against the tool's schema.
func (r *ToolRegistry) ValidateArguments(name string, arguments json.RawMessage) error {
	v, ok := r.validator[name]
	if !ok {
		return fmt.Errorf("tool %s not found", name)
	}
	var input map[string]any
	if err := json.Unmarshal(arguments, &input); err != nil {
		return err
	}
	if err := v.Validate(input); err != nil {
		return err
	}
	return nil
}
