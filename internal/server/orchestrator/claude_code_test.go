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
	"github.com/looplj/axonhub/llm/httpclient"
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

// TestBillingSystemMessageMiddlewareLeavesRawBodyIntact pins the reason the
// removal is not observable from the channel configuration alone: it rewrites
// the unified request only. A pass-through channel replays RawRequest.Body, so
// the very same channel forwards the billing block when pass-through body is on
// and drops it when it is off. Auto cannot express either intent on purpose;
// keep and strip make the outcome independent of pass-through.
func TestBillingSystemMessageMiddlewareLeavesRawBodyIntact(t *testing.T) {
	rawBody := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.42;"}]}`)
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{
			Type: entchannel.TypeAnthropic,
		}}},
	}
	billing := "x-anthropic-billing-header: cc_version=2.1.42;"
	request := &llm.Request{
		RawRequest: &httpclient.Request{Body: rawBody},
		Messages: []llm.Message{
			{Role: "system", Content: llm.MessageContent{Content: &billing}},
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}},
		},
	}

	result, err := newBillingSystemMessageMiddleware(state).
		OnOutboundLlmRequest(t.Context(), request, llm.APIFormatAnthropicMessage)

	require.NoError(t, err)
	require.Len(t, result.Messages, 1, "the unified request loses the billing block")
	require.Equal(t, rawBody, result.RawRequest.Body, "the raw body a pass-through channel replays still carries it")
}

// TestBillingHeaderModeAgainstPassThroughBody covers the pass-through boundary:
// replaying the raw inbound body must not resurrect a block on a channel pinned
// to strip, and must not cost byte-exact pass-through for anything else.
func TestBillingHeaderModeAgainstPassThroughBody(t *testing.T) {
	const billingText = "x-anthropic-billing-header: cc_version=2.1.42;"

	withBilling := []byte(`{"model":"claude-sonnet-4","system":[{"type":"text","text":"` + billingText +
		`"},{"type":"text","text":"You are Claude Code."}],"messages":[{"role":"user","content":"hi"}]}`)
	withoutBilling := []byte(`{"model":"claude-sonnet-4","system":[{"type":"text","text":"You are Claude Code."}],"messages":[{"role":"user","content":"hi"}]}`)

	tests := []struct {
		name            string
		mode            objects.ClaudeCodeBillingHeaderMode
		rawBody         []byte
		inboundMessages []llm.Message
		wantReplayed    bool
	}{
		{
			name:    "strip skips the replay that would restore the block",
			mode:    objects.ClaudeCodeBillingHeaderStrip,
			rawBody: withBilling,
			inboundMessages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr(billingText)}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
		},
		{
			name:    "strip keeps pass-through byte-exact when no block is present",
			mode:    objects.ClaudeCodeBillingHeaderStrip,
			rawBody: withoutBilling,
			inboundMessages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
			wantReplayed: true,
		},
		{
			name:    "keep replays the block as the operator asked",
			mode:    objects.ClaudeCodeBillingHeaderKeep,
			rawBody: withBilling,
			inboundMessages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr(billingText)}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
			wantReplayed: true,
		},
		{
			// Documents the historic behaviour the explicit modes exist to escape:
			// under auto the outcome still depends on pass-through.
			name:    "auto still lets pass-through decide",
			mode:    objects.ClaudeCodeBillingHeaderAuto,
			rawBody: withBilling,
			inboundMessages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr(billingText)}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
			wantReplayed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &biz.Channel{Channel: &ent.Channel{
				ID:   1,
				Name: "relay",
				Type: entchannel.TypeAnthropic,
				Settings: &objects.ChannelSettings{
					PassThroughBody:         lo.ToPtr(true),
					ClaudeCodeBillingHeader: tt.mode,
				},
			}}
			outbound := &PersistentOutboundTransformer{state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
				LlmRequest: &llm.Request{
					Model:      "claude-sonnet-4",
					APIFormat:  llm.APIFormatAnthropicMessage,
					Messages:   tt.inboundMessages,
					RawRequest: &httpclient.Request{APIFormat: string(llm.APIFormatAnthropicMessage), Body: tt.rawBody},
				},
			}}

			// What the outbound transformer serialized after the billing
			// middleware filtered the unified request.
			serialized := &httpclient.Request{
				APIFormat: string(llm.APIFormatAnthropicMessage),
				Body:      withoutBilling,
			}

			processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(t.Context(), serialized)

			require.NoError(t, err)
			require.Equal(t, tt.wantReplayed, outbound.state.PassThroughApplied)

			if tt.wantReplayed {
				require.Equal(t, string(tt.rawBody), string(processed.Body))
			} else {
				require.NotContains(t, string(processed.Body), "x-anthropic-billing-header")
			}
		})
	}
}
