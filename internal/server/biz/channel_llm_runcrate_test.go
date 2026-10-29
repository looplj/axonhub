//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to each scenario.
package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestRuncrateOutboundRouting locks in the Runcrate outbound contract:
//   - runcrate            -> OpenAI chat completions and embeddings at baseURL + "/chat/completions" and "/embeddings"
//   - runcrate_anthropic  -> Anthropic messages at baseURL + "/v1/messages"
func TestRuncrateOutboundRouting(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_llm_runcrate?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	svc := NewChannelServiceForTest(client)
	ctx := context.Background()

	newRequest := func(model string) *llm.Request {
		return &llm.Request{
			Model: model,
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: ptrTo("Hello")}},
			},
		}
	}

	t.Run("runcrate serves chat completions and embeddings", func(t *testing.T) {
		c := &ent.Channel{
			ID:          1,
			Name:        "runcrate",
			Type:        channel.TypeRuncrate,
			BaseURL:     "https://api.runcrate.ai/v1",
			Credentials: objects.ChannelCredentials{APIKey: "rc_test"},
		}

		ch, err := svc.buildChannelWithOutbounds(c)
		require.NoError(t, err)

		outbound, err := BuildOutboundByAPIFormat(ch, llm.APIFormatOpenAIChatCompletion.String())
		require.NoError(t, err)

		httpReq, err := outbound.TransformRequest(ctx, newRequest("deepseek-ai/DeepSeek-V4.1-Flash"))
		require.NoError(t, err)
		require.Equal(t, "https://api.runcrate.ai/v1/chat/completions", httpReq.URL)
		require.Equal(t, "rc_test", httpReq.Auth.APIKey)

		embeddingOutbound, err := BuildOutboundByAPIFormat(ch, llm.APIFormatOpenAIEmbedding.String())
		require.NoError(t, err, "runcrate should expose embeddings")

		embeddingReq, err := embeddingOutbound.TransformRequest(ctx, &llm.Request{
			Model:       "Qwen/Qwen3-Embedding-8B",
			RequestType: llm.RequestTypeEmbedding,
			APIFormat:   llm.APIFormatOpenAIEmbedding,
			Embedding: &llm.EmbeddingRequest{
				Input: llm.EmbeddingInput{String: "hello"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "https://api.runcrate.ai/v1/embeddings", embeddingReq.URL)
		require.Equal(t, "rc_test", embeddingReq.Auth.APIKey)
		require.Equal(t, "Qwen/Qwen3-Embedding-8B", gjson.GetBytes(embeddingReq.Body, "model").String())
		require.Equal(t, "hello", gjson.GetBytes(embeddingReq.Body, "input").String())
	})

	t.Run("runcrate_anthropic uses the Anthropic messages route", func(t *testing.T) {
		c := &ent.Channel{
			ID:          2,
			Name:        "runcrate-anthropic",
			Type:        channel.TypeRuncrateAnthropic,
			BaseURL:     "https://api.runcrate.ai",
			Credentials: objects.ChannelCredentials{APIKey: "rc_test"},
		}

		ch, err := svc.buildChannelWithOutbounds(c)
		require.NoError(t, err)

		outbound, err := BuildOutboundByAPIFormat(ch, llm.APIFormatAnthropicMessage.String())
		require.NoError(t, err)

		httpReq, err := outbound.TransformRequest(ctx, newRequest("deepseek-ai/DeepSeek-V4.1-Flash"))
		require.NoError(t, err)
		require.Equal(t, "https://api.runcrate.ai/v1/messages", httpReq.URL)
		require.Equal(t, "rc_test", httpReq.Auth.APIKey)
		require.Equal(t, httpclient.AuthTypeAPIKey, httpReq.Auth.Type)
		require.Equal(t, "X-API-Key", httpReq.Auth.HeaderKey)
		require.Equal(t, "2023-06-01", httpReq.Headers.Get("Anthropic-Version"))
	})
}
