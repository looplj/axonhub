package claudecode

import (
	"strings"
	"testing"
)

func TestRewritePromptText(t *testing.T) {
	config := &PromptEnvConfig{
		Platform:   "darwin",
		Shell:      "zsh",
		OSVersion:  "Darwin 24.4.0",
		WorkingDir: "/Users/jack/projects",
	}

	tests := []struct {
		name     string
		input    string
		cchHash  string
		contains string
	}{
		{
			name:     "rewrite platform",
			input:    "Platform: linux",
			cchHash:  "",
			contains: "Platform: darwin",
		},
		{
			name:     "rewrite shell",
			input:    "Shell: bash",
			cchHash:  "",
			contains: "Shell: zsh",
		},
		{
			name:     "rewrite os version",
			input:    "OS Version: Linux 6.5.0-generic",
			cchHash:  "",
			contains: "OS Version: Darwin 24.4.0",
		},
		{
			name:     "rewrite working directory",
			input:    "Working directory: /home/bob/myproject",
			cchHash:  "",
			contains: "Working directory: /Users/jack/projects",
		},
		{
			name:     "rewrite billing header",
			input:    "cc_version=2.1.81.abc",
			cchHash:  "xyz",
			contains: "cc_version=2.1.81.xyz",
		},
		{
			name:     "rewrite home path",
			input:    "/home/bob/test",
			cchHash:  "",
			contains: "/Users/jack/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RewritePromptText(tt.input, config, tt.cchHash)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected result to contain %q, got %q", tt.contains, result)
			}
		})
	}
}

func TestRewriteSystemReminders(t *testing.T) {
	config := &PromptEnvConfig{
		Platform:   "darwin",
		Shell:      "zsh",
		OSVersion:  "Darwin 24.4.0",
		WorkingDir: "/Users/jack/projects",
	}

	input := `User message before.
<system-reminder>
Platform: linux
Shell: bash
OS Version: Linux 6.5.0
Working directory: /home/bob/project
</system-reminder>
User message after.`

	result := RewriteSystemReminders(input, config)

	// Should rewrite content inside system-reminder
	if !strings.Contains(result, "Platform: darwin") {
		t.Error("Should rewrite Platform in system-reminder")
	}
	if !strings.Contains(result, "Shell: zsh") {
		t.Error("Should rewrite Shell in system-reminder")
	}

	// Should preserve user message
	if !strings.Contains(result, "User message before.") {
		t.Error("Should preserve user message before")
	}
	if !strings.Contains(result, "User message after.") {
		t.Error("Should preserve user message after")
	}

	// Should preserve tags
	if !strings.Contains(result, "<system-reminder>") {
		t.Error("Should preserve system-reminder opening tag")
	}
	if !strings.Contains(result, "</system-reminder>") {
		t.Error("Should preserve system-reminder closing tag")
	}
}