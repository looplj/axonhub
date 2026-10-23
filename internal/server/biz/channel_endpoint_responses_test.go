package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestCodexResponsesEndpointDefaults(t *testing.T) {
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	for _, tt := range []struct {
		name      string
		base      string
		endpoints []objects.ChannelEndpoint
		want      []string
	}{
		{"unconfigured HTTPS", "https://chatgpt.com/backend-api/codex#", nil, []string{h, w}},
		{"unconfigured HTTP", "http://example.test:8080/base#", nil, []string{h, w}},
		{"legacy WSS base", "wss://example.test/base#", nil, []string{w}},
		{"legacy WS base", "ws://example.test/base#", nil, []string{w}},
		{"explicit H", "https://example.test/base#", []objects.ChannelEndpoint{{APIFormat: h}}, []string{h}},
		{"explicit legacy HTTP", "wss://example.test/base#", []objects.ChannelEndpoint{{APIFormat: h, Transport: "http"}}, []string{h}},
		{"legacy websocket", "https://example.test/base#", []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket", Path: "/custom"}}, []string{w}},
		{"scheme inferred override", "https://example.test/base#", []objects.ChannelEndpoint{{APIFormat: h, BaseURL: "wss://other.test/custom#"}}, []string{w}},
		{"inherited WSS", "wss://example.test/base#", []objects.ChannelEndpoint{{APIFormat: h}}, []string{w}},
		{"explicit W", "https://example.test/base#", []objects.ChannelEndpoint{{APIFormat: w}}, []string{w}},
		{"canonical dual", "https://example.test/base#", []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, []string{h, w}},
		{"legacy dual", "https://example.test/base#", []objects.ChannelEndpoint{{APIFormat: h, Transport: "http"}, {APIFormat: h, Transport: "websocket"}}, []string{h, w}},
		{"unrelated override", "https://example.test/base#", []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIImageEdit.String(), Path: "/edit"}}, []string{h, w}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			entity := &ent.Channel{Type: channel.TypeCodex, BaseURL: tt.base, Endpoints: tt.endpoints}
			ch := &Channel{Channel: entity}
			// When
			resolved := ch.ResolveEndpoints()
			// Then
			var formats []string
			for _, ep := range resolved {
				if ep.APIFormat == h || ep.APIFormat == w {
					formats = append(formats, ep.APIFormat)
					require.Empty(t, ep.Transport)
				}
			}
			require.Equal(t, tt.want, formats)
			require.Len(t, resolved, len(tt.want)+3, "non-family defaults survive")
			require.Equal(t, tt.endpoints, entity.Endpoints, "reads preserve raw endpoints")
			require.Equal(t, tt.base, entity.BaseURL, "reads preserve trailing #")
		})
	}
}

func TestResponsesEndpointLegacyNormalization(t *testing.T) {
	h, w := llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseWebSocket.String()
	t.Run("normalization precedes collision validation", func(t *testing.T) {
		// Given
		endpoints := []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket"}, {APIFormat: w}}
		// When
		err := ValidateEndpoints(endpoints)
		// Then
		require.ErrorContains(t, err, "duplicate api_format")
	})
	t.Run("legacy dual has distinct identities", func(t *testing.T) {
		// Given
		endpoints := []objects.ChannelEndpoint{{APIFormat: h, Transport: "http"}, {APIFormat: h, Transport: "websocket"}}
		// When
		err := ValidateEndpoints(endpoints)
		// Then
		require.NoError(t, err)
	})
	t.Run("legacy model restriction resolves W without mutation", func(t *testing.T) {
		// Given
		settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{h}}}}
		ch := &Channel{Channel: &ent.Channel{Type: channel.TypeCodex, BaseURL: "https://example.test#", Endpoints: []objects.ChannelEndpoint{{APIFormat: h, Transport: "websocket", Path: "/custom"}}, Settings: settings}}
		// When
		formats := ch.ForcedAPIFormats("test-model")
		// Then
		require.Equal(t, []string{w}, formats)
		require.Equal(t, []string{h}, settings.ModelProtocols[0].APIFormats)
		require.Equal(t, objects.ChannelEndpoint{APIFormat: w, Path: "/custom"}, ch.ResolveEndpoints()[0])
	})
	t.Run("H restriction on dual stays H", func(t *testing.T) {
		// Given
		ch := &Channel{Channel: &ent.Channel{Type: channel.TypeCodex, Endpoints: []objects.ChannelEndpoint{{APIFormat: h}, {APIFormat: w}}, Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{h}}}}}}
		// When
		formats := ch.ForcedAPIFormats("test-model")
		// Then
		require.Equal(t, []string{h}, formats)
	})
	t.Run("W restriction rejected on H only", func(t *testing.T) {
		// Given
		settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{w}}}}
		// When
		err := ValidateModelProtocols(settings, channel.TypeCodex, []objects.ChannelEndpoint{{APIFormat: h}})
		// Then
		require.ErrorContains(t, err, "not configured on this channel")
	})
	t.Run("H restriction rejected on canonical W only", func(t *testing.T) {
		settings := &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{h}}}}
		err := ValidateModelProtocols(settings, channel.TypeCodex, []objects.ChannelEndpoint{{APIFormat: w}})
		require.ErrorContains(t, err, "not configured on this channel")
	})
	t.Run("explicit HTTP on WSS stays canonical after normalization", func(t *testing.T) {
		// Given
		endpoints := []objects.ChannelEndpoint{{APIFormat: h, Transport: "http", Path: "/custom"}}
		// When
		canonical := NormalizeEndpoints(endpoints, "wss://example.test:8080/base#")
		// Then
		require.Equal(t, []objects.ChannelEndpoint{{APIFormat: h, BaseURL: "https://example.test:8080/base#", Path: "/custom"}}, canonical)
		require.Equal(t, canonical, NormalizeEndpoints(canonical, "wss://example.test:8080/base#"))
	})
	t.Run("W format owns transport even with legacy HTTP field", func(t *testing.T) {
		// Given
		ep := objects.ChannelEndpoint{APIFormat: w, Transport: "http", BaseURL: "https://example.test#"}
		// When
		transport := endpointTransport(ep)
		// Then
		require.Equal(t, objects.ChannelEndpointTransportWebSocket, transport)
	})
	t.Run("canonical W only retains primary websocket transport", func(t *testing.T) {
		// Given
		ch := &ent.Channel{Type: channel.TypeCodex, BaseURL: "https://example.test#", Endpoints: []objects.ChannelEndpoint{{APIFormat: w}}}
		// When
		transport := primaryEndpointTransport(ch, h)
		// Then
		require.Equal(t, objects.ChannelEndpointTransportWebSocket, transport)
	})
}
