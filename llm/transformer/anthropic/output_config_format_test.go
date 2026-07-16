package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// TestOutputConfig_FormatTaskBudgetRoundTrip covers #6: the Anthropic
// output_config sub-items format (json_schema) and task_budget (tokens) must
// survive inbound (MessageRequest -> llm.Request) and outbound
// (llm.Request -> MessageRequest). The author's OutputConfig previously
// modeled only Effort, so format/task_budget were silently dropped. They are
// carried through TransformerMetadata as opaque json.RawMessage (the gateway
// does not interpret them); effort remains typed since the gateway maps it to
// reasoning_effort. effort=='max'->'xhigh' is a deliberate cross-format
// adaptation (the raw effort is stashed and restored under supportsOutputConfig)
// and is intentionally not changed here.
func TestOutputConfig_FormatTaskBudgetRoundTrip(t *testing.T) {
	format := json.RawMessage(`{"type":"json_schema","schema":{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}}`)
	taskBudget := json.RawMessage(`{"type":"tokens","total":400000}`)

	t.Run("inbound stashes full OutputConfig with format/task_budget", func(t *testing.T) {
		req := &MessageRequest{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: 4096,
			OutputConfig: &OutputConfig{
				Effort:     "high",
				Format:     format,
				TaskBudget: taskBudget,
			},
			Messages: []MessageParam{{Role: "user", Content: MessageContent{Content: lo.ToPtr("hi")}}},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		// effort still captured + mapped (unchanged behavior)
		require.Equal(t, "high", chatReq.TransformerMetadata[TransformerMetadataKeyOutputConfigEffort])
		require.Equal(t, "high", chatReq.ReasoningEffort)
		// full OutputConfig stashed (carries format/task_budget)
		oc, ok := chatReq.TransformerMetadata[TransformerMetadataKeyOutputConfig].(*OutputConfig)
		require.True(t, ok)
		require.NotNil(t, oc)
		require.Equal(t, "high", oc.Effort)
		require.JSONEq(t, string(format), string(oc.Format))
		require.JSONEq(t, string(taskBudget), string(oc.TaskBudget))
	})

	t.Run("inbound format-only (no effort) still stashes full OutputConfig", func(t *testing.T) {
		req := &MessageRequest{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: 4096,
			OutputConfig: &OutputConfig{
				Format: format,
			},
			Messages: []MessageParam{{Role: "user", Content: MessageContent{Content: lo.ToPtr("hi")}}},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		// no effort key (effort empty -> not mapped)
		_, hasEffort := chatReq.TransformerMetadata[TransformerMetadataKeyOutputConfigEffort]
		require.False(t, hasEffort)
		// full OutputConfig still stashed (format preserved)
		oc, ok := chatReq.TransformerMetadata[TransformerMetadataKeyOutputConfig].(*OutputConfig)
		require.True(t, ok)
		require.NotNil(t, oc)
		require.JSONEq(t, string(format), string(oc.Format))
	})

	t.Run("outbound restores full OutputConfig with format/task_budget", func(t *testing.T) {
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(4096)),
			TransformerMetadata: map[string]any{
				TransformerMetadataKeyOutputConfig: &OutputConfig{
					Effort:     "high",
					Format:     format,
					TaskBudget: taskBudget,
				},
			},
			Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}}},
		}

		anthropicReq, err := convertToAnthropicRequest(chatReq)

		require.NoError(t, err)
		require.NotNil(t, anthropicReq.OutputConfig)
		require.Equal(t, "high", anthropicReq.OutputConfig.Effort)
		require.JSONEq(t, string(format), string(anthropicReq.OutputConfig.Format))
		require.JSONEq(t, string(taskBudget), string(anthropicReq.OutputConfig.TaskBudget))
	})

	t.Run("outbound restores format-only OutputConfig (no effort)", func(t *testing.T) {
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(4096)),
			TransformerMetadata: map[string]any{
				TransformerMetadataKeyOutputConfig: &OutputConfig{
					Format: format,
				},
			},
			Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}}},
		}

		anthropicReq, err := convertToAnthropicRequest(chatReq)

		require.NoError(t, err)
		require.NotNil(t, anthropicReq.OutputConfig)
		require.JSONEq(t, string(format), string(anthropicReq.OutputConfig.Format))
		require.Empty(t, anthropicReq.OutputConfig.Effort)
	})

	t.Run("outbound falls back to effort-only when full stash absent (backward compat)", func(t *testing.T) {
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(4096)),
			TransformerMetadata: map[string]any{
				TransformerMetadataKeyOutputConfigEffort: "max",
			},
			Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}}},
		}

		anthropicReq, err := convertToAnthropicRequest(chatReq)

		require.NoError(t, err)
		require.NotNil(t, anthropicReq.OutputConfig)
		require.Equal(t, "max", anthropicReq.OutputConfig.Effort)
	})
}
