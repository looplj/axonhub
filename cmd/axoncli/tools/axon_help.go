package tools

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

const axonHelpDescription = `Get the full AxonCli command reference including all subcommands, flags, and usage.
Call this tool when you need to know what axoncli commands are available, their syntax, or how to use them.
This is the authoritative source for axoncli capabilities — always check here before guessing command usage.`

type AxonHelpTool struct {
	execPath string
}

func NewAxonHelpTool() *AxonHelpTool {
	execPath, err := os.Executable()
	if err != nil {
		execPath = "axoncli" // fallback to relative path
	}
	return &AxonHelpTool{execPath: execPath}
}

func (t *AxonHelpTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "AxonHelp",
		Description: axonHelpDescription,
		Parameters: jsonschema.Schema{
			Schema:     "https://json-schema.org/draft/2020-12/schema",
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{},
		},
	}
}

func (t *AxonHelpTool) Execute(ctx context.Context, _ json.RawMessage) agent.ToolResult {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.execPath, "help", "--all")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return agent.ToolResult{Error: fmt.Errorf("axoncli help timed out")}
		}
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return agent.ToolResult{Error: fmt.Errorf("axoncli help failed: %s", errMsg)}
	}

	output := stdout.String()
	if output == "" {
		output = "(no output)"
	}

	return agent.ToolResult{
		Content: agent.Content{Text: &output},
	}
}
