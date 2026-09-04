package biz

import (
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestBuildChannelWithTransformer_ZenMuxUsesProtocolTransformerAndDefaultBaseURL(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:zenmux_transformer?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })
	svc := NewChannelServiceForTest(client)

	tests := []struct {
		name          string
		channelType   channel.Type
		wantOutbound  any
		wantURLPrefix string
	}{
		{name: "openai", channelType: channel.TypeZenmux, wantOutbound: &openai.OutboundTransformer{}, wantURLPrefix: "https://zenmux.ai/api/v1/"},
		{name: "responses", channelType: channel.TypeZenmuxResponses, wantOutbound: &responses.OutboundTransformer{}, wantURLPrefix: "https://zenmux.ai/api/v1/"},
		{name: "anthropic", channelType: channel.TypeZenmuxAnthropic, wantOutbound: &anthropic.OutboundTransformer{}, wantURLPrefix: "https://zenmux.ai/api/anthropic/"},
		{name: "gemini", channelType: channel.TypeZenmuxGemini, wantOutbound: &gemini.OutboundTransformer{}, wantURLPrefix: "https://zenmux.ai/api/vertex-ai/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := svc.buildChannelWithTransformer(&ent.Channel{
				ID:          1,
				Name:        "ZenMux " + tt.name,
				Type:        tt.channelType,
				Credentials: objects.ChannelCredentials{APIKey: "test-key"},
			})
			require.NoError(t, err)
			require.IsType(t, tt.wantOutbound, ch.Outbound)

			request, err := ch.Outbound.TransformRequest(t.Context(), &llm.Request{
				Model: "test-model",
				Messages: []llm.Message{{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("hello")},
				}},
			})
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(request.URL, tt.wantURLPrefix), request.URL)
		})
	}
}
