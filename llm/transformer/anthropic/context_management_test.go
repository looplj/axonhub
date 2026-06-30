package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// TestContextManagementRoundTrip covers F21: the Anthropic top-level
// context_management (context-compression strategy, edits[]) must survive
// inbound (MessageRequest -> llm.Request) and outbound (llm.Request ->
// MessageRequest) on non-pass-through links. canonical llm.Request has no
// equivalent, so it is carried through TransformerMetadata as opaque
// json.RawMessage (mirrors cache_control / top_k passthrough). The edits
// schema is versioned and discriminated, so the proxy round-trips it raw
// rather than modeling it, like Caller / tool_result json.RawMessage.
func TestContextManagementRoundTrip(t *testing.T) {
	cm := json.RawMessage(`{"edits":[{"type":"clear_tool_uses_20250919","trigger":{"type":"input_tokens","threshold":10000}}]}`)

	t.Run("inbound preserves context_management in TransformerMetadata", func(t *testing.T) {
		req := &MessageRequest{
			Model:             "claude-3-sonnet-20240229",
			MaxTokens:         1024,
			ContextManagement: cm,
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hi")}},
			},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		v, ok := chatReq.TransformerMetadata[TransformerMetadataKeyContextManagement].(json.RawMessage)
		require.True(t, ok)
		require.JSONEq(t, string(cm), string(v))
	})

	t.Run("inbound nil context_management leaves no metadata", func(t *testing.T) {
		req := &MessageRequest{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: 1024,
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hi")}},
			},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		_, present := chatReq.TransformerMetadata[TransformerMetadataKeyContextManagement]
		require.False(t, present)
	})

	t.Run("outbound restores context_management from TransformerMetadata", func(t *testing.T) {
		transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(1024)),
			TransformerMetadata: map[string]any{
				TransformerMetadataKeyContextManagement: cm,
			},
			Messages: []llm.Message{{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hi")},
			}},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)
		require.NotNil(t, result)

		var anthropicReq MessageRequest
		require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
		require.NotEmpty(t, anthropicReq.ContextManagement)
		require.JSONEq(t, string(cm), string(anthropicReq.ContextManagement))
	})

	t.Run("outbound omits context_management when metadata absent", func(t *testing.T) {
		transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(1024)),
			Messages: []llm.Message{{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hi")},
			}},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)
		require.NotNil(t, result)

		var anthropicReq MessageRequest
		require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
		require.Empty(t, anthropicReq.ContextManagement)
	})
}
