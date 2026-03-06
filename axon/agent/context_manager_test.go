package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type testSummarizer struct{}

func (testSummarizer) Summarize(_ context.Context, _ []Message) (string, error) {
	return "test-summary", nil
}

func TestContextManagerPrepare_CompactsAndKeepsRecentHistory(t *testing.T) {
	store := NewContextManagerMemoryStore()
	cfg := DefaultContextManagerConfig()
	cfg.MaxRecentMessages = 2
	cfg.Summarizer = testSummarizer{}

	cm, err := NewSmartContextManager(cfg, store)
	require.NoError(t, err)

	ctx := context.Background()
	history := []Message{
		newTextMessage(RoleUser, "u1"),
		newTextMessage(RoleAssistant, "a1"),
		newTextMessage(RoleUser, "u2"),
		newTextMessage(RoleAssistant, "a2"),
	}
	cm.AddMessages(ctx, history...)

	res := cm.BuildMessages(ctx)
	require.GreaterOrEqual(t, len(res), 2)
	require.Equal(t, RoleUser, res[0].Role)
	require.Equal(t, "test-summary", res[0].Content.String())
	require.Len(t, cm.Messages(ctx), 3)

	snapshot := cm.Snapshot()
	require.Empty(t, snapshot.Summary)
	require.Equal(t, int64(1), snapshot.CompactionCount)
}

func TestContextManagerDecorator_CanWrapAnotherStrategy(t *testing.T) {
	base := NewSimpleContextManager(nil)
	ctx := context.Background()
	base.AddMessages(ctx, newTextMessage(RoleUser, "one"), newTextMessage(RoleAssistant, "two"))

	innerCfg := DefaultContextManagerConfig()
	innerCfg.MaxRecentMessages = 1
	innerCfg.Summarizer = testSummarizer{}
	inner, err := NewSmartContextManagerWithNext(base, innerCfg, NewContextManagerMemoryStore())
	require.NoError(t, err)

	outerCfg := DefaultContextManagerConfig()
	outerCfg.MaxRecentMessages = 1
	outerCfg.Summarizer = testSummarizer{}
	outer, err := NewSmartContextManagerWithNext(inner, outerCfg, NewContextManagerMemoryStore())
	require.NoError(t, err)

	res := outer.BuildMessages(ctx)
	require.NotEmpty(t, res)
	require.Len(t, outer.Messages(ctx), 2)
}

func TestNewSmartContextManager_RequiresSummarizer(t *testing.T) {
	cfg := DefaultContextManagerConfig()
	cfg.Summarizer = nil

	cm, err := NewSmartContextManager(cfg, NewContextManagerMemoryStore())
	require.Error(t, err)
	require.Nil(t, cm)
	require.Contains(t, err.Error(), "summarizer is required")
}

func TestSimpleContextManager_SetMessagesReplacesHistory(t *testing.T) {
	cm := NewSimpleContextManager([]Message{
		newTextMessage(RoleUser, "old"),
	})

	next := []Message{
		newTextMessage(RoleAssistant, "new-1"),
		newTextMessage(RoleUser, "new-2"),
	}
	cm.SetMessages(context.Background(), next)

	got := cm.Messages(context.Background())
	require.Len(t, got, 2)
	require.Equal(t, "new-1", got[0].Content.String())
	require.Equal(t, "new-2", got[1].Content.String())
}

func TestAdjustCompactionCut_RespectsRequestIndexGrouping(t *testing.T) {
	messages := []Message{
		newTextMessage(RoleUser, "u1"),
		{Role: RoleAssistant, Content: &Content{Text: strPtr("a1")}, RequestIndex: 100},
		{Role: RoleAssistant, Content: &Content{Text: strPtr("a2")}, RequestIndex: 100},
		newTextMessage(RoleUser, "u2"),
	}

	cut := adjustCompactionCut(messages, 2)
	require.Equal(t, 3, cut)
}

func TestAdjustCompactionCut_RespectsToolUseAndToolResultGrouping(t *testing.T) {
	toolID := "tool-1"
	messages := []Message{
		newTextMessage(RoleUser, "u1"),
		{Role: RoleAssistant, ToolUse: &ToolUse{ID: toolID, Name: "search", Input: "{}"}},
		{Role: RoleTool, ToolUseID: &toolID, Content: &Content{Text: strPtr("tool-result")}},
		newTextMessage(RoleAssistant, "a2"),
	}

	cut := adjustCompactionCut(messages, 2)
	require.Equal(t, 3, cut)
}

func newTextMessage(role Role, text string) Message {
	return Message{
		Role:    role,
		Content: &Content{Text: &text},
	}
}

func strPtr(s string) *string {
	return &s
}
