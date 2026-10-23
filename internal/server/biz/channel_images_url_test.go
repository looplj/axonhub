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

func TestCodexImagesURLMatrix(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	svc := NewChannelServiceForTest(client)
	for _, typ := range []channel.Type{channel.TypeCodex} {
		for _, tc := range []struct {
			name, base, path, prefix string
			raw                      bool
		}{
			{name: "default endpoints", base: "https://relay.example/v1", prefix: "https://relay.example/v1"},
			{name: "empty"},
			{name: "legacy", base: "https://api.openai.com/v1", prefix: "https://api.openai.com/v1"},
			{name: "hash", base: "https://relay.example/prefix#", prefix: "https://relay.example/prefix"},
			{name: "double hash", base: "https://relay.example/raw##", prefix: "https://relay.example/raw", raw: true},
			{name: "trailing slash", base: "https://relay.example/prefix/", prefix: "https://relay.example/prefix/v1"},
			{name: "custom prefix", base: "https://relay.example/prefix", prefix: "https://relay.example/prefix/v1"},
			{name: "custom endpoint", base: "https://relay.example/prefix/", path: "/custom-images", prefix: "https://relay.example/prefix"},
			{name: "websocket base", base: "wss://relay.example/prefix#", prefix: "https://relay.example/prefix"},
		} {
			for _, format := range []llm.APIFormat{llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit} {
				t.Run(string(typ)+"/"+tc.name+"/"+format.String(), func(t *testing.T) {
					// Given: the actual channel builder and existing URL marker semantics.
					c := &ent.Channel{Name: t.Name(), Type: typ, BaseURL: tc.base, Credentials: objects.ChannelCredentials{APIKey: "test-key"}}
					if tc.path != "" {
						c.Endpoints = []objects.ChannelEndpoint{{APIFormat: format.String(), Path: tc.path}}
					}
					built, err := svc.buildChannelWithOutbounds(c)
					// When
					require.NoError(t, err)
					t.Cleanup(func() { stopChannelOutbounds(built) })
					out, err := BuildOutboundByAPIFormat(built, format.String())
					require.NoError(t, err)
					req, err := out.TransformRequest(t.Context(), &llm.Request{Model: "gpt-image-2", RequestType: llm.RequestTypeImage, APIFormat: format, Image: &llm.ImageRequest{Prompt: "draw", Images: [][]byte{[]byte("fixture")}}})
					// Then: exact URLs, including raw URLs and custom paths, are retained.
					require.NoError(t, err)
					prefix := tc.prefix
					if tc.base == "" || tc.name == "legacy" {
						prefix = "https://chatgpt.com/backend-api/codex"
					}
					path := "/images/generations"
					if format == llm.APIFormatOpenAIImageEdit {
						path = "/images/edits"
					}
					if tc.path != "" {
						path = tc.path
					}
					if tc.raw {
						path = ""
					}
					require.Equal(t, prefix+path, req.URL)
					require.Equal(t, "test-key", req.Auth.APIKey)
				})
			}
		}
	}
}
