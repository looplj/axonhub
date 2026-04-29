package responses

import (
	"os"
	"testing"
)

func TestSanitizeSpawnAgentArgs(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		arguments   string
		envEnabled  bool
		want        string
	}{
		{
			name:       "message only - unchanged",
			toolName:   "spawn_agent",
			arguments:  `{"message":"task"}`,
			envEnabled: true,
			want:       `{"message":"task"}`,
		},
		{
			name:       "items with content - unchanged",
			toolName:   "spawn_agent",
			arguments:  `{"items":[{"message":"task1"}]}`,
			envEnabled: true,
			want:       `{"items":[{"message":"task1"}]}`,
		},
		{
			name:       "message with empty items - items removed",
			toolName:   "spawn_agent",
			arguments:  `{"message":"task","items":[]}`,
			envEnabled: true,
			want:       `{"message":"task"}`,
		},
		{
			name:       "message with non-empty items - unchanged",
			toolName:   "spawn_agent",
			arguments:  `{"message":"task","items":[{"message":"task1"}]}`,
			envEnabled: true,
			want:       `{"message":"task","items":[{"message":"task1"}]}`,
		},
		{
			name:       "items empty without message - unchanged",
			toolName:   "spawn_agent",
			arguments:  `{"items":[]}`,
			envEnabled: true,
			want:       `{"items":[]}`,
		},
		{
			name:       "invalid json - unchanged",
			toolName:   "spawn_agent",
			arguments:  `{not valid json}`,
			envEnabled: true,
			want:       `{not valid json}`,
		},
		{
			name:       "toolName not spawn_agent - unchanged",
			toolName:   "other_tool",
			arguments:  `{"message":"task","items":[]}`,
			envEnabled: true,
			want:       `{"message":"task","items":[]}`,
		},
		{
			name:       "env not set - unchanged",
			toolName:   "spawn_agent",
			arguments:  `{"message":"task","items":[]}`,
			envEnabled: false,
			want:       `{"message":"task","items":[]}`,
		},
		{
			name:       "message empty string - unchanged",
			toolName:   "spawn_agent",
			arguments:  `{"message":"","items":[]}`,
			envEnabled: true,
			want:       `{"message":"","items":[]}`,
		},
		{
			name:       "items is not array - unchanged",
			toolName:   "spawn_agent",
			arguments:  `{"message":"task","items":"not an array"}`,
			envEnabled: true,
			want:       `{"message":"task","items":"not an array"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envEnabled {
				os.Setenv("AXONHUB_SANITIZE_SPAWN_AGENT_ARGS", "1")
			} else {
				os.Unsetenv("AXONHUB_SANITIZE_SPAWN_AGENT_ARGS")
			}
			t.Cleanup(func() {
				os.Unsetenv("AXONHUB_SANITIZE_SPAWN_AGENT_ARGS")
			})

			got := maybeSanitizeSpawnAgentArgs(tt.toolName, tt.arguments)
			if got != tt.want {
				t.Errorf("maybeSanitizeSpawnAgentArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeSpawnAgentArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		want      string
	}{
		{
			name:      "message only - unchanged",
			arguments: `{"message":"task"}`,
			want:      `{"message":"task"}`,
		},
		{
			name:      "message with empty items - items removed",
			arguments: `{"message":"task","items":[]}`,
			want:      `{"message":"task"}`,
		},
		{
			name:      "message with non-empty items - unchanged",
			arguments: `{"message":"task","items":[{"type":"text"}]}`,
			want:      `{"message":"task","items":[{"type":"text"}]}`,
		},
		{
			name:      "no message field - unchanged",
			arguments: `{"items":[],"other":"value"}`,
			want:      `{"items":[],"other":"value"}`,
		},
		{
			name:      "empty message - unchanged",
			arguments: `{"message":"","items":[]}`,
			want:      `{"message":"","items":[]}`,
		},
		{
			name:      "invalid json - unchanged",
			arguments: `{invalid}`,
			want:      `{invalid}`,
		},
		{
			name:      "items is object not array - unchanged",
			arguments: `{"message":"task","items":{}}`,
			want:      `{"message":"task","items":{}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSpawnAgentArguments(tt.arguments)
			if got != tt.want {
				t.Errorf("sanitizeSpawnAgentArguments() = %v, want %v", got, tt.want)
			}
		})
	}
}
