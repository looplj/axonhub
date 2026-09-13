//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to each scenario.
package orchestrator

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	entchannel "github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestBillingSystemMessageMiddleware(t *testing.T) {
	oauthCredentials := objects.ChannelCredentials{
		OAuth: &objects.OAuthCredentials{AccessToken: "test-access-token"},
	}
	apiKeyCredentials := objects.ChannelCredentials{APIKey: "test-api-key"}

	tests := []struct {
		name        string
		channelType entchannel.Type
		credentials objects.ChannelCredentials
		settings    *objects.ChannelSettings
		wantBilling bool
	}{
		{
			name:        "auto preserves billing message for official Claude Code OAuth",
			channelType: entchannel.TypeClaudecode,
			credentials: oauthCredentials,
			settings:    &objects.ChannelSettings{ClaudeCodeBillingHeader: objects.ClaudeCodeBillingHeaderAuto},
			wantBilling: true,
		},
		{
			name:        "auto removes billing message for Claude Code API key",
			channelType: entchannel.TypeClaudecode,
			credentials: apiKeyCredentials,
			settings:    &objects.ChannelSettings{ClaudeCodeBillingHeader: objects.ClaudeCodeBillingHeaderAuto},
		},
		{
			name:        "auto removes billing message for Anthropic-compatible channel",
			channelType: entchannel.TypeAnthropic,
			settings:    &objects.ChannelSettings{ClaudeCodeBillingHeader: objects.ClaudeCodeBillingHeaderAuto},
		},
		{
			name:        "keep preserves billing message for Anthropic-compatible channel",
			channelType: entchannel.TypeAnthropic,
			credentials: apiKeyCredentials,
			settings:    &objects.ChannelSettings{ClaudeCodeBillingHeader: objects.ClaudeCodeBillingHeaderKeep},
			wantBilling: true,
		},
		{
			name:        "strip removes billing message for official Claude Code OAuth",
			channelType: entchannel.TypeClaudecode,
			credentials: oauthCredentials,
			settings:    &objects.ChannelSettings{ClaudeCodeBillingHeader: objects.ClaudeCodeBillingHeaderStrip},
		},
		{
			name:        "unset settings fall back to auto for official Claude Code OAuth",
			channelType: entchannel.TypeClaudecode,
			credentials: oauthCredentials,
			wantBilling: true,
		},
		{
			name:        "unset settings fall back to auto for Anthropic-compatible channel",
			channelType: entchannel.TypeAnthropic,
			credentials: apiKeyCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{
					Type:        tt.channelType,
					Credentials: tt.credentials,
					Settings:    tt.settings,
				}}},
			}
			middleware := newBillingSystemMessageMiddleware(state)
			billing := "x-anthropic-billing-header: cc_version=2.1.42; cch=38a80;"
			request := &llm.Request{Messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: &billing}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
			}}

			result, err := middleware.OnOutboundLlmRequest(t.Context(), request, llm.APIFormatAnthropicMessage)

			require.NoError(t, err)
			if tt.wantBilling {
				require.Same(t, request, result)
				require.Equal(t, billing, *result.Messages[0].Content.Content)
			} else {
				require.NotSame(t, request, result)
				require.Len(t, result.Messages, 1)
				require.Equal(t, "user", result.Messages[0].Role)
				require.Len(t, request.Messages, 2, "filtering must not mutate the shared request")
			}
		})
	}
}
