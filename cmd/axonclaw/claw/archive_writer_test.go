package claw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"
	"github.com/stretchr/testify/require"
)

func TestAppendArchiveMessage_AppendsToDailyThreadFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := bus.ContextWithMetadata(context.Background(), bus.Metadata{ThreadID: "th/test:1"})

	require.NoError(t, AppendArchiveMessage(ctx, workspace, agent.Message{
		Role:    agent.RoleUser,
		Content: &agent.Content{Text: new("hello")},
	}))
	require.NoError(t, AppendArchiveMessage(ctx, workspace, agent.Message{
		Role:    agent.RoleAssistant,
		Content: &agent.Content{Text: new("world")},
	}))

	archiveDir := filepath.Join(workspace, ".axonclaw", "messages", "archives")
	entries, err := os.ReadDir(archiveDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Name(), "th_test_1")

	data, err := os.ReadFile(filepath.Join(archiveDir, entries[0].Name()))
	require.NoError(t, err)

	content := string(data)
	require.Contains(t, content, "hello")
	require.Contains(t, content, "world")
	require.Equal(t, 2, strings.Count(content, "## "))
}
