package responses

import "testing"

func TestSanitizeSpawnAgentArguments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"invalid json", "not json", "not json"},
		{"no message field", `{"items":[]}`, `{"items":[]}`},
		{"empty message", `{"message":"","items":[]}`, `{"message":"","items":[]}`},
		{"non-empty items", `{"message":"hello","items":["a"]}`, `{"message":"hello","items":["a"]}`},
		{"message with empty items", `{"message":"hello","items":[]}`, `{"message":"hello"}`},
		{"preserves other fields", `{"message":"hi","items":[],"other":42}`, `{"message":"hi","other":42}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSpawnAgentArguments(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeSpawnAgentArguments(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
