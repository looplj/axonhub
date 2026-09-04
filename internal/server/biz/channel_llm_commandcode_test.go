//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to each scenario.
package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestCommandCodeOutboundRouting locks in the Command Code outbound contract:
//   - commandcode            -> OpenAI chat completions at baseURL + "/chat/completions", Authorization: Bearer
//   - commandcode_anthropic  -> Anthropic messages at baseURL + "/messages", Authorization: Bearer
//   - ordinary anthropic direct channels keep X-API-Key
func TestCommandCodeOutboundRouting(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_llm_commandcode?mode=memory&_fk=0")
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

	base := "https://api.commandcode.ai/provider/v1"
	anthropicBase := "https://api.commandcode.ai/provider/v1"

	t.Run("commandcode chat uses /chat/completions with Bearer", func(t *testing.T) {
		c := &ent.Channel{
			ID:          1,
			Name:        "cc",
			Type:        channel.TypeCommandcode,
			BaseURL:     base,
			Credentials: objects.ChannelCredentials{APIKey: "sk-test"},
		}

		ch, err := svc.buildChannelWithOutbounds(c)
		require.NoError(t, err)

		outbound := ch.Outbounds[llm.APIFormatOpenAIChatCompletion.String()]
		require.NotNil(t, outbound)

		httpReq, err := outbound.TransformRequest(ctx, newRequest("gpt-5-codex"))
		require.NoError(t, err)
		require.Equal(t, "https://api.commandcode.ai/provider/v1/chat/completions", httpReq.URL)
		require.Equal(t, "sk-test", httpReq.Auth.APIKey)
		require.Equal(t, httpclient.AuthTypeBearer, httpReq.Auth.Type)
	})

	t.Run("commandcode chat accepts OpenAI-format models", func(t *testing.T) {
		c := &ent.Channel{
			ID:          2,
			Name:        "cc2",
			Type:        channel.TypeCommandcode,
			BaseURL:     base,
			Credentials: objects.ChannelCredentials{APIKey: "sk-test"},
		}
		ch, err := svc.buildChannelWithOutbounds(c)
		require.NoError(t, err)
		outbound := ch.Outbounds[llm.APIFormatOpenAIChatCompletion.String()]
		require.NotNil(t, outbound)

		for _, model := range []string{"gpt-5-codex", "deepseek-v3"} {
			httpReq, err := outbound.TransformRequest(ctx, newRequest(model))
			require.NoError(t, err)
			require.Equal(t, "https://api.commandcode.ai/provider/v1/chat/completions", httpReq.URL)
		}
	})

	t.Run("commandcode_anthropic uses /messages with Bearer", func(t *testing.T) {
		c := &ent.Channel{
			ID:          3,
			Name:        "cca",
			Type:        channel.TypeCommandcodeAnthropic,
			BaseURL:     anthropicBase,
			Credentials: objects.ChannelCredentials{APIKey: "sk-test"},
		}

		ch, err := svc.buildChannelWithOutbounds(c)
		require.NoError(t, err)

		outbound := ch.Outbounds[llm.APIFormatAnthropicMessage.String()]
		require.NotNil(t, outbound)

		httpReq, err := outbound.TransformRequest(ctx, newRequest("claude-sonnet-4-5"))
		require.NoError(t, err)
		require.Equal(t, "https://api.commandcode.ai/provider/v1/messages", httpReq.URL)
		require.Equal(t, "sk-test", httpReq.Auth.APIKey)
		require.Equal(t, httpclient.AuthTypeBearer, httpReq.Auth.Type)
	})

	t.Run("commandcode_anthropic endpoint is model independent", func(t *testing.T) {
		c := &ent.Channel{
			ID:          4,
			Name:        "cca2",
			Type:        channel.TypeCommandcodeAnthropic,
			BaseURL:     anthropicBase,
			Credentials: objects.ChannelCredentials{APIKey: "sk-test"},
		}
		ch, err := svc.buildChannelWithOutbounds(c)
		require.NoError(t, err)
		outbound := ch.Outbounds[llm.APIFormatAnthropicMessage.String()]
		require.NotNil(t, outbound)

		for _, model := range []string{"claude-sonnet-4-5", "claude-opus-4-1"} {
			httpReq, err := outbound.TransformRequest(ctx, newRequest(model))
			require.NoError(t, err)
			require.Equal(t, "https://api.commandcode.ai/provider/v1/messages", httpReq.URL)
		}
	})

	t.Run("plain anthropic direct keeps X-API-Key and /v1/messages", func(t *testing.T) {
		c := &ent.Channel{
			ID:          5,
			Name:        "anthropic",
			Type:        channel.TypeAnthropic,
			BaseURL:     "https://api.anthropic.com/v1",
			Credentials: objects.ChannelCredentials{APIKey: "sk-ant-test"},
		}

		ch, err := svc.buildChannelWithOutbounds(c)
		require.NoError(t, err)

		outbound := ch.Outbounds[llm.APIFormatAnthropicMessage.String()]
		require.NotNil(t, outbound)

		httpReq, err := outbound.TransformRequest(ctx, newRequest("claude-sonnet-4-5"))
		require.NoError(t, err)
		require.Equal(t, "https://api.anthropic.com/v1/messages", httpReq.URL)
		require.Equal(t, "sk-ant-test", httpReq.Auth.APIKey)
		require.Equal(t, httpclient.AuthTypeAPIKey, httpReq.Auth.Type)
		require.Equal(t, "X-API-Key", httpReq.Auth.HeaderKey)
	})

	t.Run("commandcode types require an enabled API key", func(t *testing.T) {
		for _, typ := range []channel.Type{channel.TypeCommandcode, channel.TypeCommandcodeAnthropic} {
			c := &ent.Channel{
				ID:          6,
				Name:        "cc-nokey",
				Type:        typ,
				BaseURL:     base,
				Credentials: objects.ChannelCredentials{},
			}
			_, err := svc.buildChannelWithOutbounds(c)
			require.Error(t, err)
			require.Contains(t, err.Error(), "missing api key")
		}
	})
	t.Run("explicit API key override can use a disabled configured key", func(t *testing.T) {
		c := &ent.Channel{
			ID:              7,
			Name:            "cc-disabled-key",
			Type:            channel.TypeCommandcode,
			BaseURL:         base,
			Credentials:     objects.ChannelCredentials{APIKeys: []string{"disabled-key"}},
			DisabledAPIKeys: []objects.DisabledAPIKey{{Key: "disabled-key"}},
		}

		_, err := svc.buildChannelWithOutbounds(c, "disabled-key")
		require.NoError(t, err)
	})
	t.Run("blank API key override does not replace an enabled key", func(t *testing.T) {
		c := &ent.Channel{
			ID:          9,
			Name:        "cc-blank-override",
			Type:        channel.TypeCommandcode,
			BaseURL:     base,
			Credentials: objects.ChannelCredentials{APIKey: "stored-key"},
		}

		ch, err := svc.buildChannelWithOutbounds(c, "  ")
		require.NoError(t, err)
		httpReq, err := ch.Outbounds[llm.APIFormatOpenAIChatCompletion.String()].TransformRequest(ctx, newRequest("gpt-5-codex"))
		require.NoError(t, err)
		require.Equal(t, "stored-key", httpReq.Auth.APIKey)
	})

	t.Run("insecure base URL is rejected", func(t *testing.T) {
		c := &ent.Channel{
			ID:          8,
			Name:        "cc-insecure",
			Type:        channel.TypeCommandcode,
			BaseURL:     "http://api.commandcode.ai/provider/v1",
			Credentials: objects.ChannelCredentials{APIKey: "sk-test"},
		}

		_, err := svc.buildChannelWithOutbounds(c)
		require.ErrorContains(t, err, "HTTPS")
	})
}

func ptrTo(s string) *string {
	return &s
}
