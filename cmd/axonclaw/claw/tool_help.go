package claw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/looplj/axonhub/axon/agent"
)

type AxonClawHelpTool struct {
	execPath string
}

func NewAxonClawHelpTool() *AxonClawHelpTool {
	execPath, err := os.Executable()
	if err != nil {
		execPath = "axonclaw"
	}
	return &AxonClawHelpTool{execPath: execPath}
}

func (t *AxonClawHelpTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name: "AxonClawHelp",
		Description: `Get the AxonClaw command reference including subcommands, flags, and usage.

Call this tool when you need to know what axonclaw commands are available, their syntax, or how to use them.
This is the authoritative source for axonclaw capabilities — always check here before guessing command usage.`,
		Parameters: jsonschema.Schema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
			Properties: map[string]*jsonschema.Schema{
				"all": {
					Type:        "boolean",
					Description: "Show full command reference with all subcommands and flags",
				},
				"subcommand": {
					Type:        "string",
					Description: "Show help for a specific subcommand (e.g., 'node', 'remote', 'chat'). If specified, 'all' parameter is ignored.",
				},
			},
		},
	}
}

type helpParams struct {
	All        bool   `json:"all,omitempty"`
	Subcommand string `json:"subcommand,omitempty"`
}

func (t *AxonClawHelpTool) Execute(ctx context.Context, params json.RawMessage) agent.ToolResult {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var p helpParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return agent.ToolResult{Error: fmt.Errorf("invalid parameters: %w", err)}
		}
	}

	var args []string
	if p.Subcommand != "" {
		args = []string{p.Subcommand, "--help"}
	} else if p.All {
		args = []string{"help", "--all"}
	} else {
		args = []string{"help"}
	}

	cmd := exec.CommandContext(ctx, t.execPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return agent.ToolResult{Error: fmt.Errorf("axonclaw help timed out")}
		}
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return agent.ToolResult{Error: fmt.Errorf("axonclaw help failed: %s", errMsg)}
	}

	output := stdout.String()
	if output == "" {
		output = "(no output)"
	}

	return agent.ToolResult{
		Content: agent.Content{Text: &output},
	}
}
