package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestCodexChannel_ImageMainModelUsesDefaultTestModel(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:codex_image_main_model?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	svc := NewChannelServiceForTest(client)

	for _, auth := range []string{"api key", "oauth"} {
		for _, tt := range []struct {
			name, defaultModel, want string
			settings                 *objects.ChannelSettings
		}{
			{name: "default", defaultModel: "gpt-6-sol", want: "gpt-6-sol"},
			{name: "empty", want: "gpt-6-luna"},
			{name: "image default", defaultModel: "gpt-image-2", want: "gpt-6-luna"},
			{
				name: "mapped alias", defaultModel: " main ", want: "gpt-6-sol",
				settings: &objects.ChannelSettings{ModelMappings: []objects.ModelMapping{{From: "main", To: "gpt-6-sol"}}},
			},
			{
				name: "mapped image", defaultModel: "main", want: "gpt-6-luna",
				settings: &objects.ChannelSettings{ModelMappings: []objects.ModelMapping{{From: "main", To: "gpt-image-2"}}},
			},
			{
				name: "case insensitive alias", defaultModel: "MAIN", want: "gpt-6-sol",
				settings: &objects.ChannelSettings{LowercaseModelID: true, ModelMappings: []objects.ModelMapping{{From: "main", To: "gpt-6-sol"}}},
			},
		} {
			t.Run(auth+"/"+tt.name, func(t *testing.T) {
				credentials := objects.ChannelCredentials{APIKey: "test-key"}
				if auth == "oauth" {
					credentials = objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{
						AccessToken: "test-token", RefreshToken: "test-refresh", ExpiresAt: time.Now().Add(time.Hour),
					}}
				}
				built, err := svc.buildChannelWithOutbounds(&ent.Channel{
					Name: "codex images", Type: channel.TypeCodex,
					BaseURL: "https://relay.example/backend-api/codex#", Credentials: credentials,
					SupportedModels:  []string{"gpt-6-sol", "gpt-image-2"},
					DefaultTestModel: tt.defaultModel, Settings: tt.settings,
					Endpoints: []objects.ChannelEndpoint{{
						APIFormat: llm.APIFormatOpenAIImageGeneration.String(),
						BaseURL:   "https://images.example/backend-api/codex#",
					}},
				})
				require.NoError(t, err)
				t.Cleanup(func() { stopChannelOutbounds(built) })
				imageOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIImageGeneration.String())
				require.NoError(t, err)
				require.NotSame(t, built.Outbound, imageOutbound, "custom endpoint must exercise a separate transformer")
				for _, out := range []transformer.Outbound{built.Outbound, imageOutbound} {
					req, err := out.TransformRequest(t.Context(), &llm.Request{
						Model: "gpt-image-2", RequestType: llm.RequestTypeImage, APIFormat: llm.APIFormatOpenAIImageGeneration,
						Image: &llm.ImageRequest{Prompt: "draw a tree"},
					})
					require.NoError(t, err)
					require.Equal(t, tt.want, gjson.GetBytes(req.Body, "model").String())
					require.Equal(t, "gpt-image-2", gjson.GetBytes(req.Body, "tools.0.model").String())
				}
			})
		}
	}
}
