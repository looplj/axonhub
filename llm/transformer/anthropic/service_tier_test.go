package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// TestServiceTierRoundTrip covers #7: Anthropic request service_tier must
// survive both inbound (MessageRequest -> llm.Request) and outbound
// (llm.Request -> MessageRequest) conversion on non-pass-through links.
func TestServiceTierRoundTrip(t *testing.T) {
	t.Run("inbound preserves service_tier", func(t *testing.T) {
		req := &MessageRequest{
			Model:       "claude-3-sonnet-20240229",
			MaxTokens:   1024,
			ServiceTier: "priority",
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hi")}},
			},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.NotNil(t, chatReq.ServiceTier)
		require.Equal(t, "priority", *chatReq.ServiceTier)
	})

	t.Run("inbound empty service_tier stays nil", func(t *testing.T) {
		req := &MessageRequest{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: 1024,
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hi")}},
			},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Nil(t, chatReq.ServiceTier)
	})

	t.Run("outbound preserves service_tier", func(t *testing.T) {
		transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		chatReq := &llm.Request{
			Model:       "claude-3-sonnet-20240229",
			MaxTokens:   lo.ToPtr(int64(1024)),
			ServiceTier: lo.ToPtr("priority"),
			Messages: []llm.Message{{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hi"),
				},
			}},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)
		require.NotNil(t, result)

		var anthropicReq MessageRequest
		require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
		require.Equal(t, "priority", anthropicReq.ServiceTier)
	})

	t.Run("outbound omits service_tier when nil", func(t *testing.T) {
		transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(1024)),
			Messages: []llm.Message{{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hi"),
				},
			}},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)
		require.NotNil(t, result)

		var anthropicReq MessageRequest
		require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
		require.Empty(t, anthropicReq.ServiceTier)
	})
}
