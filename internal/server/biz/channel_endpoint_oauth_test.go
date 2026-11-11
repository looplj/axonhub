package biz

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestBuildNonDefaultEndpointOutbound_RejectsMissingAPIKey(t *testing.T) {
	for _, credentials := range []struct {
		name  string
		value objects.ChannelCredentials
	}{
		{"empty", objects.ChannelCredentials{}},
		{"oauth", objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "synthetic-token"}}},
		{"legacy oauth", objects.ChannelCredentials{APIKey: `{"access_token":"synthetic-token"}`}},
	} {
		for _, format := range detectableEndpointAPIFormats {
			t.Run(credentials.name+"/"+format, func(t *testing.T) {
				entity := &ent.Channel{Name: "missing-key", Type: channel.TypeAntigravity, BaseURL: "https://upstream.example", Credentials: credentials.value}
				ch := buildChannel(entity, nil)
				require.Empty(t, ch.cachedEnabledAPIKeys)
				svc := &ChannelService{}
				require.NotPanics(t, func() {
					out, err := svc.buildNonDefaultEndpointOutbound(entity, ch, objects.ChannelEndpoint{APIFormat: format})
					require.ErrorContains(t, err, "no enabled api key")
					require.Nil(t, out)
				})
			})
		}
	}
}

func TestBuildNonDefaultEndpointOutbound_PreservesOAuthAndKeyAuth(t *testing.T) {
	for _, tc := range []struct {
		name     string
		typ      channel.Type
		format   string
		oauth    bool
		override string
	}{
		{"codex responses", channel.TypeCodex, llm.APIFormatOpenAIResponse.String(), true, ""},
		{"codex websocket", channel.TypeCodex, llm.APIFormatOpenAIResponseWebSocket.String(), true, ""},
		{"fenno responses", channel.TypeFenno, llm.APIFormatOpenAIResponse.String(), true, ""},
		{"fenno websocket", channel.TypeFenno, llm.APIFormatOpenAIResponseWebSocket.String(), true, ""},
		{"codex alpha search", channel.TypeCodex, llm.APIFormatOpenAIAlphaSearch.String(), true, ""},
		{"api key", channel.TypeOpenai, llm.APIFormatOpenAIChatCompletion.String(), false, ""},
		{"key override", channel.TypeOpenai, llm.APIFormatOpenAIChatCompletion.String(), false, "override-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			credentials := objects.ChannelCredentials{APIKey: "synthetic-key"}
			if tc.oauth {
				credentials = objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "synthetic-token", ExpiresAt: time.Now().Add(time.Hour)}}
			} else if tc.override != "" {
				credentials = objects.ChannelCredentials{}
			}
			entity := &ent.Channel{Name: tc.name, Type: tc.typ, BaseURL: "https://upstream.example", Credentials: credentials}
			ch := buildChannel(entity, nil)
			ch.apiKeyOverride = tc.override
			svc := &ChannelService{}
			out, err := svc.buildNonDefaultEndpointOutbound(entity, ch, objects.ChannelEndpoint{APIFormat: tc.format})
			require.NoError(t, err)
			require.NotNil(t, out)
		})
	}
}

func TestDetectChannelEndpoints_RecoversProbePanic(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	transport := &endpointDetectTransport{responder: func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/responses") {
			panic("synthetic probe failure")
		}
		return &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	}}
	svc.httpClient = httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})
	ch := newEndpointDetectTestChannel(t, ctx, client, "https://upstream.example")
	payload, err := svc.DetectChannelEndpoints(ctx, DetectChannelEndpointsInput{ChannelID: objects.GUID{Type: "Channel", ID: ch.ID}})
	require.NoError(t, err)
	require.Len(t, payload.Endpoints, len(detectableEndpointAPIFormats))
	for i, ep := range payload.Endpoints {
		require.Equal(t, detectableEndpointAPIFormats[i], ep.APIFormat)
		if ep.APIFormat == llm.APIFormatOpenAIResponse.String() {
			require.False(t, ep.Supported)
			require.Equal(t, endpointDetectReasonInvalid, ep.Reason)
		} else {
			require.True(t, ep.Supported)
		}
	}
}

func TestBuildNonDefaultEndpointOutbound_MissingKeyBoundaries(t *testing.T) {
	svc := &ChannelService{}
	for _, typ := range []channel.Type{channel.TypeCodex, channel.TypeFenno} {
		t.Run(string(typ), func(t *testing.T) {
			entity := &ent.Channel{Name: string(typ), Type: typ, BaseURL: "https://upstream.example"}
			out, err := svc.buildNonDefaultEndpointOutbound(entity, buildChannel(entity, nil), objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAIResponse.String()})
			require.ErrorContains(t, err, "no enabled api key")
			require.Nil(t, out)
		})
	}
	t.Run("multi-key fallback", func(t *testing.T) {
		entity := &ent.Channel{Name: "multi-key", Type: channel.TypeOpenai, BaseURL: "https://upstream.example", Credentials: objects.ChannelCredentials{APIKeys: []string{"key-1", "key-2"}}}
		ch := buildChannel(entity, nil)
		ch.cachedEnabledAPIKeys = nil
		out, err := svc.buildNonDefaultEndpointOutbound(entity, ch, objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAIChatCompletion.String()})
		require.NoError(t, err)
		require.NotNil(t, out)
	})
}

func TestDetectChannelEndpoints_OAuthOnly(t *testing.T) {
	for _, tc := range []struct {
		name         string
		typ          channel.Type
		legacy       bool
		custom       bool
		wantRequests int
	}{
		{"antigravity", channel.TypeAntigravity, true, false, 0},
		{"codex", channel.TypeCodex, false, false, 1},
		{"codex invalid custom endpoint", channel.TypeCodex, false, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, client := setupTestChannelService(t)
			defer client.Close()
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			transport := &endpointDetectTransport{responder: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
			}}
			svc.httpClient = httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})
			oauth := &objects.OAuthCredentials{AccessToken: "synthetic-token", ExpiresAt: time.Now().Add(time.Hour)}
			credentials := objects.ChannelCredentials{OAuth: oauth}
			if tc.legacy {
				raw, err := oauth.ToJSON()
				require.NoError(t, err)
				credentials = objects.ChannelCredentials{APIKey: raw}
			}
			builder := client.Channel.Create().SetType(tc.typ).SetName(tc.name).
				SetBaseURL("https://upstream.example").SetCredentials(credentials).
				SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model")
			if tc.custom {
				builder.SetEndpoints([]objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}})
			}
			ch, err := builder.Save(ctx)
			require.NoError(t, err)
			payload, err := svc.DetectChannelEndpoints(ctx, DetectChannelEndpointsInput{ChannelID: objects.GUID{Type: "Channel", ID: ch.ID}})
			require.NoError(t, err)
			require.Len(t, payload.Endpoints, 3)
			require.Len(t, transport.requests, tc.wantRequests)
			for _, ep := range payload.Endpoints {
				if tc.typ == channel.TypeCodex && ep.APIFormat == llm.APIFormatOpenAIResponse.String() {
					require.True(t, ep.Supported)
				} else {
					require.False(t, ep.Supported)
					require.Equal(t, endpointDetectReasonInvalid, ep.Reason)
				}
			}
		})
	}
}
