package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
)

//go:embed read.md
var readDescription string

const (
	defaultReadMaxLines     = 500
	defaultReadMaxLineChars = 2000
)

const readTruncationHint = "Use read_range or a narrower path to inspect a smaller section."

type ReadTool struct {
	workspace string
	restrict  bool
}

func NewReadTool(workspace string, restrict bool) *ReadTool {
	return &ReadTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

type readInput struct {
	Path      string `json:"path"`
	ReadRange []int  `json:"read_range,omitempty"`
}

var readParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"path": {
			Type:        "string",
			MinLength:   new(1),
			Description: "Path to the file or directory to read",
		},
		"read_range": {
			Type:        "array",
			Description: "Line range [start, end] (1-indexed)",
			Items: &jsonschema.Schema{
				Type:    "integer",
				Minimum: new(1.0),
			},
			MinItems: new(2),
			MaxItems: new(2),
		},
	},
	Required: []string{"path"},
}

func (t *ReadTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Read",
		Description: readDescription,
		Parameters:  readParameters,
	}
}

func (t *ReadTool) Execute(ctx context.Context, input readInput) agent.ToolResult {
	path, err := validatePath(input.Path, t.workspace, t.restrict)
	if err != nil {
		return ErrorResult(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return ErrorResult(fmt.Errorf("cannot access %q: %w", path, err))
	}

	if info.IsDir() {
		return t.listDirectory(path)
	}

	return t.readFile(path, input.ReadRange)
}

func (t *ReadTool) listDirectory(path string) agent.ToolResult {
	entries, err := os.ReadDir(path)
	if err != nil {
		return ErrorResult(fmt.Errorf("cannot read directory %q: %w", path, err))
	}

	var sb strings.Builder
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		sb.WriteString(name)
		sb.WriteString("\n")
	}

	return TextResult(truncateToolOutput(sb.String(), 0, readTruncationHint))
}

func (t *ReadTool) readFile(path string, readRange []int) agent.ToolResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return ErrorResult(fmt.Errorf("cannot read file %q: %w", path, err))
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	start := 1
	end := len(lines)
	truncated := false

	if len(readRange) == 2 {
		start = readRange[0]
		end = readRange[1]
	} else if end > defaultReadMaxLines {
		end = defaultReadMaxLines
		truncated = true
	}

	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return TextResult("")
	}

	ext := filepath.Ext(path)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s (%d lines%s)\n", path, len(lines), extInfo(ext))
	for i := start; i <= end; i++ {
		fmt.Fprintf(&sb, "%d: %s\n", i, truncateReadLine(lines[i-1]))
	}

	if truncated {
		fmt.Fprintf(&sb, "... (truncated, showing lines %d-%d of %d; use read_range to read more)\n", start, end, len(lines))
	}

	return TextResult(truncateToolOutput(sb.String(), 0, readTruncationHint))
}

func truncateReadLine(line string) string {
	runes := []rune(line)
	if len(runes) <= defaultReadMaxLineChars {
		return line
	}

	return string(runes[:defaultReadMaxLineChars]) + "... (truncated)"
}

func extInfo(ext string) string {
	if ext != "" {
		return ", " + ext + " file"
	}
	return ""
}
