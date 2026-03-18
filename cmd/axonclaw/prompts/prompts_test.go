package prompts

import (
	"strings"
	"testing"
)

func TestBuildSystemPromptsWrapsSystemPrompt(t *testing.T) {
	prompts := BuildSystemPrompts(PromptEnv{}, &Bootstrap{
		System: MarkdownFile{
			Content: "Base system prompt.",
			Path:    "/tmp/.axonclaw/AGENTS.md",
		},
	})

	var systemPrompt string

	for _, p := range prompts {
		if strings.Contains(p, "# How You Should Operate Here") {
			systemPrompt = p
			break
		}
	}

	if systemPrompt == "" {
		t.Fatalf("missing system prompt in results")
	}

	if !strings.HasPrefix(systemPrompt, "# How You Should Operate Here\n\nUse this file as workspace-specific system instruction that should guide how you operate in this environment.\n\n") {
		t.Fatalf("missing system header: %q", systemPrompt)
	}

	if !strings.Contains(systemPrompt, "Base system prompt.") {
		t.Fatalf("system content not found: %q", systemPrompt)
	}
}

func TestBuildSystemPromptsReturnsEmptyWhenSystemFileIsEmpty(t *testing.T) {
	prompts := BuildSystemPrompts(PromptEnv{}, &Bootstrap{
		System: MarkdownFile{
			Path: "/tmp/.axonclaw/AGENTS.md",
		},
	})

	for _, p := range prompts {
		if strings.Contains(p, "# How You Should Operate Here") {
			t.Fatalf("expected no system prompt when empty: %q", p)
		}
	}
}

func TestDefaultSystemTemplateContainsBasicContent(t *testing.T) {
	system := DefaultSystemTemplate
	if !strings.Contains(system, "helpful assistant") {
		t.Fatalf("expected default system template to contain helpful assistant")
	}
}
