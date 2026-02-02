package orchestrator

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
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
