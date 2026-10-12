package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestNativeImageRoutingSurvivesCustomResponses(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	svc := NewChannelServiceForTest(client)
	for _, typ := range []channel.Type{channel.TypeGemini, channel.TypeDoubao, channel.TypeZai, channel.TypeOpenrouter, channel.TypeQiniu} {
		t.Run(string(typ), func(t *testing.T) {
			// Given
			c := &Channel{Channel: &ent.Channel{Type: typ, Endpoints: []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIResponse.String(), BaseURL: "https://response.example/v1"}}}}
			before := (&Channel{Channel: &ent.Channel{Type: typ}}).ResolveEndpoints()
			// When
			after := c.ResolveEndpoints()
			// Then: custom Responses adds no preferred Images routing ahead of native fallback.
			require.Equal(t, append(before, c.Endpoints...), after)
			c.BaseURL = "https://native.example/v1"
			c.Name = t.Name()
			c.Credentials = objects.ChannelCredentials{APIKey: "test-key"}
			built, err := svc.buildChannelWithOutbounds(c.Channel)
			require.NoError(t, err)
			t.Cleanup(func() { stopChannelOutbounds(built) })
			out, err := BuildOutboundByAPIFormat(built, after[0].APIFormat)
			require.NoError(t, err)
			req, err := out.TransformRequest(t.Context(), &llm.Request{Model: "image-model", RequestType: llm.RequestTypeImage, APIFormat: llm.APIFormatOpenAIImageGeneration, Image: &llm.ImageRequest{Prompt: "draw"}})
			require.NoError(t, err)
			require.Contains(t, req.URL, "https://native.example/")
			require.NotContains(t, req.URL, "response.example")
		})
	}
}

func TestNonCodexResponsesDoNotInferImageEndpoints(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	svc := NewChannelServiceForTest(client)
	for _, typ := range []channel.Type{channel.TypeFenno, channel.TypeOpenaiResponses, channel.TypeZenmuxResponses, channel.TypeNanogptResponses, channel.TypeXaiResponses, channel.TypeBailianResponses, channel.TypeOpencodeGo, channel.TypeAnthropic} {
		for _, custom := range []bool{false, true} {
			t.Run(string(typ)+"/custom="+map[bool]string{false: "false", true: "true"}[custom], func(t *testing.T) {
				// Given: a non-Codex channel, optionally with a custom Responses endpoint.
				c := &ent.Channel{Name: t.Name(), Type: typ, BaseURL: "https://channel.example/v1", Credentials: objects.ChannelCredentials{APIKey: "test-key"}}
				if custom {
					c.Endpoints = []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIResponse.String(), BaseURL: "https://responses.example/v1"}}
				}
				// When
				built, err := svc.buildChannelWithOutbounds(c)
				// Then: neither endpoint resolution nor outbound construction invents Images.
				require.NoError(t, err)
				t.Cleanup(func() { stopChannelOutbounds(built) })
				for _, format := range []llm.APIFormat{llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit} {
					for _, ep := range built.ResolveEndpoints() {
						require.NotEqual(t, format.String(), ep.APIFormat)
					}
					_, err := BuildOutboundByAPIFormat(built, format.String())
					require.ErrorContains(t, err, "does not support api_format")
				}
			})
		}
	}
}

func TestCustomResponsesDoNotInferImageEndpoints(t *testing.T) {
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIResponse, llm.APIFormatOpenAIResponseCompact} {
		// Given: a generic Responses primary with a separate custom Responses URL.
		c := &Channel{Channel: &ent.Channel{Type: channel.TypeXaiSubscription, Endpoints: []objects.ChannelEndpoint{{APIFormat: format.String(), BaseURL: "https://responses.example/v1"}}}}
		// When
		endpoints := c.ResolveEndpoints()
		// Then
		want := []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIResponse.String()}}
		if format == llm.APIFormatOpenAIResponse {
			want = c.Endpoints
		} else {
			want = append(want, c.Endpoints...)
		}
		require.Equal(t, want, endpoints)
	}
}

func TestExplicitImagesDoNotInheritResponsesBaseURL(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	svc := NewChannelServiceForTest(client)
	for _, typ := range []channel.Type{channel.TypeCodex, channel.TypeFenno, channel.TypeOpenaiResponses, channel.TypeZenmuxResponses, channel.TypeNanogptResponses, channel.TypeXaiResponses, channel.TypeBailianResponses, channel.TypeOpencodeGo} {
		for _, explicit := range []bool{false, true} {
			t.Run(string(typ)+"/explicit="+map[bool]string{false: "false", true: "true"}[explicit], func(t *testing.T) {
				// Given: three distinct addresses to prove override precedence.
				c := &ent.Channel{Name: t.Name(), Type: typ, BaseURL: "https://channel.example/a#", Credentials: objects.ChannelCredentials{APIKey: "test-key"}, Endpoints: []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIResponse.String(), BaseURL: "https://responses.example/b#", Path: "/custom-responses"}}}
				want := "https://channel.example/a/images/generations"
				if explicit {
					c.Endpoints = append(c.Endpoints, objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAIImageGeneration.String(), BaseURL: "https://images.example/c#", Path: "/custom-images"})
					want = "https://images.example/c/custom-images"
				}
				built, err := svc.buildChannelWithOutbounds(c)
				require.NoError(t, err)
				t.Cleanup(func() { stopChannelOutbounds(built) })
				out, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIImageGeneration.String())
				if !explicit && typ != channel.TypeCodex {
					require.ErrorContains(t, err, "does not support api_format")
					return
				}
				require.NoError(t, err)
				// When
				req, err := out.TransformRequest(t.Context(), &llm.Request{Model: "gpt-image-2", RequestType: llm.RequestTypeImage, APIFormat: llm.APIFormatOpenAIImageGeneration, Image: &llm.ImageRequest{Prompt: "draw"}})
				// Then
				require.NoError(t, err)
				require.Equal(t, want, req.URL)
			})
		}
	}
}
