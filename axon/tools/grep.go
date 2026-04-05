package tools

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/pkg/grep"
)

//go:embed grep.md
var grepDescription string

const (
	grepTruncationHint = "Use path, glob, head_limit, or a more specific pattern to narrow matches."
	grepOutputMaxLines = 500
)

type GrepTool struct {
	workspace string
	restrict  bool
}

func NewGrepTool(workspace string, restrict bool) *GrepTool {
	return &GrepTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

type grepInput struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Glob       string `json:"glob,omitempty"`
	OutputMode string `json:"output_mode,omitempty"`
	Before     *int   `json:"before,omitempty"`
	After      *int   `json:"after,omitempty"`
	Context    *int   `json:"context,omitempty"`
	LineNumber *bool  `json:"line_number,omitempty"`
	IgnoreCase *bool  `json:"ignore_case,omitempty"`
	FileType   string `json:"type,omitempty"`
	HeadLimit  int    `json:"head_limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Multiline  bool   `json:"multiline,omitempty"`
	Literal    bool   `json:"literal,omitempty"`
}

var grepParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"pattern": {
			Type:        "string",
			MinLength:   new(1),
			Description: "The regular expression pattern to search for in file contents",
		},
		"path": {
			Type:        "string",
			Description: "File or directory to search in. Defaults to workspace root.",
		},
		"glob": {
			Type:        "string",
			Description: "Glob pattern to filter files (e.g. \"*.js\", \"*.{ts,tsx}\")",
		},
		"output_mode": {
			Type:        "string",
			Description: "Output mode: \"content\" shows matching lines, \"files_with_matches\" shows file paths, \"count\" shows match counts. Defaults to \"files_with_matches\".",
			Enum:        []any{"content", "files_with_matches", "count"},
		},
		"before": {
			Type:        "integer",
			Minimum:     new(0.0),
			Description: "Number of lines to show before each match. Requires output_mode: \"content\".",
		},
		"after": {
			Type:        "integer",
			Minimum:     new(0.0),
			Description: "Number of lines to show after each match. Requires output_mode: \"content\".",
		},
		"context": {
			Type:        "integer",
			Minimum:     new(0.0),
			Description: "Number of lines to show before and after each match. Requires output_mode: \"content\".",
		},
		"line_number": {
			Type:        "boolean",
			Description: "Show line numbers in output. Requires output_mode: \"content\". Defaults to true.",
		},
		"ignore_case": {
			Type:        "boolean",
			Description: "Case insensitive search",
		},
		"type": {
			Type:        "string",
			Description: "File type to search (e.g., js, py, go, rust)",
		},
		"head_limit": {
			Type:        "integer",
			Minimum:     new(0.0),
			Description: "Limit output to first N lines/entries. Defaults to 0 (unlimited).",
		},
		"offset": {
			Type:        "integer",
			Minimum:     new(0.0),
			Description: "Skip first N lines/entries before applying head_limit. Defaults to 0.",
		},
		"multiline": {
			Type:        "boolean",
			Description: "Enable multiline mode where patterns can span lines. Default: false.",
		},
		"literal": {
			Type:        "boolean",
			Description: "Treat the pattern as a literal string instead of a regex. Default: false.",
		},
	},
	Required: []string{"pattern"},
}

func (t *GrepTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Grep",
		Description: grepDescription,
		Parameters:  grepParameters,
	}
}

func (t *GrepTool) Execute(ctx context.Context, input grepInput) agent.ToolResult {
	searchRoot := normalizeWorkspacePath(t.workspace)
	searchPath := "."
	pathPrefix := ""
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
		pathPrefix = scope.displayPrefix
	}

	searcher := grep.NewSearcher(searchRoot)
	opts := grep.Options{
		Pattern:    input.Pattern,
		Path:       searchPath,
		PathPrefix: pathPrefix,
		Glob:       input.Glob,
		OutputMode: input.OutputMode,
		Before:     input.Before,
		After:      input.After,
		Context:    input.Context,
		LineNumber: input.LineNumber,
		IgnoreCase: input.IgnoreCase,
		FileType:   input.FileType,
		HeadLimit:  input.HeadLimit,
		Offset:     input.Offset,
		Multiline:  input.Multiline,
		Literal:    input.Literal,
	}

	result, err := searcher.Search(ctx, opts)
	if err != nil {
		return ErrorResult(err)
	}

	text := result.Text
	if result.Truncated {
		text += fmt.Sprintf("Results were also capped by grep at %d matches before tool-level truncation.\n", grep.MaxMatches)
	}

	text = truncateToolOutputLines(text, grepOutputMaxLines, grepTruncationHint)

	return TextResult(truncateToolOutput(text, 0, grepTruncationHint))
}
