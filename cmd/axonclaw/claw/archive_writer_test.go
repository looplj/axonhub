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
	messagesDir := filepath.Join(workspace, "messages")
	archiveDir := filepath.Join(messagesDir, "archives")
	t.Setenv("HOME", t.TempDir())
	ctx := bus.ContextWithMetadata(context.Background(), bus.Metadata{ThreadID: "th/test:1"})

	require.NoError(t, AppendArchiveMessage(ctx, messagesDir, agent.Message{
		Role:    agent.RoleUser,
		Content: &agent.Content{Text: new("hello")},
	}))
	require.NoError(t, AppendArchiveMessage(ctx, messagesDir, agent.Message{
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

func TestAppendArchiveMessage_SeparatesFilesBySource(t *testing.T) {
	workspace := t.TempDir()
	messagesDir := filepath.Join(workspace, "messages")
	archiveDir := filepath.Join(messagesDir, "archives")

	t.Setenv("HOME", t.TempDir())

	// Main agent message (no source).
	ctxMain := bus.ContextWithMetadata(context.Background(), bus.Metadata{ThreadID: "th-100"})
	require.NoError(t, AppendArchiveMessage(ctxMain, messagesDir, agent.Message{
		Role:    agent.RoleUser,
		Content: &agent.Content{Text: new("main prompt")},
	}))

	// Subagent message with source.
	ctxSub := bus.ContextWithMetadata(context.Background(), bus.Metadata{
		ThreadID: "th-100",
		Source:   "search-docs",
	})
	require.NoError(t, AppendArchiveMessage(ctxSub, messagesDir, agent.Message{
		Role:    agent.RoleAssistant,
		Content: &agent.Content{Text: new("subagent result")},
	}))

	entries, err := os.ReadDir(archiveDir)
	require.NoError(t, err)
	require.Len(t, entries, 2, "main and subagent should write to separate files")

	// Verify file names.
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}

	require.Contains(t, names[0], "th-100")

	// One file should contain the source suffix.
	foundSource := false

	for _, n := range names {
		if strings.Contains(n, "search-docs") {
			foundSource = true
		}
	}

	require.True(t, foundSource, "expected a file with source suffix 'search-docs', got: %v", names)
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
