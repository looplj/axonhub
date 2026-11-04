package biz

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestMoonshotCodingChannelFiltersAnthropicNativeWebSearch(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(context.Background())

	entity := client.Channel.Create().
		SetName("Kimi Coding").
		SetType(channel.TypeMoonshotCoding).
		SetBaseURL("https://api.kimi.com/coding").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"k3.1-pro"}).
		SetDefaultTestModel("k3.1-pro").
		SaveX(ctx)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entity)
	require.NoError(t, err)
	require.IsType(t, &anthropic.OutboundTransformer{}, built.Outbound)

	request, err := built.Outbound.TransformRequest(ctx, &llm.Request{
		Model:     "k3.1-pro",
		MaxTokens: lo.ToPtr[int64](128),
		Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}},
		Stream:    lo.ToPtr(false),
		APIFormat: llm.APIFormatOpenAIResponseWebSocket,
		Tools:     moonshotCodingTestTools(),
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.kimi.com/coding/v1/messages", request.URL)

	var payload struct {
		Tools []anthropic.Tool `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(request.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "get_weather", payload.Tools[0].Name)
}

func moonshotCodingTestTools() []llm.Tool {
	return []llm.Tool{
		{
			Type: "function",
			Function: llm.Function{
				Name:       "get_weather",
				Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type:      llm.ToolTypeWebSearch,
			WebSearch: &llm.WebSearch{},
		},
	}
}
