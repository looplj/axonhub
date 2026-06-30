package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// TestTopKRoundTrip covers #8: Anthropic request top_k must survive both
// inbound (MessageRequest -> llm.Request) and outbound (llm.Request ->
// MessageRequest) conversion on non-pass-through links. canonical llm.Request
// has no TopK field, so top_k is carried through TransformerMetadata.
func TestTopKRoundTrip(t *testing.T) {
	t.Run("inbound preserves top_k in TransformerMetadata", func(t *testing.T) {
		req := &MessageRequest{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: 1024,
			TopK:      lo.ToPtr(int64(40)),
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hi")}},
			},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		v, ok := chatReq.TransformerMetadata[shared.TransformerMetadataKeyTopK].(*int64)
		require.True(t, ok)
		require.NotNil(t, v)
		require.Equal(t, int64(40), *v)
	})

	t.Run("inbound nil top_k leaves no metadata", func(t *testing.T) {
		req := &MessageRequest{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: 1024,
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hi")}},
			},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		_, present := chatReq.TransformerMetadata[shared.TransformerMetadataKeyTopK]
		require.False(t, present)
	})

	t.Run("outbound restores top_k from TransformerMetadata", func(t *testing.T) {
		transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(1024)),
			TransformerMetadata: map[string]any{
				shared.TransformerMetadataKeyTopK: lo.ToPtr(int64(40)),
			},
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
		require.NotNil(t, anthropicReq.TopK)
		require.Equal(t, int64(40), *anthropicReq.TopK)
	})

	t.Run("outbound omits top_k when metadata absent", func(t *testing.T) {
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
		require.Nil(t, anthropicReq.TopK)
	})
}

// C3/D23: chat top_k must reach anthropic provider (cross-format).
func TestTopK_ChatToAnthropic(t *testing.T) {
	inbound := openai.NewInboundTransformer()
	req := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"top_k":40}`),
	}

	llmReq, err := inbound.TransformRequest(t.Context(), req)
	require.NoError(t, err)

	transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	result, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var anthropicReq MessageRequest
	require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
	require.NotNil(t, anthropicReq.TopK, "C3: chat top_k must reach anthropic provider")
	require.Equal(t, int64(40), *anthropicReq.TopK)
}
