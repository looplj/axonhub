package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContextManagerFileStore_SaveLoadAndArchive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewContextManagerFileStore(dir)

	ctx := context.Background()
	state := ContextManagerState{
		Summary:         "summary",
		CompactionCount: 1,
	}
	current := []Message{
		newTextMessage(RoleUser, "u2"),
		newTextMessage(RoleAssistant, "a2"),
	}
	archived := []Message{
		newTextMessage(RoleUser, "u1"),
		newTextMessage(RoleAssistant, "a1"),
	}

	require.NoError(t, store.Save(ctx, state, current, archived))

	loadedState, loadedMessages, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, state.Summary, loadedState.Summary)
	require.Equal(t, state.CompactionCount, loadedState.CompactionCount)
	require.Len(t, loadedMessages, 2)
	require.Equal(t, RoleUser, loadedMessages[0].Role)

	messagesData, err := os.ReadFile(filepath.Join(dir, "messages.json"))
	require.NoError(t, err)
	require.NotEmpty(t, messagesData)

	indexData, err := os.ReadFile(filepath.Join(dir, "index.json"))
	require.NoError(t, err)
	var index map[string]any
	require.NoError(t, json.Unmarshal(indexData, &index))
	archives, ok := index["archives"].([]any)
	require.True(t, ok)
	require.Len(t, archives, 1)

	entries, err := os.ReadDir(filepath.Join(dir, "archives"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Name(), "archive-")
	require.True(t, strings.HasSuffix(entries[0].Name(), ".md"))

	archiveData, err := os.ReadFile(filepath.Join(dir, "archives", entries[0].Name()))
	require.NoError(t, err)

	content := string(archiveData)
	require.Contains(t, content, "# Context Archive")
	require.Contains(t, content, "Message count**: 2")
	require.Contains(t, content, "u1")
	require.Contains(t, content, "a1")
}

func TestSmartContextManager_ClearMessages_ArchivesAndClears(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewContextManagerFileStore(dir)

	cfg := DefaultContextManagerConfig()
	cfg.Summarizer = testSummarizer{}

	cm, err := NewSmartContextManager(cfg, store)
	require.NoError(t, err)

	ctx := context.Background()
	cm.AddMessages(ctx,
		newTextMessage(RoleUser, "hello"),
		newTextMessage(RoleAssistant, "world"),
	)

	cm.ClearMessages(ctx)
	require.Empty(t, cm.Messages(ctx))

	indexData, err := os.ReadFile(filepath.Join(dir, "index.json"))
	require.NoError(t, err)

	var index contextManagerIndexFile
	require.NoError(t, json.Unmarshal(indexData, &index))
	require.Len(t, index.Archives, 1)

	archivePath := filepath.Join(dir, filepath.FromSlash(index.Archives[0].File))
	archiveData, err := os.ReadFile(archivePath)
	require.NoError(t, err)

	content := string(archiveData)
	require.Contains(t, content, "hello")
	require.Contains(t, content, "world")

	_, loadedMessages, err := store.Load(ctx)
	require.NoError(t, err)
	require.Empty(t, loadedMessages)
}

func TestContextManagerFileStore_LoadMissingReturnsEmpty(t *testing.T) {
	t.Parallel()

	store := NewContextManagerFileStore(t.TempDir())
	state, messages, err := store.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, emptyContextState(), state)
	require.Empty(t, messages)
}
