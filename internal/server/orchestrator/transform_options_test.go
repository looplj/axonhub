//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to each scenario.
package orchestrator

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
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

func TestApplyReasoningEffortMapping(t *testing.T) {
	settings := &objects.ChannelSettings{TransformOptions: objects.TransformOptions{
		ReasoningEffortMapping: []llm.ReasoningEffortMapping{
			{From: "xhigh", To: "max"},
			{From: "max", To: "high"},
		},
	}}

	t.Run("maps the unified request effort", func(t *testing.T) {
		req := &llm.Request{Model: "test-model", ReasoningEffort: "xhigh"}

		result := applyReasoningEffortMapping(req, settings)

		require.NotSame(t, req, result)
		require.Equal(t, "max", result.ReasoningEffort)
	})

	t.Run("first matching entry wins", func(t *testing.T) {
		req := &llm.Request{Model: "test-model", ReasoningEffort: "max"}

		result := applyReasoningEffortMapping(req, settings)

		require.Equal(t, "high", result.ReasoningEffort)
	})

	t.Run("unmapped effort keeps the same request", func(t *testing.T) {
		req := &llm.Request{Model: "test-model", ReasoningEffort: "low"}

		result := applyReasoningEffortMapping(req, settings)

		require.Same(t, req, result)
	})

	t.Run("empty effort keeps the same request", func(t *testing.T) {
		req := &llm.Request{Model: "test-model"}

		result := applyReasoningEffortMapping(req, settings)

		require.Same(t, req, result)
	})

	t.Run("empty mapping keeps the original request", func(t *testing.T) {
		req := &llm.Request{Model: "test-model", ReasoningEffort: "xhigh"}

		result := applyReasoningEffortMapping(req, &objects.ChannelSettings{})

		require.Same(t, req, result)
	})

	t.Run("nil settings keeps the original request", func(t *testing.T) {
		req := &llm.Request{Model: "test-model", ReasoningEffort: "xhigh"}

		result := applyReasoningEffortMapping(req, nil)

		require.Same(t, req, result)
	})

	t.Run("syncs the anthropic output_config effort metadata", func(t *testing.T) {
		req := &llm.Request{
			Model:           "test-model",
			ReasoningEffort: "max",
			TransformerMetadata: map[string]any{
				anthropic.TransformerMetadataKeyOutputConfigEffort: "max",
				anthropic.TransformerMetadataKeyThinkingType:       "adaptive",
			},
		}

		result := applyReasoningEffortMapping(req, settings)

		require.Equal(t, "high", result.ReasoningEffort)
		// The outbound transformer rebuilds output_config.effort from this marker;
		// keeping the original "max" would bypass the mapping entirely.
		require.Equal(t, "high", result.TransformerMetadata[anthropic.TransformerMetadataKeyOutputConfigEffort])
		require.Equal(t, "adaptive", result.TransformerMetadata[anthropic.TransformerMetadataKeyThinkingType])
	})

	t.Run("mapping does not mutate the original request metadata", func(t *testing.T) {
		metadata := map[string]any{
			anthropic.TransformerMetadataKeyOutputConfigEffort: "max",
		}
		req := &llm.Request{
			Model:               "test-model",
			ReasoningEffort:     "max",
			TransformerMetadata: metadata,
		}

		result := applyReasoningEffortMapping(req, settings)

		require.Equal(t, "high", result.ReasoningEffort)
		require.Equal(t, "high", result.TransformerMetadata[anthropic.TransformerMetadataKeyOutputConfigEffort])
		// The shallow request copy shares the map with the original request; the
		// sync must clone before writing.
		require.Equal(t, "max", req.TransformerMetadata[anthropic.TransformerMetadataKeyOutputConfigEffort])
	})

	t.Run("no anthropic metadata marker keeps metadata untouched", func(t *testing.T) {
		req := &llm.Request{Model: "test-model", ReasoningEffort: "xhigh"}

		result := applyReasoningEffortMapping(req, settings)

		require.Equal(t, "max", result.ReasoningEffort)
		require.Nil(t, result.TransformerMetadata)
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
