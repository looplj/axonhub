package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/pkg/glob"
)

//go:embed glob.md
var globDescription string

type GlobTool struct {
	workspace string
	restrict  bool
}

func NewGlobTool(workspace string, restrict bool) *GlobTool {
	return &GlobTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

var globParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"pattern": {
			Type:        "string",
			MinLength:   new(1),
			Description: "The glob pattern to match files against",
		},
		"path": {
			Type:        "string",
			Description: "The directory to search in. Defaults to workspace root.",
		},
	},
	Required: []string{"pattern"},
}

func (t *GlobTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Glob",
		Description: globDescription,
		Parameters:  globParameters,
	}
}

func (t *GlobTool) Execute(ctx context.Context, input globInput) agent.ToolResult {
	searchRoot := normalizeWorkspacePath(t.workspace)
	searchPath := "."
	displayPrefix := ""
	if input.Path != "" {
		resolved, err := validatePath(input.Path, t.workspace, t.restrict)
		if err != nil {
			return ErrorResult(err)
		}

		scope, err := resolveFSScope(resolved, t.workspace)
		if err != nil {
			return ErrorResult(err)
		}

		searchRoot = scope.root
		searchPath = scope.path
		displayPrefix = scope.displayPrefix
	}

	globber := glob.NewGlobber(searchRoot)
	result, err := globber.Glob(ctx, glob.Options{
		Pattern: input.Pattern,
		Path:    searchPath,
	})
	if err != nil {
		return ErrorResult(err)
	}

	if len(result.Matches) == 0 {
		return TextResult("No files found matching pattern.")
	}

	var sb strings.Builder
	for _, path := range result.Matches {
		sb.WriteString(displayPath(displayPrefix, path))
		sb.WriteString("\n")
	}
	if result.Truncated {
		fmt.Fprintf(&sb, "... (showing first %d results)\n", glob.MaxResults)
	}

	return TextResult(sb.String())
}
