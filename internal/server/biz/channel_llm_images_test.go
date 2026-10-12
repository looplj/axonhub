package biz

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
)

func TestRelayImageEndpointRouting(t *testing.T) {
	// Given: Codex defaults and explicit Images configurations on other channels.
	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	svc := NewChannelServiceForTest(client)
	for _, typ := range []channel.Type{channel.TypeCodex, channel.TypeFenno, channel.TypeOpenaiResponses, channel.TypeZenmuxResponses, channel.TypeNanogptResponses, channel.TypeXaiResponses, channel.TypeBailianResponses, channel.TypeOpencodeGo} {
		for _, custom := range []bool{false, true} {
			for _, format := range []llm.APIFormat{llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit} {
				t.Run(string(typ)+"/"+format.String()+"/custom="+map[bool]string{false: "false", true: "true"}[custom], func(t *testing.T) {
					c := &ent.Channel{Name: t.Name(), Type: typ, BaseURL: "https://relay.example/prefix#", Credentials: objects.ChannelCredentials{APIKey: "relay-key"}}
					wantURL := "https://relay.example/prefix/images/generations"
					if format == llm.APIFormatOpenAIImageEdit {
						wantURL = "https://relay.example/prefix/images/edits"
					}
					if custom {
						c.Endpoints = []objects.ChannelEndpoint{{APIFormat: format.String(), BaseURL: "https://images.example/override/", Path: "/custom-images"}}
						wantURL = "https://images.example/override/custom-images"
					}
					built, err := svc.buildChannelWithOutbounds(c)
					require.NoError(t, err)
					t.Cleanup(func() { stopChannelOutbounds(built) })
					// When: the resolved Images capability constructs a request.
					out, err := BuildOutboundByAPIFormat(built, format.String())
					if !custom && typ != channel.TypeCodex {
						require.ErrorContains(t, err, "does not support api_format")
						return
					}
					require.NoError(t, err)
					req, err := out.TransformRequest(t.Context(), &llm.Request{
						Model: "gpt-image-2", RequestType: llm.RequestTypeImage, APIFormat: format,
						Image: &llm.ImageRequest{Prompt: "draw a square", Images: [][]byte{[]byte("image-fixture")}}, RawRequest: &httpclient.Request{Headers: http.Header{"Session-Id": {"image-session"}}},
					})
					// Then: native routing retains URL, auth and the requested image model.
					require.NoError(t, err)
					require.NotSame(t, built.Outbound, out)
					require.Equal(t, wantURL, req.URL)
					require.Equal(t, http.MethodPost, req.Method)
					require.Equal(t, "relay-key", req.Auth.APIKey)
					require.Equal(t, format.String(), req.APIFormat)
					require.NotContains(t, string(req.Body), "image_generation")
					_, customized := out.(pipeline.ChannelCustomizedExecutor)
					require.False(t, customized, "Images must use the ordinary HTTP executor")
					if format == llm.APIFormatOpenAIImageGeneration || typ == channel.TypeCodex {
						require.Equal(t, "gpt-image-2", gjson.GetBytes(req.Body, "model").String())
					}
				})
			}
		}
	}
}

func TestRelayImageEndpointRoutingOAuth(t *testing.T) {
	// Given: real OAuth credentials on the channel, with a different API-key fallback.
	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	svc := NewChannelServiceForTest(client)
	for _, typ := range []channel.Type{channel.TypeCodex} {
		t.Run(string(typ), func(t *testing.T) {
			built, err := svc.buildChannelWithOutbounds(&ent.Channel{
				Name: t.Name(), Type: typ, BaseURL: "https://relay.example/backend#",
				Credentials: objects.ChannelCredentials{APIKey: "wrong-fallback", OAuth: &objects.OAuthCredentials{AccessToken: "oauth-image-token", RefreshToken: "refresh-fixture", ExpiresAt: time.Now().Add(time.Hour)}},
			})
			require.NoError(t, err)
			t.Cleanup(func() { stopChannelOutbounds(built) })
			// When
			out, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIImageGeneration.String())
			require.NoError(t, err)
			req, err := out.TransformRequest(t.Context(), &llm.Request{Model: "gpt-image-2", RequestType: llm.RequestTypeImage, APIFormat: llm.APIFormatOpenAIImageGeneration, Image: &llm.ImageRequest{Prompt: "square"}})
			// Then
			require.NoError(t, err)
			require.Equal(t, "oauth-image-token", req.Auth.APIKey)
			require.Equal(t, "https://relay.example/backend/images/generations", req.URL)
		})
	}
}
