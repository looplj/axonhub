package cacheidentity

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
)

func TestChannelAllowsPromptCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		chType       channel.Type
		baseURL      string
		trustedHosts []string
		wantAllow    bool
	}{
		{
			name:      "official openai host allowed",
			chType:    channel.TypeOpenai,
			baseURL:   "https://api.openai.com/v1",
			wantAllow: true,
		},
		{
			name:      "responses official openai host allowed",
			chType:    channel.TypeOpenaiResponses,
			baseURL:   "https://api.openai.com/v1",
			wantAllow: true,
		},
		{
			name:      "empty base url denied",
			chType:    channel.TypeOpenai,
			baseURL:   "",
			wantAllow: false,
		},
		{
			name:      "third party host denied with no trusted list",
			chType:    channel.TypeOpenai,
			baseURL:   "https://openrouter.ai/api/v1",
			wantAllow: false,
		},
		{
			name:      "non openai channel denied even with trusted host",
			chType:    channel.TypeOpenrouter,
			baseURL:   "https://api.openai.com/v1",
			wantAllow: false,
		},
		{
			name:         "trusted proxy allowed",
			chType:       channel.TypeOpenai,
			baseURL:      "https://ai-hub.miguocomics.co/v1",
			trustedHosts: []string{"ai-hub.miguocomics.co"},
			wantAllow:    true,
		},
		{
			name:         "trusted proxy case insensitive",
			chType:       channel.TypeOpenai,
			baseURL:      "https://ai-hub.miguocomics.co/v1",
			trustedHosts: []string{"AI-Hub.MiguoComics.CO"},
			wantAllow:    true,
		},
		{
			name:         "untrusted proxy denied even with other trusted hosts",
			chType:       channel.TypeOpenai,
			baseURL:      "https://unknown-proxy.example.com/v1",
			trustedHosts: []string{"ai-hub.miguocomics.co"},
			wantAllow:    false,
		},
		{
			name:         "official openai always trusted even without trusted list",
			chType:       channel.TypeOpenai,
			baseURL:      "https://api.openai.com/v1",
			trustedHosts: nil,
			wantAllow:    true,
		},
		{
			name:         "non openai channel denied even with matching trusted host",
			chType:       channel.TypeGeminiOpenai,
			baseURL:      "https://ai-hub.miguocomics.co/v1",
			trustedHosts: []string{"ai-hub.miguocomics.co"},
			wantAllow:    false,
		},
		{
			name:         "openai responses channel with trusted proxy",
			chType:       channel.TypeOpenaiResponses,
			baseURL:      "https://ai-hub.miguocomics.co/v1",
			trustedHosts: []string{"ai-hub.miguocomics.co"},
			wantAllow:    true,
		},
		{
			name:         "multiple trusted hosts second match",
			chType:       channel.TypeOpenai,
			baseURL:      "https://proxy-b.internal/v1",
			trustedHosts: []string{"proxy-a.internal", "proxy-b.internal"},
			wantAllow:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ChannelAllowsPromptCacheKey(tc.chType, tc.baseURL, tc.trustedHosts)
			require.Equal(t, tc.wantAllow, got)
		})
	}
}
