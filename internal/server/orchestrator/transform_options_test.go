package orchestrator

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestApplyTransformOptions_ReplaceDeveloperRoleWithSystem(t *testing.T) {
	developerContent := "dev"
	userContent := "hi"
	req := &llm.Request{
		Model: "test-model",
		Messages: []llm.Message{
			{Role: "developer", Content: llm.MessageContent{Content: &developerContent}},
			{Role: "user", Content: llm.MessageContent{Content: &userContent}},
		},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ReplaceDeveloperRoleWithSystem: true,
		},
	}

	result := applyTransformOptions(req, settings)

	require.NotSame(t, req, result)
	require.Equal(t, "system", result.Messages[0].Role)
	require.Equal(t, "user", result.Messages[1].Role)
}

func TestApplyTransformOptions_KeepDeveloperRoleWhenDisabled(t *testing.T) {
	developerContent := "dev"
	req := &llm.Request{
		Model: "test-model",
		Messages: []llm.Message{
			{Role: "developer", Content: llm.MessageContent{Content: &developerContent}},
		},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ReplaceDeveloperRoleWithSystem: false,
		},
	}

	result := applyTransformOptions(req, settings)

	require.Same(t, req, result)
	require.Equal(t, "developer", result.Messages[0].Role)
}

func TestApplyTransformOptions_NilSettings(t *testing.T) {
	req := &llm.Request{Model: "test-model"}

	result := applyTransformOptions(req, nil)

	require.Same(t, req, result)
}

func TestApplyTransformOptions_ForceArrayInstructions(t *testing.T) {
	req := &llm.Request{Model: "test-model"}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInstructions: true,
		},
	}

	result := applyTransformOptions(req, settings)

	require.NotSame(t, req, result)
	require.Equal(t, lo.ToPtr(true), result.TransformOptions.ArrayInstructions)
}

func TestApplyTransformOptions_ForceArrayInputs(t *testing.T) {
	req := &llm.Request{Model: "test-model"}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInputs: true,
		},
	}

	result := applyTransformOptions(req, settings)

	require.NotSame(t, req, result)
	require.Equal(t, lo.ToPtr(true), result.TransformOptions.ArrayInputs)
}

