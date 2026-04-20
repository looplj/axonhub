package cacheidentity

import (
	"testing"

	"github.com/looplj/axonhub/internal/ent/channel"
)

func TestChannelAllowsPromptCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		chType    channel.Type
		baseURL   string
		wantAllow bool
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
			name:      "third party host denied",
			chType:    channel.TypeOpenai,
			baseURL:   "https://openrouter.ai/api/v1",
			wantAllow: false,
		},
		{
			name:      "non openai channel denied",
			chType:    channel.TypeOpenrouter,
			baseURL:   "https://api.openai.com/v1",
			wantAllow: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ChannelAllowsPromptCacheKey(tc.chType, tc.baseURL)
			if got != tc.wantAllow {
				t.Fatalf("ChannelAllowsPromptCacheKey(%q, %q) = %v, want %v", tc.chType, tc.baseURL, got, tc.wantAllow)
			}
		})
	}
}
