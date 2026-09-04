package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
)

func TestDefaultEndpointsForChannelType_ZenMuxMatchesProtocolTwin(t *testing.T) {
	tests := []struct {
		name string
		typ  channel.Type
		twin channel.Type
	}{
		{name: "openai", typ: channel.TypeZenmux, twin: channel.TypeOpenai},
		{name: "responses", typ: channel.TypeZenmuxResponses, twin: channel.TypeNanogptResponses},
		{name: "anthropic", typ: channel.TypeZenmuxAnthropic, twin: channel.TypeMinimaxAnthropic},
		{name: "gemini", typ: channel.TypeZenmuxGemini, twin: channel.TypeGemini},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, DefaultEndpointsForChannelType(tt.twin), DefaultEndpointsForChannelType(tt.typ))
		})
	}
}
