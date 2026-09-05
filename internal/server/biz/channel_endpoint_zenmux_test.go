package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestDefaultEndpointsForChannelType_ZenMuxProtocolDefaults(t *testing.T) {
	tests := []struct {
		name     string
		typ      channel.Type
		expected []objects.ChannelEndpoint
	}{
		{
			name: "openai plus native video",
			typ:  channel.TypeZenmux,
			expected: append(
				append([]objects.ChannelEndpoint{}, DefaultEndpointsForChannelType(channel.TypeOpenai)...),
				objects.ChannelEndpoint{APIFormat: llm.APIFormatZenmuxVideo.String()},
			),
		},
		{name: "responses", typ: channel.TypeZenmuxResponses, expected: DefaultEndpointsForChannelType(channel.TypeNanogptResponses)},
		{name: "anthropic", typ: channel.TypeZenmuxAnthropic, expected: DefaultEndpointsForChannelType(channel.TypeMinimaxAnthropic)},
		{name: "gemini", typ: channel.TypeZenmuxGemini, expected: DefaultEndpointsForChannelType(channel.TypeGemini)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, DefaultEndpointsForChannelType(tt.typ))
		})
	}
}
