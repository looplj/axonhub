package summarizer

import (
	"context"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/axon/agent"
)

const defaultSystemPrompt = "You are a conversation summarizer. Your task is to produce a concise factual summary that preserves key decisions, constraints, unresolved items, and important context.\n\nOutput requirements:\n- Output ONLY the summary in markdown format\n- Do NOT include any preamble, explanations, or meta-commentary\n- Do NOT wrap the output in code blocks\n- Start directly with the summary content"

type ProviderOptions struct {
	Provider     agent.Provider
	Model        string
	SystemPrompt string
}

type ProviderSummarizer struct {
	provider     agent.Provider
	model        string
	systemPrompt string
}

func NewProvider(opts ProviderOptions) *ProviderSummarizer {
	prompt := strings.TrimSpace(opts.SystemPrompt)
	if prompt == "" {
		prompt = defaultSystemPrompt
	}

	return &ProviderSummarizer{
		provider:     opts.Provider,
		model:        opts.Model,
		systemPrompt: prompt,
	}
}

func (s *ProviderSummarizer) Summarize(ctx context.Context, messages []agent.Message) (string, error) {
	if s.provider == nil {
		return "", fmt.Errorf("summarizer provider is nil")
	}
	if strings.TrimSpace(s.model) == "" {
		return "", fmt.Errorf("summarizer model is empty")
	}

	prompt := buildSummarizationPrompt(messages)
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

func buildSummarizationPrompt(messages []agent.Message) string {
	var b strings.Builder
	b.WriteString("ranscript:\n")

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
		if text := strings.TrimSpace(messageText(msg)); text != "" {
			return text
		}
	}
	return ""
}

func messageText(msg agent.Message) string {
	if msg.Content == nil {
		return ""
	}

	if msg.Content.Text != nil {
		return *msg.Content.Text
	}

	var b strings.Builder

	for _, part := range msg.Content.Parts {
		//nolint:exhaustive // Checked.
		switch part.Type {
		case agent.ContentPartText:
			b.WriteString(part.Text)
		case agent.ContentPartThinking:
			fmt.Fprintf(&b, "**Thinking:**\n%s\n", part.Thinking)
		}
	}

	return b.String()
}

var _ agent.Summarizer = (*ProviderSummarizer)(nil)
