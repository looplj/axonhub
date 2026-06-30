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
)

// TestUserBridge covers #13: the user identity must cross the anthropic boundary.
// chat/responses use a top-level `user` (canonical.User *string); anthropic has no
// top-level user and carries identity via metadata.user_id. Without a bridge the two
// never copy into each other, so cross-format routing drops the user.
func TestUserBridge(t *testing.T) {
	t.Run("inbound bridges metadata.user_id to canonical.User", func(t *testing.T) {
		req := &MessageRequest{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: 1024,
			Metadata:  &AnthropicMetadata{UserID: "u-123"},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hi")}},
			},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.NotNil(t, chatReq.User, "#13: anthropic metadata.user_id must bridge to canonical.User")
		require.Equal(t, "u-123", *chatReq.User)
		require.Equal(t, "u-123", chatReq.Metadata["user_id"], "#13: anthropic-native metadata.user_id retained for round-trip")
	})

	t.Run("inbound nil metadata leaves no User", func(t *testing.T) {
		req := &MessageRequest{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: 1024,
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hi")}},
			},
		}

		chatReq, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.Nil(t, chatReq.User, "#13: no metadata.user_id must not set canonical.User")
		_, present := chatReq.Metadata["user_id"]
		require.False(t, present)
	})

	t.Run("outbound restores from canonical.User fallback", func(t *testing.T) {
		transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(1024)),
			User:      lo.ToPtr("u-456"),
			Messages: []llm.Message{{
				Role: "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hi")},
			}},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)

		var anthropicReq MessageRequest
		require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
		require.NotNil(t, anthropicReq.Metadata, "#13: canonical.User must fall back to anthropic metadata.user_id")
		require.Equal(t, "u-456", anthropicReq.Metadata.UserID)
	})

	t.Run("outbound prefers metadata.user_id over canonical.User", func(t *testing.T) {
		transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(1024)),
			User:      lo.ToPtr("cross"),
			Metadata:  map[string]string{"user_id": "native"},
			Messages: []llm.Message{{
				Role: "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hi")},
			}},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)

		var anthropicReq MessageRequest
		require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
		require.NotNil(t, anthropicReq.Metadata)
		require.Equal(t, "native", anthropicReq.Metadata.UserID, "#13: anthropic-native metadata.user_id must win over canonical.User")
	})

	t.Run("outbound omits metadata when no user", func(t *testing.T) {
		transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		chatReq := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(1024)),
			Messages: []llm.Message{{
				Role: "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hi")},
			}},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)

		var anthropicReq MessageRequest
		require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
		require.Nil(t, anthropicReq.Metadata, "#13: no user identity must yield no metadata")
	})
}

// #13: chat top-level `user` must reach the anthropic provider (cross-format bridge).
func TestUserBridge_ChatToAnthropic(t *testing.T) {
	inbound := openai.NewInboundTransformer()
	req := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"user":"u-789"}`),
	}

	llmReq, err := inbound.TransformRequest(t.Context(), req)
	require.NoError(t, err)

	transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	result, err := transformer.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var anthropicReq MessageRequest
	require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
	require.NotNil(t, anthropicReq.Metadata, "#13: chat user must bridge to anthropic metadata.user_id")
	require.Equal(t, "u-789", anthropicReq.Metadata.UserID)
}
