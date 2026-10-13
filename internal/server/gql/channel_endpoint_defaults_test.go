package gql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestCodexResponsesEndpointDefaultsGraphQL(t *testing.T) {
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	for _, tt := range []struct {
		name      string
		base      string
		endpoints []objects.ChannelEndpoint
		want      []string
	}{
		{"HTTPS defaults", "https://chatgpt.com/backend-api/codex#", nil, []string{h, w}},
		{"legacy WSS defaults", "wss://example.test#", nil, []string{w}},
		{"explicit H", "https://example.test#", []objects.ChannelEndpoint{{APIFormat: h}}, []string{h}},
		{"legacy explicit HTTP", "wss://example.test#", []objects.ChannelEndpoint{{APIFormat: h, Transport: "http"}}, []string{h}},
		{"legacy websocket", "https://example.test#", []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket"}}, []string{w}},
		{"scheme inferred websocket", "https://example.test#", []objects.ChannelEndpoint{{APIFormat: h, BaseURL: "wss://other.test#"}}, []string{w}},
		{"canonical W", "https://example.test#", []objects.ChannelEndpoint{{APIFormat: w}}, []string{w}},
		{"dual", "https://example.test#", []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, []string{h, w}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			resolver := &channelResolver{}
			entity := &ent.Channel{Type: channel.TypeCodex, BaseURL: tt.base, Endpoints: tt.endpoints}
			// When
			defaults, err := resolver.DefaultEndpoints(context.Background(), entity)
			// Then
			require.NoError(t, err)
			var family []string
			for _, ep := range defaults {
				if ep.APIFormat == h || ep.APIFormat == w {
					family = append(family, ep.APIFormat)
					require.Empty(t, ep.Transport)
				}
			}
			require.Equal(t, tt.want, family)
			require.Equal(t, tt.endpoints, entity.Endpoints)
		})
	}
}
