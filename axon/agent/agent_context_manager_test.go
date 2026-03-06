package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contextTestProvider struct {
	mu     sync.Mutex
	count  int
	inputs [][]Message
}

func (p *contextTestProvider) Chat(_ context.Context, _ string, _ []ToolDefinition, messages []Message) (Response, error) {
	p.mu.Lock()
	p.inputs = append(p.inputs, cloneMessages(messages))
	p.count++
	count := p.count
	p.mu.Unlock()

	text := fmt.Sprintf("assistant-%d", count)
	return Response{
		Messages: []Message{{Role: RoleAssistant, Content: &Content{Text: &text}}},
	}, nil
}

func (p *contextTestProvider) ChatStream(_ context.Context, _ string, _ []ToolDefinition, _ []Message) (<-chan StreamEvent, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestAgentWithContextManager_CompactsHistory(t *testing.T) {
	store := NewContextManagerMemoryStore()
	cfg := DefaultContextManagerConfig()
	cfg.MaxRecentMessages = 2
	cfg.Summarizer = testSummarizer{}
	cm, err := NewSmartContextManager(cfg, store)
	require.NoError(t, err)

	provider := &contextTestProvider{}
	a := New(Config{Model: "test-model", MaxIterations: 5}, provider, WithContextManager(cm))

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		msg := fmt.Sprintf("user-%d", i+1)
		err := a.Process(ctx, Content{Text: &msg})
		require.NoError(t, err)
	}

	history := a.Messages()
	assert.LessOrEqual(t, len(history), 4)

	require.Len(t, provider.inputs, 3)
	thirdCall := provider.inputs[2]
	require.NotEmpty(t, thirdCall)
	assert.Equal(t, RoleUser, thirdCall[0].Role)
	assert.Equal(t, "test-summary", thirdCall[0].Content.String())
}

var _ Provider = (*contextTestProvider)(nil)
