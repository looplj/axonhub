package prompts

import (
	"strings"
	"testing"

	"github.com/looplj/axonhub/axon/api"
)

func TestBuildSystemPromptsWrapsSkillsInsidePathHeader(t *testing.T) {
	skillContent := "Use the skill exactly as documented."
	prompts := BuildSystemPrompts(PromptEnv{}, &Bootstrap{
		System: MarkdownFile{
			Content: "Base system prompt.",
			Path:    "/tmp/.axonclaw/SYSTEM.md",
		},
	}, []*api.AgentBootstrapAgentBootstrapSkillsAgentSkillDefinition{
		{
			Name:    "skill-a",
			Content: &skillContent,
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

	if !strings.Contains(systemPrompt, "Base system prompt.\n\n---\n\n## Skill: skill-a\n\nUse the skill exactly as documented.") {
		t.Fatalf("skills not appended inside wrapped system prompt: %q", systemPrompt)
	}
}

func TestBuildSystemPromptsReturnsSkillsWhenSystemFileIsEmpty(t *testing.T) {
	skillContent := "skill body"
	prompts := BuildSystemPrompts(PromptEnv{}, &Bootstrap{
		System: MarkdownFile{
			Path: "/tmp/.axonclaw/SYSTEM.md",
		},
	}, []*api.AgentBootstrapAgentBootstrapSkillsAgentSkillDefinition{
		{
			Name:    "skill-only",
			Content: &skillContent,
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

	if !strings.Contains(systemPrompt, "## Skill: skill-only") {
		t.Fatalf("expected prompt to include skill content: %q", systemPrompt)
	}

	if strings.Contains(systemPrompt, "/tmp/.axonclaw/SYSTEM.md") {
		t.Fatalf("expected prompt to avoid leaking file path: %q", systemPrompt)
	}
}

func TestDefaultInstructionTemplateContainsBootstrapFileMaintenanceGuidance(t *testing.T) {
	instruction, err := RenderInstructionTemplate(PromptEnv{Workspace: "/workspace with spaces"})
	if err != nil {
		t.Fatalf("render instruction template: %v", err)
	}

	expectedSnippets := []string{
		"## Identity, User, And Soul Maintenance",
		"Update `/workspace with spaces/.axonclaw/IDENTITY.md` when you learn stable facts",
		"Update `/workspace with spaces/.axonclaw/USER.md` when you learn durable facts about the user",
		"Update `/workspace with spaces/.axonclaw/SOUL.md` when you learn durable guidance",
		"Do not write one-off task details, temporary context, or fleeting emotional states into these files.",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(instruction, snippet) {
			t.Fatalf("expected instruction template to contain %q", snippet)
		}
	}
}
