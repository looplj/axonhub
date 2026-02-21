package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/jsonschema-go/jsonschema"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
)

//go:embed write.md
var writeDescription string

type WriteTool struct {
	workspace string
	restrict  bool
}

func NewWriteTool(workspace string, restrict bool) *WriteTool {
	return &WriteTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

type writeInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

var writeParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"path": {
			Type:        "string",
			MinLength:   new(1),
			Description: "Path to the file to write",
		},
		"content": {
			Type:        "string",
			Description: "Content to write to the file",
		},
	},
	Required: []string{"path", "content"},
}

func (t *WriteTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Write",
		Description: writeDescription,
		Parameters:  writeParameters,
	}
}

func (t *WriteTool) Execute(ctx context.Context, input writeInput) agent.ToolResult {
	path, err := validatePath(input.Path, t.workspace, t.restrict)
	if err != nil {
		return ErrorResult(err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ErrorResult(fmt.Errorf("cannot create directory %q: %w", dir, err))
	}

	if err := os.WriteFile(path, []byte(input.Content), 0o644); err != nil {
		return ErrorResult(fmt.Errorf("cannot write file %q: %w", path, err))
	}

	return TextResult(fmt.Sprintf("Successfully wrote %d bytes to %s", len(input.Content), path))
}
