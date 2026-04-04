package subagent

import (
	"context"
	"testing"

	"github.com/looplj/axonhub/axon/agent"
)

type testProvider struct {
	lastMessages []agent.Message
}

func (p *testProvider) Chat(_ context.Context, _ string, _ []agent.ToolDefinition, messages []agent.Message) (agent.Response, error) {
	p.lastMessages = append([]agent.Message(nil), messages...)
	text := "ok"

	return agent.Response{
		Messages: []agent.Message{
			{
				Role:    agent.RoleAssistant,
				Content: &agent.Content{Text: &text},
			},
		},
	}, nil
}

func (p *testProvider) ChatStream(context.Context, string, []agent.ToolDefinition, []agent.Message) (<-chan agent.StreamEvent, error) {
	return nil, nil
}

func TestRunAcceptsMultipleSystemPrompts(t *testing.T) {
	provider := &testProvider{}

	_, err := Run(context.Background(), Config{
		Model:         "test-model",
		SystemPrompts: []string{"first system", "second system"},
		Provider:      provider,
	}, "hello", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(provider.lastMessages) == 0 {
		t.Fatal("expected provider to receive messages")
	}

	if provider.lastMessages[0].Role != agent.RoleSystem {
		t.Fatalf("first message role = %q", provider.lastMessages[0].Role)
	}

	text := ""
	if provider.lastMessages[0].Content.Text != nil {
		text = *provider.lastMessages[0].Content.Text
	}

	if text != "first system" {
		t.Fatalf("first system message = %q, want %q", text, "first system")
	}

	if len(provider.lastMessages) < 2 || provider.lastMessages[1].Role != agent.RoleSystem {
		t.Fatalf("expected second system message, got %#v", provider.lastMessages)
	}

	secondText := ""
	if provider.lastMessages[1].Content.Text != nil {
		secondText = *provider.lastMessages[1].Content.Text
	}

	if secondText != "second system" {
		t.Fatalf("second system message = %q, want %q", secondText, "second system")
	}
}

func TestRunRejectsEmptySystemPrompts(t *testing.T) {
	provider := &testProvider{}

	_, err := Run(context.Background(), Config{
		Model:         "test-model",
		SystemPrompts: []string{"", "  "},
		Provider:      provider,
	}, "hello", nil)
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
}