func TestApplyClaudeCodeOpenAIReasoningEffortMapping(t *testing.T) {
	settings := &objects.ChannelSettings{TransformOptions: objects.TransformOptions{
		ReasoningEffortMapping: []llm.ReasoningEffortMapping{
			{From: "xhigh", To: "max"},
			{From: "max", To: "high"},
		},
	}}

	tests := []struct {
		name             string
		format           llm.APIFormat
		claudeCodeClient bool
		originalEffort   string
		body             string
		path             string
		expectedEffort   string
		expectClone      bool
	}{
		{
			name:             "maps unmapped Chat Completions body",
			format:           llm.APIFormatOpenAIChatCompletion,
			claudeCodeClient: true,
			originalEffort:   "xhigh",
			body:             `{"model":"test","reasoning_effort":"xhigh"}`,
			path:             "reasoning_effort",
			expectedEffort:   "max",
			expectClone:      true,
		},
		{
			name:             "maps unmapped Responses body",
			format:           llm.APIFormatOpenAIResponse,
			claudeCodeClient: true,
			originalEffort:   "xhigh",
			body:             `{"model":"test","reasoning":{"effort":"xhigh"}}`,
			path:             "reasoning.effort",
			expectedEffort:   "max",
			expectClone:      true,
		},
		{
			name:             "maps unmapped compact Responses body",
			format:           llm.APIFormatOpenAIResponseCompact,
			claudeCodeClient: true,
			originalEffort:   "xhigh",
			body:             `{"model":"test","reasoning":{"effort":"xhigh"}}`,
			path:             "reasoning.effort",
			expectedEffort:   "max",
			expectClone:      true,
		},
		{
			name:             "does not apply a second mapping",
			format:           llm.APIFormatOpenAIChatCompletion,
			claudeCodeClient: true,
			originalEffort:   "xhigh",
			body:             `{"model":"test","reasoning_effort":"max"}`,
			path:             "reasoning_effort",
			expectedEffort:   "max",
		},
		{
			name:             "does not restore provider-cleared effort",
			format:           llm.APIFormatOpenAIChatCompletion,
			claudeCodeClient: true,
			originalEffort:   "xhigh",
			body:             `{"model":"test"}`,
			path:             "reasoning_effort",
		},
		{
			name:           "non Claude Code client keeps original behavior",
			format:         llm.APIFormatOpenAIChatCompletion,
			originalEffort: "xhigh",
			body:           `{"model":"test","reasoning_effort":"xhigh"}`,
			path:           "reasoning_effort",
			expectedEffort: "xhigh",
		},
		{
			name:             "Anthropic outbound keeps original behavior",
			format:           llm.APIFormatAnthropicMessage,
			claudeCodeClient: true,
			originalEffort:   "xhigh",
			body:             `{"model":"test","output_config":{"effort":"max"}}`,
			path:             "output_config.effort",
			expectedEffort:   "max",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpRequest := &httpclient.Request{Body: []byte(tt.body)}

			result, err := applyClaudeCodeOpenAIReasoningEffortMapping(
				httpRequest,
				settings,
				tt.format,
				tt.claudeCodeClient,
				tt.originalEffort,
			)
			require.NoError(t, err)

			if tt.expectClone {
				require.NotSame(t, httpRequest, result)
			} else {
				require.Same(t, httpRequest, result)
			}
			require.Equal(t, tt.expectedEffort, gjson.GetBytes(result.Body, tt.path).String())
		})
	}

	t.Run("empty mapping keeps original behavior", func(t *testing.T) {
		httpRequest := &httpclient.Request{Body: []byte(`{"model":"test","reasoning_effort":"xhigh"}`)}
		result, err := applyClaudeCodeOpenAIReasoningEffortMapping(
			httpRequest,
			&objects.ChannelSettings{},
			llm.APIFormatOpenAIChatCompletion,
			true,
			"xhigh",
		)
		require.NoError(t, err)
		require.Same(t, httpRequest, result)
		require.Equal(t, "xhigh", gjson.GetBytes(result.Body, "reasoning_effort").String())
	})

	t.Run("mapped effort updates the stored JSON body", func(t *testing.T) {
		body := []byte(`{"model":"test","reasoning_effort":"xhigh"}`)
		httpRequest := &httpclient.Request{Body: body, JSONBody: body}

		result, err := applyClaudeCodeOpenAIReasoningEffortMapping(
			httpRequest,
			settings,
			llm.APIFormatOpenAIChatCompletion,
			true,
			"xhigh",
		)
		require.NoError(t, err)
		require.Equal(t, "max", gjson.GetBytes(result.Body, "reasoning_effort").String())
		require.Equal(t, "max", gjson.GetBytes(result.JSONBody, "reasoning_effort").String())
	})
}

func TestReplaceDeveloperRoleWithSystem(t *testing.T) {
	tests := []struct {
		name     string
		messages []llm.Message
		expected []string
	}{
		{
			name:     "empty messages",
			messages: []llm.Message{},
			expected: []string{},
		},
		{
			name: "developer role replaced",
			messages: []llm.Message{
				{Role: "developer"},
				{Role: "user"},
			},
			expected: []string{"system", "user"},
		},
		{
			name: "Developer case insensitive",
			messages: []llm.Message{
				{Role: "Developer"},
				{Role: "DEVELOPER"},
			},
			expected: []string{"system", "system"},
		},
		{
			name: "no developer role",
			messages: []llm.Message{
				{Role: "system"},
				{Role: "user"},
			},
			expected: []string{"system", "user"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceDeveloperRoleWithSystem(tt.messages)
			for i, role := range tt.expected {
				require.Equal(t, role, result[i].Role)
			}
		})
	}
}
