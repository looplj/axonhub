package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
)

//go:embed edit.md
var editDescription string

type EditTool struct {
	workspace string
	restrict  bool
}

func NewEditTool(workspace string, restrict bool) *EditTool {
	return &EditTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

type editInput struct {
	Path       string `json:"path"`
	OldText    string `json:"old_text"`
	NewText    string `json:"new_text"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

var editParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"path": {
			Type:        "string",
			MinLength:   new(1),
			Description: "Path to the file to edit",
		},
		"old_text": {
			Type:        "string",
			MinLength:   new(1),
			Description: "Text to search for in the file",
		},
		"new_text": {
			Type:        "string",
			Description: "Text to replace old_text with",
		},
		"replace_all": {
			Type:        "boolean",
			Description: "If true, replace all occurrences of old_text. Defaults to false",
		},
	},
	Required: []string{"path", "old_text", "new_text"},
}

func (t *EditTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Edit",
		Description: editDescription,
		Parameters:  editParameters,
	}
}

func (t *EditTool) Execute(ctx context.Context, input editInput) agent.ToolResult {
	path, err := validatePath(input.Path, t.workspace, t.restrict)
	if err != nil {
		return ErrorResult(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ErrorResult(fmt.Errorf("cannot read file %q: %w", path, err))
	}

	content := string(data)
	count := strings.Count(content, input.OldText)

	if count == 0 {
		return ErrorResult(fmt.Errorf("old_text not found in %s", path))
	}
	if !input.ReplaceAll && count > 1 {
		return ErrorResult(fmt.Errorf("old_text found %d times in %s; must match exactly once — use replace_all to replace every occurrence", count, path))
	}

	n := 1
	if input.ReplaceAll {
		n = -1
	}
	newContent := strings.Replace(content, input.OldText, input.NewText, n)

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return ErrorResult(fmt.Errorf("cannot write file %q: %w", path, err))
	}

	return TextResult(fmt.Sprintf("Successfully edited %s", path))
}
