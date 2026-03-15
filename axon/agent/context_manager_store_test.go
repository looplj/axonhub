package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContextManagerFileStore_SaveLoad(t *testing.T) {
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
	require.NoError(t, store.Save(ctx, state, current))

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

	var index contextManagerIndexFile
	require.NoError(t, json.Unmarshal(indexData, &index))
	require.False(t, index.UpdatedAt.IsZero())
}

func TestSmartContextManager_ClearMessages_Clears(t *testing.T) {
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
	require.False(t, index.UpdatedAt.IsZero())

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
