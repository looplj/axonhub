package biz

import (
	"context"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	zenmuxtransformer "github.com/looplj/axonhub/llm/transformer/zenmux"
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

func TestBuildChannelWithOutbounds_ZenMuxNativeVideoIsBoundOnlyToZenMux(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:zenmux_video_outbounds?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })
	svc := NewChannelServiceForTest(client)

	t.Run("ZenMux keeps OpenAI primary and binds native video separately", func(t *testing.T) {
		ch, err := svc.buildChannelWithOutbounds(&ent.Channel{
			ID:          1,
			Name:        "ZenMux native video",
			Type:        channel.TypeZenmux,
			Credentials: objects.ChannelCredentials{APIKey: "test-key"},
		})

		require.NoError(t, err)
		require.IsType(t, &openai.OutboundTransformer{}, ch.Outbound)
		require.Same(t, ch.Outbound, ch.Outbounds[llm.APIFormatOpenAIChatCompletion.String()])
		require.Same(t, ch.Outbound, ch.Outbounds[llm.APIFormatOpenAIVideo.String()])
		require.IsType(t, &zenmuxtransformer.OutboundTransformer{}, ch.Outbounds[llm.APIFormatZenmuxVideo.String()])
	})

	t.Run("unrelated channel rejects a persisted ZenMux video endpoint", func(t *testing.T) {
		ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

		_, err := svc.createChannel(ctx, ent.CreateChannelInput{
			Type:             channel.TypeOpenai,
			Name:             "OpenAI with invalid ZenMux endpoint",
			Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
			SupportedModels:  []string{"video-model"},
			DefaultTestModel: "video-model",
			Endpoints: []objects.ChannelEndpoint{{
				APIFormat: llm.APIFormatZenmuxVideo.String(),
			}},
		})

		require.ErrorContains(t, err, "zenmux/video")
	})

	t.Run("unrelated channel cannot instantiate a ZenMux video outbound", func(t *testing.T) {
		_, err := svc.buildChannelWithOutbounds(&ent.Channel{
			ID:          2,
			Name:        "OpenAI with invalid ZenMux endpoint",
			Type:        channel.TypeOpenai,
			BaseURL:     "https://api.example.invalid",
			Credentials: objects.ChannelCredentials{APIKey: "test-key"},
			Endpoints: []objects.ChannelEndpoint{{
				APIFormat: llm.APIFormatZenmuxVideo.String(),
			}},
		})

		require.ErrorContains(t, err, "zenmux/video")
	})

	t.Run("changing a ZenMux channel to an unrelated type rejects its native endpoint", func(t *testing.T) {
		ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
		zenmuxChannel, err := svc.createChannel(ctx, ent.CreateChannelInput{
			Type:             channel.TypeZenmux,
			Name:             "ZenMux endpoint type transition",
			Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
			SupportedModels:  []string{"video-model"},
			DefaultTestModel: "video-model",
			Endpoints: []objects.ChannelEndpoint{{
				APIFormat: llm.APIFormatZenmuxVideo.String(),
			}},
		})
		require.NoError(t, err)

		_, err = svc.UpdateChannel(ctx, zenmuxChannel.ID, &ent.UpdateChannelInput{
			Type: lo.ToPtr(channel.TypeOpenai),
		})

		require.ErrorContains(t, err, "zenmux/video")
	})
}
