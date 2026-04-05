package claw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"
	"github.com/stretchr/testify/require"
)

func TestAppendArchiveMessage_AppendsToDailyThreadFile(t *testing.T) {
	workspace := t.TempDir()
	archiveDir := filepath.Join(workspace, "messages", "archives")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	t.Setenv("HOME", t.TempDir())
	ctx := bus.ContextWithMetadata(context.Background(), bus.Metadata{ThreadID: "th/test:1"})

	require.NoError(t, AppendArchiveMessage(ctx, archiveDir, agent.Message{
		Role:    agent.RoleUser,
		Content: &agent.Content{Text: new("hello")},
	}))
	require.NoError(t, AppendArchiveMessage(ctx, archiveDir, agent.Message{
		Role:    agent.RoleAssistant,
		Content: &agent.Content{Text: new("world")},
	}))

	entries, err := os.ReadDir(archiveDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Name(), "th_test_1")

	data, err := os.ReadFile(filepath.Join(archiveDir, entries[0].Name()))
	require.NoError(t, err)

	content := string(data)
	require.Contains(t, content, "user: hello")
	require.Contains(t, content, "assistant: world")
	require.Equal(t, 2, strings.Count(content, "\n"))
}

func TestRenderMessage_ToolUse(t *testing.T) {
	msg := agent.Message{
		Role: agent.RoleAssistant,
		ToolCall: &agent.ToolCall{
			ID:    "tool_123",
			Name:  "read_file",
			Input: `{"path": "/src/main.go"}`,
		},
	}

	result := RenderMessage(time.Now(), msg, MessageRenderOptions{TimePrefix: true})
	require.Contains(t, result, "tool:read_file")
	require.Contains(t, result, "id:tool_123")
	require.Contains(t, result, `"path": "/src/main.go"`)
}

func TestRenderMessage_ToolResult(t *testing.T) {
	toolID := "tool_123"
	msg := agent.Message{
		Role:      agent.RoleTool,
		ToolUseID: &toolID,
		Content:   &agent.Content{Text: new("file content here")},
	}

	result := RenderMessage(time.Now(), msg, MessageRenderOptions{TimePrefix: true})
	require.Contains(t, result, "tool_result(tool_123)")
	require.Contains(t, result, "file content here")
}

func TestRenderMessage_ToolError(t *testing.T) {
	toolID := "tool_123"
	isError := true
	msg := agent.Message{
		Role:      agent.RoleTool,
		ToolUseID: &toolID,
		IsError:   &isError,
		Content:   &agent.Content{Text: new("error: file not found")},
	}

	result := RenderMessage(time.Now(), msg, MessageRenderOptions{TimePrefix: true})
	require.Contains(t, result, "tool_error(tool_123)")
	require.Contains(t, result, "error: file not found")
}

func TestRenderMessage_WithThinking(t *testing.T) {
	msg := agent.Message{
		Role: agent.RoleAssistant,
		Content: &agent.Content{Parts: []agent.ContentPart{
			{Type: agent.ContentPartThinking, Thinking: "Let me think..."},
			{Type: agent.ContentPartText, Text: "Here is the answer"},
		}},
	}

	result := RenderMessage(time.Now(), msg, MessageRenderOptions{TimePrefix: true})
	require.Contains(t, result, "[thinking]")
	require.Contains(t, result, "Let me think...")
	require.Contains(t, result, "Here is the answer")
}
