package biz

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestProviderQuotaService_RegistryCoverageMatrix(t *testing.T) {
	svc := &ProviderQuotaService{
		checkers:   make(map[string]provider_quota.QuotaChecker),
		httpClient: httpclient.NewHttpClient(),
	}
	svc.registerProviderQuotaSupport()

	expectedCheckers := []string{
		"apertis", "antigravity", "charm_hyper", "claudecode", "cline", "codex",
		"commandcode", "github_copilot", "kimi_code", "minimax", "nanogpt",
		"neuralwatt", "opencode_go", "synthetic", "wafer", "xai_subscription",
		"ollama", "zenmux", "zhipu",
	}
	require.ElementsMatch(t, expectedCheckers, mapKeys(svc.checkers))

	expectedRelations := []struct {
		checker     string
		channelType channel.Type
		baseURL     string
	}{
		{"claudecode", channel.TypeClaudecode, ""},
		{"codex", channel.TypeCodex, ""},
		{"antigravity", channel.TypeAntigravity, ""},
		{"xai_subscription", channel.TypeXaiSubscription, ""},
		{"github_copilot", channel.TypeGithubCopilot, ""},
		{"nanogpt", channel.TypeNanogpt, ""},
		{"nanogpt", channel.TypeNanogptResponses, ""},
		{"zenmux", channel.TypeZenmux, ""},
		{"zenmux", channel.TypeZenmuxResponses, ""},
		{"zenmux", channel.TypeZenmuxAnthropic, ""},
		{"zenmux", channel.TypeZenmuxGemini, ""},
		{"zenmux", channel.TypeZenmuxVideo, ""},
		{"cline", channel.TypeCline, ""},
		{"wafer", channel.TypeOpenai, "https://pass.wafer.ai"},
		{"wafer", channel.TypeOpenaiResponses, "https://pass.wafer.ai"},
		{"synthetic", channel.TypeOpenai, "https://api.synthetic.new"},
		{"synthetic", channel.TypeOpenaiResponses, "https://api.synthetic.new"},
		{"neuralwatt", channel.TypeOpenai, "https://api.neuralwatt.com"},
		{"neuralwatt", channel.TypeOpenaiResponses, "https://api.neuralwatt.com"},
		{"apertis", channel.TypeOpenai, "https://api.apertis.ai"},
		{"apertis", channel.TypeOpenaiResponses, "https://api.apertis.ai"},
		{"opencode_go", channel.TypeOpencodeGo, ""},
		{"opencode_go", channel.TypeOpencodeGoAnthropic, ""},
		{"kimi_code", channel.TypeMoonshotCoding, ""},
		{"minimax", channel.TypeMinimax, ""},
		{"minimax", channel.TypeMinimaxAnthropic, ""},
		{"zhipu", channel.TypeZhipu, ""},
		{"zhipu", channel.TypeZhipuAnthropic, ""},
		{"charm_hyper", channel.TypeOpenai, "https://hyper.charm.land"},
		{"charm_hyper", channel.TypeOpenaiResponses, "https://hyper.charm.land"},
		{"commandcode", channel.TypeCommandcode, ""},
		{"commandcode", channel.TypeCommandcodeAnthropic, ""},
		{"ollama", channel.TypeOllama, ""},
		{"ollama", channel.TypeOllamaAnthropic, ""},
	}

	for _, relation := range expectedRelations {
		key := fmt.Sprintf("%s/%s/%s", relation.checker, relation.channelType, relation.baseURL)
		quotaChannel := &ent.Channel{Type: relation.channelType, BaseURL: relation.baseURL}
		if relation.checker == "commandcode" {
			quotaChannel.Settings = &objects.ChannelSettings{
				ProviderQuota: &objects.ChannelProviderQuotaSettings{
					CommandCode: &objects.CommandCodeQuotaSettings{AuthCookie: "fixture-cookie"},
				},
			}
		}
		if relation.checker == "ollama" {
			quotaChannel.Settings = &objects.ChannelSettings{
				ProviderQuota: &objects.ChannelProviderQuotaSettings{
					Ollama: &objects.OllamaQuotaSettings{AuthCookie: "fixture-cookie"},
				},
			}
		}
		providerType := svc.getProviderType(quotaChannel)
		require.Equal(t, relation.checker, providerType, key)
		checker, ok := svc.checkers[providerType]
		require.True(t, ok, key)
		require.True(t, checker.SupportsChannel(quotaChannel), key)
	}

	productionTypes := make(map[channel.Type]struct{}, len(providerQuotaChannelTypes))
	for _, channelType := range providerQuotaChannelTypes {
		productionTypes[channelType] = struct{}{}
	}
	for _, relation := range expectedRelations {
		_, ok := productionTypes[relation.channelType]
		require.True(t, ok, "expected relation missing from production channel registry: %s", relation.channelType)
	}
	for _, channelType := range providerQuotaChannelTypes {
		found := false
		for _, relation := range expectedRelations {
			if relation.channelType == channelType {
				found = true
				break
			}
		}
		require.True(t, found, "production channel registry has no independent expected relation: %s", channelType)
	}
}

func mapKeys(checkers map[string]provider_quota.QuotaChecker) []string {
	keys := make([]string, 0, len(checkers))
	for key := range checkers {
		keys = append(keys, key)
	}
	return keys
}
