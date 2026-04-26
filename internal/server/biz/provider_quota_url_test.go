package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

func TestGetProviderType_OpenaiWithWaferURL(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://pass.wafer.ai",
	}
	result := svc.getProviderType(ch)
	require.Equal(t, "wafer", result)
}

func TestGetProviderType_OpenaiWithUnknownURL(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.unknown.com",
	}
	result := svc.getProviderType(ch)
	require.Equal(t, "", result)
}

func TestGetProviderType_ExistingTypesPreserved(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	tests := []struct {
		name           string
		channelType    channel.Type
		expectedResult string
	}{
		{"claudecode", channel.TypeClaudecode, "claudecode"},
		{"codex", channel.TypeCodex, "codex"},
		{"github_copilot", channel.TypeGithubCopilot, "github_copilot"},
		{"nanogpt", channel.TypeNanogpt, "nanogpt"},
		{"nanogpt_responses", channel.TypeNanogptResponses, "nanogpt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &ent.Channel{Type: tt.channelType}
			result := svc.getProviderType(ch)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetProviderType_OpenaiResponsesWithSyntheticURL(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	ch := &ent.Channel{
		Type:    channel.TypeOpenaiResponses,
		BaseURL: "https://api.synthetic.new",
	}
	result := svc.getProviderType(ch)
	require.Equal(t, "synthetic", result)
}

func TestGetProviderType_OpenaiWithNeuralWattURL(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.neuralwatt.com",
	}
	result := svc.getProviderType(ch)
	require.Equal(t, "neuralwatt", result)
}

func TestGetProviderType_OpenaiWithEmptyURL(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "",
	}
	result := svc.getProviderType(ch)
	require.Equal(t, "", result)
}

func TestHasCredentialsForProvider_WaferAPIKey(t *testing.T) {
	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://wafer.ai",
		Credentials: objects.ChannelCredentials{
			APIKey: "sk-test",
		},
	}
	require.True(t, hasCredentialsForProvider(ch))
}

func TestHasCredentialsForProvider_WaferNoKey(t *testing.T) {
	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://wafer.ai",
		Credentials: objects.ChannelCredentials{},
	}
	require.False(t, hasCredentialsForProvider(ch))
}

func TestHasCredentialsForProvider_WaferOAuthIgnored(t *testing.T) {
	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://wafer.ai",
		Credentials: objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: "token"},
		},
	}
	require.False(t, hasCredentialsForProvider(ch))
}

func TestHasCredentialsForProvider_SyntheticAPIKeys(t *testing.T) {
	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.synthetic.new",
		Credentials: objects.ChannelCredentials{
			APIKeys: []string{"sk-test"},
		},
	}
	require.True(t, hasCredentialsForProvider(ch))
}

func TestHasCredentialsForProvider_NonOpenaiWithOAuth(t *testing.T) {
	ch := &ent.Channel{
		Type: channel.TypeClaudecode,
		Credentials: objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: "token"},
		},
	}
	require.True(t, hasCredentialsForProvider(ch))
}

func TestHasCredentialsForProvider_NonOpenaiNoCreds(t *testing.T) {
	ch := &ent.Channel{
		Type:        channel.TypeClaudecode,
		Credentials: objects.ChannelCredentials{},
	}
	require.False(t, hasCredentialsForProvider(ch))
}

func TestGetProviderType_OpenaiWithWaferURLPort(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://pass.wafer.ai:443",
	}
	result := svc.getProviderType(ch)
	require.Equal(t, "wafer", result)
}

func TestGetProviderType_OpenaiWithSyntheticURLPort(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	ch := &ent.Channel{
		Type:    channel.TypeOpenaiResponses,
		BaseURL: "https://api.synthetic.new:443",
	}
	result := svc.getProviderType(ch)
	require.Equal(t, "synthetic", result)
}

func TestGetProviderType_OpenaiWithNeuralWattURLPort(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.neuralwatt.com:443",
	}
	result := svc.getProviderType(ch)
	require.Equal(t, "neuralwatt", result)
}

func TestGetProviderType_OpenaiWithFalsePositiveURL(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}

	ch := &ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://evilwafer.ai",
	}
	result := svc.getProviderType(ch)
	require.Equal(t, "", result)
}
