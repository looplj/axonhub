package summarizer

import (
	"context"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/axon/agent"
)

const defaultSystemPrompt = "You are a conversation summarizer. Produce a concise factual summary that preserves key decisions, constraints, unresolved items, and important context for continuing the conversation."

type ProviderOptions struct {
	Provider      agent.Provider
	Model         string
	SystemPrompt  string
	MaxSummaryLen int
}

type ProviderSummarizer struct {
	provider      agent.Provider
	model         string
	systemPrompt  string
	maxSummaryLen int
}

func NewProvider(opts ProviderOptions) *ProviderSummarizer {
	prompt := strings.TrimSpace(opts.SystemPrompt)
	if prompt == "" {
		prompt = defaultSystemPrompt
	}

	return &ProviderSummarizer{
		provider:      opts.Provider,
		model:         opts.Model,
		systemPrompt:  prompt,
		maxSummaryLen: opts.MaxSummaryLen,
	}
}

func (s *ProviderSummarizer) Summarize(ctx context.Context, messages []agent.Message) (string, error) {
	if s.provider == nil {
		return "", fmt.Errorf("summarizer provider is nil")
	}
	if strings.TrimSpace(s.model) == "" {
		return "", fmt.Errorf("summarizer model is empty")
	}

	prompt := buildSummarizationPrompt(messages, s.maxSummaryLen)
	req := []agent.Message{
		{
			Role:    agent.RoleSystem,
			Content: &agent.Content{Text: &s.systemPrompt},
		},
		{
			Role:    agent.RoleUser,
			Content: &agent.Content{Text: &prompt},
		},
	}

	resp, err := s.provider.Chat(ctx, s.model, nil, req)
	if err != nil {
		return "", err
	}

	summary := strings.TrimSpace(firstText(resp.Messages))
	if summary == "" {
		return "", fmt.Errorf("empty summary response")
	}
	return summary, nil
}

func buildSummarizationPrompt(messages []agent.Message, maxLen int) string {
	var b strings.Builder
	b.WriteString("Summarize the conversation transcript below.\n")
	b.WriteString("Requirements:\n")
	b.WriteString("- Keep critical facts, user intent, constraints, decisions, and unresolved questions.\n")
	b.WriteString("- Exclude filler.\n")
	b.WriteString("- Keep it neutral and factual.\n")
	if maxLen > 0 {
		fmt.Fprintf(&b, "- Keep the summary under %d characters.\n", maxLen)
	}
	b.WriteString("\nTranscript:\n")

	for i, msg := range messages {
		fmt.Fprintf(&b, "%d. role=%s", i+1, msg.Role)
		if msg.ToolUse != nil {
			fmt.Fprintf(&b, " tool_use=%s(%s)", msg.ToolUse.Name, msg.ToolUse.Input)
		}
		if msg.IsError != nil && *msg.IsError {
			fmt.Fprintf(&b, " is_error=true")
		}
		fmt.Fprintf(&b, "\n")

		text := strings.TrimSpace(messageText(msg))
		if text != "" {
			fmt.Fprintf(&b, "%s\n", text)
		}
	}

	return b.String()
}

func firstText(messages []agent.Message) string {
	for _, msg := range messages {
		if msg.Content == nil {
			continue
		}
		if text := strings.TrimSpace(msg.Content.String()); text != "" {
			return text
		}
	}
	return ""
}

func messageText(msg agent.Message) string {
	if msg.Content == nil {
		return ""
	}
	return msg.Content.String()
}

var _ agent.Summarizer = (*ProviderSummarizer)(nil)
