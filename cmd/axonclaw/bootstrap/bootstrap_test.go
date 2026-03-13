package bootstrap

import (
	"strings"
	"testing"

	"github.com/looplj/axonhub/cmd/axonclaw/prompts"
)

func TestInitPersonalityFilesUsesDefaultSystemTemplateWhenServerPromptMissing(t *testing.T) {
	configDir := t.TempDir()

	initParams := &prompts.InitParams{
		Env: prompts.PromptEnv{
			Workspace: "/workspace/project",
			AgentID:   "agent-1",
		},
		ServerSystemPrompt: "",
	}

	_, err := prompts.Load(configDir, initParams)
	if err != nil {
		t.Fatalf("load prompts with init: %v", err)
	}

	loaded, err := prompts.Load(configDir, nil)
	if err != nil {
		t.Fatalf("load prompts: %v", err)
	}

	if loaded.System.Path == "" {
		t.Fatalf("expected system file path to be set")
	}

	if !strings.Contains(loaded.System.Content, "## Project Context") {
		t.Fatalf("expected default system template content, got %q", loaded.System.Content)
	}

	if !strings.Contains(loaded.System.Content, "/workspace/project") {
		t.Fatalf("expected rendered workspace in system content, got %q", loaded.System.Content)
	}

	if !strings.Contains(loaded.System.Content, "agent-1") {
		t.Fatalf("expected rendered agent id in system content, got %q", loaded.System.Content)
	}
}

func TestInitPersonalityFilesRendersSystemTemplateWithFullEnv(t *testing.T) {
	configDir := t.TempDir()

	env := prompts.PromptEnv{
		Date:         "2026-03-14",
		Timezone:     "UTC+8",
		OS:           "darwin",
		Workspace:    "/workspace/project",
		ThreadID:     "th-123",
		AxonClawPath: "/usr/local/bin/axonclaw",
		SkillsRoot:   "/skills",
		AgentID:      "agent-1",
		AgentName:    "claw",
	}

	initParams := &prompts.InitParams{
		Env:                env,
		ServerSystemPrompt: "{{.Date}}|{{.Timezone}}|{{.OS}}|{{.Workspace}}|{{.ThreadID}}|{{.AxonClawPath}}|{{.SkillsRoot}}|{{.AgentID}}|{{.AgentName}}",
	}

	_, err := prompts.Load(configDir, initParams)
	if err != nil {
		t.Fatalf("load prompts with init: %v", err)
	}

	loaded, err := prompts.Load(configDir, nil)
	if err != nil {
		t.Fatalf("load prompts: %v", err)
	}

	want := []string{
		"2026-03-14",
		"UTC+8",
		"darwin",
		"/workspace/project",
		"th-123",
		"/usr/local/bin/axonclaw",
		"/skills",
		"agent-1",
		"claw",
	}
	for _, part := range want {
		if !strings.Contains(loaded.System.Content, part) {
			t.Fatalf("expected rendered system content to contain %q, got %q", part, loaded.System.Content)
		}
	}
}
