package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// TestPatchPassThroughPromptProtection verifies provider-native text paths and
// role scopes for all direct chat APIs that support request-body pass-through.
func TestPatchPassThroughPromptProtection(t *testing.T) {
	rules := []*ent.PromptProtectionRule{
		{
			Name:    "mask-user-secret",
			Pattern: "secret-[0-9]+",
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[MASKED]",
				Scopes:      []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		},
	}

	tests := []struct {
		name      string
		apiFormat llm.APIFormat
		body      string
		masked    []string
		unchanged []string
	}{
		{
			name:      "OpenAI Responses",
			apiFormat: llm.APIFormatOpenAIResponse,
			body:      `{"instructions":"secret-001","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"secret-002"}]},{"type":"function_call_output","output":"secret-003"}]}`,
			masked:    []string{"input.0.content.0.text"},
			unchanged: []string{"instructions", "input.1.output"},
		},
		{
			name:      "Anthropic Messages",
			apiFormat: llm.APIFormatAnthropicMessage,
			body:      `{"system":"secret-001","messages":[{"role":"user","content":"secret-002"},{"role":"assistant","content":[{"type":"text","text":"secret-003"}]}]}`,
			masked:    []string{"messages.0.content"},
			unchanged: []string{"system", "messages.1.content.0.text"},
		},
		{
			name:      "Gemini GenerateContent",
			apiFormat: llm.APIFormatGeminiContents,
			body:      `{"systemInstruction":{"parts":[{"text":"secret-001"}]},"contents":[{"role":"user","parts":[{"text":"secret-002"}]},{"role":"model","parts":[{"text":"secret-003"}]}]}`,
			masked:    []string{"contents.0.parts.0.text"},
			unchanged: []string{"systemInstruction.parts.0.text", "contents.1.parts.0.text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patched, err := patchPassThroughPromptProtection([]byte(tt.body), tt.apiFormat, rules)
			require.NoError(t, err)

			for _, path := range tt.masked {
				assert.Equal(t, "[MASKED]", gjson.GetBytes(patched, path).String(), path)
			}

			for _, path := range tt.unchanged {
				assert.Contains(t, gjson.GetBytes(patched, path).String(), "secret-", path)
			}
		})
	}
}

// TestPatchPassThroughPromptProtectionRejectsUnsupportedFormat verifies the
// fail-safe path: protected content is never replayed unmodified for unknown layouts.
func TestPatchPassThroughPromptProtectionRejectsUnsupportedFormat(t *testing.T) {
	rules := []*ent.PromptProtectionRule{{
		Name:    "mask-secret",
		Pattern: "secret",
		Settings: &objects.PromptProtectionSettings{
			Action:      objects.PromptProtectionActionMask,
			Replacement: "[MASKED]",
		},
	}}

	_, err := patchPassThroughPromptProtection([]byte(`{"messages":[{"role":"user","content":"secret"}]}`), llm.APIFormatAiSDKText, rules)
	require.Error(t, err)
	assert.ErrorContains(t, err, "does not support raw prompt protection patches")
}
