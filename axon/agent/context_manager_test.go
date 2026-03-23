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
		{Role: RoleUser, Content: &Content{Text: new("u1")}, RoundIndex: 1},
		{Role: RoleAssistant, Content: &Content{Text: new("a1")}, RoundIndex: 1},
		{Role: RoleUser, Content: &Content{Text: new("u2")}, RoundIndex: 2},
		{Role: RoleAssistant, Content: &Content{Text: new("a2")}, RoundIndex: 2},
		{Role: RoleUser, Content: &Content{Text: new("u3")}, RoundIndex: 3},
		{Role: RoleAssistant, Content: &Content{Text: new("a3")}, RoundIndex: 3},
	}
	cm.AddMessages(ctx, history...)

	res := cm.BuildMessages(ctx)
	require.GreaterOrEqual(t, len(res), 2)
	require.Equal(t, RoleUser, res[0].Role)
	require.Equal(t, "test-summary", res[0].Content.String())

	snapshot := cm.Snapshot()
	require.Empty(t, snapshot.Summary)
	require.Equal(t, int64(1), snapshot.CompactionCount)
}

func TestContextManagerDecorator_CanWrapAnotherStrategy(t *testing.T) {
	base := NewSimpleContextManager(nil)
	ctx := context.Background()
	msgs := []Message{
		{Role: RoleUser, Content: &Content{Text: new("one")}, RoundIndex: 1},
		{Role: RoleAssistant, Content: &Content{Text: new("two")}, RoundIndex: 1},
		{Role: RoleUser, Content: &Content{Text: new("three")}, RoundIndex: 2},
	}
	base.AddMessages(ctx, msgs...)

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

func TestAdjustCompactionCut_RespectsRoundIndexGrouping(t *testing.T) {
	messages := []Message{
		newTextMessage(RoleUser, "u1"),
		{Role: RoleAssistant, Content: &Content{Text: new("a1")}, RoundIndex: 100},
		{Role: RoleAssistant, Content: &Content{Text: new("a2")}, RoundIndex: 100},
		newTextMessage(RoleUser, "u2"),
	}

	cut := adjustCompactionCut(messages, 2)
	require.Equal(t, 3, cut)
}

func TestAdjustCompactionCut_RespectsToolUseAndToolResultGrouping(t *testing.T) {
	toolID := "tool-1"
	messages := []Message{
		newTextMessage(RoleUser, "u1"),
		{Role: RoleAssistant, ToolCall: &ToolCall{ID: toolID, Name: "search", Input: "{}"}},
		{Role: RoleTool, ToolUseID: &toolID, Content: &Content{Text: new("tool-result")}},
		newTextMessage(RoleAssistant, "a2"),
	}

	cut := adjustCompactionCut(messages, 2)
	require.Equal(t, 3, cut)
}

func TestCountUniqueRounds(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		want     int
	}{
		{
			name:     "empty messages",
			messages: []Message{},
			want:     0,
		},
		{
			name: "no round index",
			messages: []Message{
				newTextMessage(RoleUser, "u1"),
				newTextMessage(RoleAssistant, "a1"),
			},
			want: 0,
		},
		{
			name: "single round",
			messages: []Message{
				{Role: RoleUser, Content: &Content{Text: new("u1")}, RoundIndex: 1},
				{Role: RoleAssistant, Content: &Content{Text: new("a1")}, RoundIndex: 1},
			},
			want: 1,
		},
		{
			name: "multiple rounds",
			messages: []Message{
				{Role: RoleUser, Content: &Content{Text: new("u1")}, RoundIndex: 1},
				{Role: RoleAssistant, Content: &Content{Text: new("a1")}, RoundIndex: 1},
				{Role: RoleUser, Content: &Content{Text: new("u2")}, RoundIndex: 2},
				{Role: RoleAssistant, Content: &Content{Text: new("a2")}, RoundIndex: 2},
			},
			want: 2,
		},
		{
			name: "mixed with and without round index",
			messages: []Message{
				newTextMessage(RoleUser, "u0"),
				{Role: RoleUser, Content: &Content{Text: new("u1")}, RoundIndex: 1},
				{Role: RoleAssistant, Content: &Content{Text: new("a1")}, RoundIndex: 1},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countUniqueRounds(tt.messages)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFindCutIndexForRounds(t *testing.T) {
	tests := []struct {
		name       string
		messages   []Message
		keepRounds int
		want       int
	}{
		{
			name:       "empty messages",
			messages:   []Message{},
			keepRounds: 2,
			want:       0,
		},
		{
			name:       "keep all rounds",
			messages:   []Message{{RoundIndex: 1}, {RoundIndex: 2}},
			keepRounds: 5,
			want:       0,
		},
		{
			name: "cut to keep 1 round",
			messages: []Message{
				{Role: RoleUser, RoundIndex: 1},
				{Role: RoleAssistant, RoundIndex: 1},
				{Role: RoleUser, RoundIndex: 2},
				{Role: RoleAssistant, RoundIndex: 2},
			},
			keepRounds: 1,
			want:       2,
		},
		{
			name: "cut to keep 2 rounds",
			messages: []Message{
				{Role: RoleUser, RoundIndex: 1},
				{Role: RoleAssistant, RoundIndex: 1},
				{Role: RoleUser, RoundIndex: 2},
				{Role: RoleAssistant, RoundIndex: 2},
				{Role: RoleUser, RoundIndex: 3},
				{Role: RoleAssistant, RoundIndex: 3},
			},
			keepRounds: 2,
			want:       2,
		},
		{
			name: "messages without round index at start",
			messages: []Message{
				newTextMessage(RoleUser, "u0"),
				{Role: RoleUser, RoundIndex: 1},
				{Role: RoleAssistant, RoundIndex: 1},
				{Role: RoleUser, RoundIndex: 2},
			},
			keepRounds: 1,
			want:       3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findCutIndexForRounds(tt.messages, tt.keepRounds)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildMessages_CompactsByRounds(t *testing.T) {
	store := NewContextManagerMemoryStore()
	cfg := DefaultContextManagerConfig()
	cfg.MaxRecentMessages = 2
	cfg.Summarizer = testSummarizer{}

	cm, err := NewSmartContextManager(cfg, store)
	require.NoError(t, err)

	ctx := context.Background()
	history := []Message{
		{Role: RoleUser, Content: &Content{Text: new("u1")}, RoundIndex: 1},
		{Role: RoleAssistant, Content: &Content{Text: new("a1")}, RoundIndex: 1},
		{Role: RoleTool, ToolUseID: new("tool-1"), Content: &Content{Text: new("r1")}, RoundIndex: 1},
		{Role: RoleUser, Content: &Content{Text: new("u2")}, RoundIndex: 2},
		{Role: RoleAssistant, Content: &Content{Text: new("a2")}, RoundIndex: 2},
		{Role: RoleUser, Content: &Content{Text: new("u3")}, RoundIndex: 3},
		{Role: RoleAssistant, Content: &Content{Text: new("a3")}, RoundIndex: 3},
	}
	cm.AddMessages(ctx, history...)

	res := cm.BuildMessages(ctx)
	require.Equal(t, RoleUser, res[0].Role)
	require.Equal(t, "test-summary", res[0].Content.String())

	retainedRounds := countUniqueRounds(res[1:])
	require.LessOrEqual(t, retainedRounds, 2)
}

func newTextMessage(role Role, text string) Message {
	return Message{
		Role:    role,
		Content: &Content{Text: &text},
	}
}
