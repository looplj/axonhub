package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/memory"
)

//go:embed memory.md
var memoryDescription string

type MemoryAddTool struct {
	store memory.Store
}

func NewMemoryAddTool(store memory.Store) *MemoryAddTool {
	return &MemoryAddTool{
		store: store,
	}
}

type memoryAddInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Source  string `json:"source,omitempty"`
}

var memoryAddParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"path": {
			Type:        "string",
			MinLength:   new(1),
			Description: "Memory path following conventions: 'daily/YYYY-MM-DD', 'longterm/MEMORY', 'project/{name}', or 'session/{id}'",
		},
		"content": {
			Type:        "string",
			MinLength:   new(1),
			Description: "The content to remember",
		},
		"source": {
			Type:        "string",
			Description: "Optional source identifier for this memory",
		},
	},
	Required: []string{"path", "content"},
}

func (t *MemoryAddTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "MemoryAdd",
		Description: memoryDescription,
		Parameters:  memoryAddParameters,
	}
}

func (t *MemoryAddTool) Execute(ctx context.Context, input memoryAddInput) agent.ToolResult {
	if err := t.store.Add(ctx, input.Path, input.Content, input.Source); err != nil {
		return ErrorResult(fmt.Errorf("failed to add memory: %w", err))
	}

	return TextResult(fmt.Sprintf("Memory added to path: %s", input.Path))
}

type MemoryGetTool struct {
	store memory.Store
}

func NewMemoryGetTool(store memory.Store) *MemoryGetTool {
	return &MemoryGetTool{
		store: store,
	}
}

type memoryGetInput struct {
	Path string `json:"path"`
}

var memoryGetParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"path": {
			Type:        "string",
			MinLength:   new(1),
			Description: "The path identifier to retrieve memories from",
		},
	},
	Required: []string{"path"},
}

func (t *MemoryGetTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "MemoryGet",
		Description: "Get all memory content from a specified path.",
		Parameters:  memoryGetParameters,
	}
}

func (t *MemoryGetTool) Execute(ctx context.Context, input memoryGetInput) agent.ToolResult {
	content, err := t.store.Get(ctx, input.Path)
	if err != nil {
		return ErrorResult(fmt.Errorf("failed to get memory: %w", err))
	}

	if content == "" {
		return TextResult("No memories found at this path.")
	}

	return TextResult(fmt.Sprintf("Memory content at %s:\n\n%s", input.Path, content))
}

type MemorySearchTool struct {
	store memory.Store
}

func NewMemorySearchTool(store memory.Store) *MemorySearchTool {
	return &MemorySearchTool{
		store: store,
	}
}

type memorySearchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

var memorySearchParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"query": {
			Type:        "string",
			MinLength:   new(1),
			Description: "Search query",
		},
		"limit": {
			Type:        "integer",
			Description: "Max results to return (default: 10)",
		},
	},
	Required: []string{"query"},
}

func (t *MemorySearchTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "MemorySearch",
		Description: "Search previously saved memories by keyword. Returns matching memory entries.",
		Parameters:  memorySearchParameters,
	}
}

func (t *MemorySearchTool) Execute(ctx context.Context, input memorySearchInput) agent.ToolResult {
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}

	entries, err := t.store.Search(ctx, input.Query, limit)
	if err != nil {
		return ErrorResult(fmt.Errorf("failed to search memories: %w", err))
	}

	if len(entries) == 0 {
		return TextResult("No matching memories found.")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d result(s):\n\n", len(entries))
	for _, entry := range entries {
		fmt.Fprintf(&sb, "[%s] %s\n", entry.ID, entry.Path)
		fmt.Fprintf(&sb, "Source: %s\n", entry.Source)
		fmt.Fprintf(&sb, "Content: %s\n", entry.Content)
		fmt.Fprintf(&sb, "Created: %s\n\n", entry.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	return TextResult(sb.String())
}

type MemoryListTool struct {
	store memory.Store
}

func NewMemoryListTool(store memory.Store) *MemoryListTool {
	return &MemoryListTool{
		store: store,
	}
}

type memoryListInput struct {
	Limit int `json:"limit,omitempty"`
}

var memoryListParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"limit": {
			Type:        "integer",
			Description: "Max results to return (default: 20)",
		},
	},
	Required: []string{},
}

func (t *MemoryListTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "MemoryList",
		Description: "List all memory entries. Use this to see what memories have been stored.",
		Parameters:  memoryListParameters,
	}
}

func (t *MemoryListTool) Execute(ctx context.Context, input memoryListInput) agent.ToolResult {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	entries, err := t.store.List(ctx, limit)
	if err != nil {
		return ErrorResult(fmt.Errorf("failed to list memories: %w", err))
	}

	if len(entries) == 0 {
		return TextResult("No memories found.")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d memory entr(y/ies):\n\n", len(entries))
	for _, entry := range entries {
		fmt.Fprintf(&sb, "[%s] %s\n", entry.ID, entry.Path)
		fmt.Fprintf(&sb, "Source: %s\n", entry.Source)
		fmt.Fprintf(&sb, "Content: %s\n", entry.Content)
		fmt.Fprintf(&sb, "Created: %s\n\n", entry.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	return TextResult(sb.String())
}

type MemoryDeleteTool struct {
	store memory.Store
}

func NewMemoryDeleteTool(store memory.Store) *MemoryDeleteTool {
	return &MemoryDeleteTool{
		store: store,
	}
}

type memoryDeleteInput struct {
	Path string `json:"path"`
}

var memoryDeleteParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"path": {
			Type:        "string",
			MinLength:   new(1),
			Description: "The path identifier to delete memories from",
		},
	},
	Required: []string{"path"},
}

func (t *MemoryDeleteTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "MemoryDelete",
		Description: "Delete all memory entries at a specified path. Use with caution as this cannot be undone.",
		Parameters:  memoryDeleteParameters,
	}
}

func (t *MemoryDeleteTool) Execute(ctx context.Context, input memoryDeleteInput) agent.ToolResult {
	if err := t.store.Delete(ctx, input.Path); err != nil {
		return ErrorResult(fmt.Errorf("failed to delete memory: %w", err))
	}

	return TextResult(fmt.Sprintf("Memory deleted at path: %s", input.Path))
}
