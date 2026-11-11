package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{name: "exact match", pattern: "claude-sonnet-4.5", value: "claude-sonnet-4.5", want: true},
		{name: "exact mismatch", pattern: "claude-sonnet-4.5", value: "claude-opus-4.5", want: false},
		{name: "star stays within a segment", pattern: "anthropic/claude-*", value: "anthropic/claude-sonnet-4.5", want: true},
		{name: "star does not cross a slash", pattern: "anthropic/*", value: "anthropic/openai/claude", want: false},
		{name: "bare star does not cross a slash", pattern: "*", value: "anthropic/claude", want: false},
		{name: "double star crosses segments", pattern: "**/claude-*", value: "anthropic/openai/claude-sonnet", want: true},
		{name: "double star matches zero segments", pattern: "**/claude-*", value: "claude-sonnet", want: true},
		{name: "double star alone matches everything", pattern: "**", value: "a/b/c", want: true},
		{name: "double star in the middle matches zero segments", pattern: "a/**/b", value: "a/b", want: true},
		{name: "question mark matches one character", pattern: "model-?", value: "model-1", want: true},
		{name: "question mark matches only one character", pattern: "model-?", value: "model-12", want: false},
		{name: "character class matches", pattern: "model-[ab]", value: "model-b", want: true},
		{name: "alternation matches", pattern: "model-{a,b}", value: "model-b", want: true},
		{name: "case sensitive", pattern: "Claude-*", value: "claude-sonnet", want: false},
		{name: "empty value", pattern: "*", value: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := globMatch(tt.pattern, tt.value)
			require.NoError(t, err)
			require.Equal(t, tt.want, matched)
		})
	}
}

func TestGlobMatchInvalidPattern(t *testing.T) {
	matched, err := globMatch("model-[", "model-a")
	require.Error(t, err)
	require.False(t, matched)
}
