//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to each scenario.
package biz

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// TestBuildChannelWithOutbounds_FamilyRoutingOnCustomChatEndpoints asserts that
// custom chat endpoints are served by the channel family's own transformer
// (zai v4 / doubao v3) instead of the generic OpenAI transformer, whose /v1
// normalization produces broken URLs for those providers. It also asserts the
// family transformer strips metadata (GLM coding plan rejects it with 1210).
func TestBuildChannelWithOutbounds_FamilyRoutingOnCustomChatEndpoints(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_endpoint_routing?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	svc := NewChannelServiceForTest(client)
	ctx := context.Background()

	newRequest := func() *llm.Request {
		return &llm.Request{
			Model: "glm-5.3-flash",
			Metadata: map[string]string{
				"user_id": "user-123",
			},
			ReasoningEffort: "high",
			Messages: []llm.Message{
				{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
				},
			},
		}
	}

	transformChatEndpoint := func(t *testing.T, channelType channel.Type, ep objects.ChannelEndpoint) (url string, body map[string]any) {
		t.Helper()

		c := &ent.Channel{
			ID:          1,
			Name:        "test",
			Type:        channelType,
			BaseURL:     "https://upstream.example.com",
			Credentials: objects.ChannelCredentials{APIKey: "test-key"},
			Endpoints:   []objects.ChannelEndpoint{ep},
		}

		ch, err := svc.buildChannelWithOutbounds(c)
		require.NoError(t, err)

		outbound := ch.Outbounds[llm.APIFormatOpenAIChatCompletion.String()]
		require.NotNil(t, outbound)

		httpReq, err := outbound.TransformRequest(ctx, newRequest())
		require.NoError(t, err)

		require.NoError(t, json.Unmarshal(httpReq.Body, &body))

		return httpReq.URL, body
	}

	t.Run("zhipu_anthropic chat endpoint uses the zai transformer URL convention", func(t *testing.T) {
		url, body := transformChatEndpoint(t, channel.TypeZhipuAnthropic, objects.ChannelEndpoint{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			BaseURL:   "https://open.bigmodel.cn/api/coding/paas/v4",
		})

		require.Equal(t, "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions", url)
		require.NotContains(t, body, "metadata")
		require.Contains(t, body, "thinking")
	})

	t.Run("zhipu_anthropic chat endpoint with path keeps working", func(t *testing.T) {
		url, body := transformChatEndpoint(t, channel.TypeZhipuAnthropic, objects.ChannelEndpoint{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			BaseURL:   "https://open.bigmodel.cn/api/coding/paas/v4",
			Path:      "/chat/completions",
		})

		require.Equal(t, "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions", url)
		require.NotContains(t, body, "metadata")
	})

	t.Run("doubao chat endpoint uses the doubao transformer URL convention", func(t *testing.T) {
		url, body := transformChatEndpoint(t, channel.TypeDoubao, objects.ChannelEndpoint{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			BaseURL:   "https://ark.cn-beijing.volces.com/api/v3",
		})

		require.Equal(t, "https://ark.cn-beijing.volces.com/api/v3/chat/completions", url)
		require.NotContains(t, body, "metadata")
	})

	t.Run("legacy family endpoint without version keeps the generic v1 route", func(t *testing.T) {
		url, _ := transformChatEndpoint(t, channel.TypeZhipuAnthropic, objects.ChannelEndpoint{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			BaseURL:   "https://legacy-proxy.example.com/api",
		})

		require.Equal(t, "https://legacy-proxy.example.com/api/v1/chat/completions", url)
	})

	t.Run("generic openai channels keep the generic transformer", func(t *testing.T) {
		url, _ := transformChatEndpoint(t, channel.TypeOpenai, objects.ChannelEndpoint{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			BaseURL:   "https://api.example.com",
		})

		// Generic openai normalization appends /v1, which is correct for v1 APIs.
		require.Equal(t, "https://api.example.com/v1/chat/completions", url)
	})
}
