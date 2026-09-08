package provider_quota

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestQuotaCheckers_NormalizeReportedLimits(t *testing.T) {
	// Given a successful Antigravity response with two model quota limits.
	body := []byte(`{
		"models": {
			"gemini-3-pro": {"displayName":"Gemini 3 Pro","quotaInfo":{"remainingFraction":0.75,"resetTime":"2099-09-04T08:00:00Z"}},
			"claude-sonnet": {"displayName":"Claude Sonnet","quotaInfo":{"remainingFraction":0.1,"resetTime":"2099-09-04T08:00:00Z"}}
		}
	}`)

	// When the checker parses the provider response.
	quota, err := parseAntigravityQuota(body)

	// Then every provider-reported model limit is available in normalized data.
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)
	require.Equal(t, "claude-sonnet", quota.Limits[0].Window)
	require.Equal(t, "gemini-3-pro", quota.Limits[1].Window)
	require.InDelta(t, 0.9, quota.Limits[0].UsageRatio, 1e-9)
	require.InDelta(t, 0.25, quota.Limits[1].UsageRatio, 1e-9)
	assertNormalizedLimitContract(t, quota)
}

func TestQuotaCheckers_KeepZenmuxMonthlyQuotaInRawData(t *testing.T) {
	// Given a successful ZenMux response with 5-hour, 7-day, and monthly metadata.
	body := []byte(`{
		"success": true,
		"data": {
			"account_status": "healthy",
			"quota_5_hour": {"usage_percentage": 0.25, "resets_at": "2099-09-04T15:00:00Z"},
			"quota_7_day": {"usage_percentage": 0.5, "resets_at": "2099-09-11T10:00:00Z"},
			"quota_monthly": {"max_flows": 1000, "max_value_usd": 50}
		}
	}`)

	// When the checker parses the provider response.
	quota, err := parseZenmuxQuotaResponse(body)

	// Then only the actual 5-hour and 7-day windows are available in normalized data.
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)
	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
	require.Equal(t, QuotaWindow7d, quota.Limits[1].Window)
	require.Contains(t, quota.RawData, "quota_monthly")
	assertNormalizedLimitContract(t, quota)
}

func TestQuotaCheckers_MalformedWindows(t *testing.T) {
	// Given one malformed usage ratio and one valid provider window.
	checker := &OpenCodeGoQuotaChecker{now: func() time.Time {
		return time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	}}
	body := []byte(`{"usage":{
		"rolling":{"percent":-5,"resetsAt":"2026-09-04T15:00:00Z"},
		"weekly":{"percent":25,"resetsAt":"2026-09-11T10:00:00Z"}
	}}`)

	// When the checker parses the response.
	quota, err := checker.parseResponse(body)

	// Then the malformed window is omitted without affecting the valid window.
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, QuotaWindowWeekly, quota.Limits[0].Window)
	require.InDelta(t, 0.25, quota.Limits[0].UsageRatio, 1e-9)
}

func TestQuotaCheckers_NormalizeDuplicateLimits(t *testing.T) {
	// Given duplicate limits with the same type/window and different valid resets.
	earliestReset := time.Date(2099, 9, 4, 12, 0, 0, 0, time.UTC)
	laterReset := earliestReset.Add(time.Hour)
	periodStart := earliestReset.Add(-5 * time.Hour)

	// When the common contract normalizes them.
	quota := NormalizeQuotaData(QuotaData{
		Status: "unknown",
		Limits: []QuotaLimitStatus{
			{
				Type:        QuotaLimitTypeToken,
				Window:      QuotaWindow5h,
				UsageRatio:  0.25,
				Status:      "available",
				NextResetAt: &laterReset,
				PeriodStart: &periodStart,
			},
			{
				Type:        QuotaLimitTypeToken,
				Window:      QuotaWindow5h,
				UsageRatio:  0.9,
				Status:      "warning",
				NextResetAt: &earliestReset,
			},
		},
	})

	// Then the duplicate identity is merged without losing the earliest reset.
	require.Len(t, quota.Limits, 1)
	require.InDelta(t, 0.9, quota.Limits[0].UsageRatio, 1e-9)
	require.Equal(t, "warning", quota.Limits[0].Status)
	require.Equal(t, earliestReset, *quota.Limits[0].NextResetAt)
	require.Equal(t, periodStart, *quota.Limits[0].PeriodStart)
}

func TestQuotaCheckerCoverageMatrix(t *testing.T) {
	client := httpclient.NewHttpClient()
	checkers := map[string]QuotaChecker{
		"claudecode":       NewClaudeCodeQuotaChecker(client),
		"codex":            NewCodexQuotaChecker(client),
		"antigravity":      NewAntigravityQuotaChecker(client),
		"xai_subscription": NewXAISubscriptionQuotaChecker(client),
		"github_copilot":   NewGithubCopilotQuotaChecker(client),
		"nanogpt":          NewNanoGPTQuotaChecker(client),
		"zenmux":           NewZenmuxQuotaChecker(client),
		"cline":            NewClineQuotaChecker(client),
		"wafer":            NewWaferQuotaChecker(client),
		"synthetic":        NewSyntheticQuotaChecker(client),
		"neuralwatt":       NewNeuralWattQuotaChecker(client),
		"apertis":          NewApertisQuotaChecker(client),
		"opencode_go":      NewOpenCodeGoQuotaChecker(client),
		"kimi_code":        NewKimiCodeQuotaChecker(client),
		"minimax":          NewMinimaxQuotaChecker(client),
		"zhipu":            NewZhipuQuotaChecker(client),
		"charm_hyper":      NewCharmHyperQuotaChecker(client),
		"commandcode":      NewCommandCodeQuotaChecker(client),
	}

	relations := []struct {
		checker     string
		channelType channel.Type
		baseURL     string
		fixture     string
		limitCount  int
	}{
		{"claudecode", channel.TypeClaudecode, "", "claudecode_headers", 2},
		{"codex", channel.TypeCodex, "", "codex_windows", 2},
		{"antigravity", channel.TypeAntigravity, "", "antigravity_models", 2},
		{"xai_subscription", channel.TypeXaiSubscription, "", "xai_weekly_monthly", 2},
		{"github_copilot", channel.TypeGithubCopilot, "", "copilot_snapshots", 2},
		{"nanogpt", channel.TypeNanogpt, "", "nanogpt_windows", 3},
		{"nanogpt", channel.TypeNanogptResponses, "", "nanogpt_windows", 3},
		{"zenmux", channel.TypeZenmux, "", "zenmux_windows", 2},
		{"zenmux", channel.TypeZenmuxResponses, "", "zenmux_windows", 2},
		{"zenmux", channel.TypeZenmuxAnthropic, "", "zenmux_windows", 2},
		{"zenmux", channel.TypeZenmuxGemini, "", "zenmux_windows", 2},
		{"zenmux", channel.TypeZenmuxVideo, "", "zenmux_windows", 2},
		{"cline", channel.TypeCline, "", "cline_windows", 3},
		{"wafer", channel.TypeOpenai, "https://pass.wafer.ai", "wafer_cycle", 1},
		{"wafer", channel.TypeOpenaiResponses, "https://pass.wafer.ai", "wafer_cycle", 1},
		{"synthetic", channel.TypeOpenai, "https://api.synthetic.new", "synthetic_windows", 2},
		{"synthetic", channel.TypeOpenaiResponses, "https://api.synthetic.new", "synthetic_windows", 2},
		{"neuralwatt", channel.TypeOpenai, "https://api.neuralwatt.com", "neuralwatt_kwh", 1},
		{"neuralwatt", channel.TypeOpenaiResponses, "https://api.neuralwatt.com", "neuralwatt_kwh", 1},
		{"apertis", channel.TypeOpenai, "https://api.apertis.ai", "apertis_limits", 1},
		{"apertis", channel.TypeOpenaiResponses, "https://api.apertis.ai", "apertis_limits", 1},
		{"opencode_go", channel.TypeOpencodeGo, "", "opencode_go_windows", 3},
		{"opencode_go", channel.TypeOpencodeGoAnthropic, "", "opencode_go_windows", 3},
		{"kimi_code", channel.TypeMoonshotCoding, "", "kimi_windows", 2},
		{"minimax", channel.TypeMinimax, "", "minimax_windows", 2},
		{"minimax", channel.TypeMinimaxAnthropic, "", "minimax_windows", 2},
		{"zhipu", channel.TypeZhipu, "", "zhipu_windows", 2},
		{"zhipu", channel.TypeZhipuAnthropic, "", "zhipu_windows", 2},
		{"charm_hyper", channel.TypeOpenai, "https://hyper.charm.land", "charm_credits", 1},
		{"charm_hyper", channel.TypeOpenaiResponses, "https://hyper.charm.land", "charm_credits", 1},
		{"commandcode", channel.TypeCommandcode, "", "commandcode_windows", 2},
		{"commandcode", channel.TypeCommandcodeAnthropic, "", "commandcode_windows", 2},
	}

	entries := make([]map[string]any, 0, len(relations))
	for _, relation := range relations {
		checker, ok := checkers[relation.checker]
		require.True(t, ok, relation.checker)
		quotaChannel := &ent.Channel{Type: relation.channelType, BaseURL: relation.baseURL}
		if relation.checker == "commandcode" {
			quotaChannel.Settings = &objects.ChannelSettings{
				ProviderQuota: &objects.ChannelProviderQuotaSettings{
					CommandCode: &objects.CommandCodeQuotaSettings{AuthCookie: "fixture-cookie"},
				},
			}
		}
		require.True(t, checker.SupportsChannel(quotaChannel), relation.checker+"/"+string(relation.channelType))
		require.NotEmpty(t, relation.fixture)
		require.Positive(t, relation.limitCount)
		fixtureQuota, err := quotaCheckerFixture(relation.checker)
		require.NoError(t, err, relation.fixture)
		require.Len(t, fixtureQuota.Limits, relation.limitCount, relation.fixture)
		assertNormalizedLimitContract(t, fixtureQuota)
		identities := normalizedLimitIdentities(fixtureQuota)
		entries = append(entries, map[string]any{
			"checker":          relation.checker,
			"channel_types":    []string{string(relation.channelType)},
			"fixture":          relation.fixture,
			"limit_count":      relation.limitCount,
			"limit_identities": identities,
			"contract_passed":  true,
		})
	}

	if os.Getenv("AXONHUB_WRITE_QUOTA_EVIDENCE") != "1" {
		return
	}
	evidenceDir := filepath.Join("..", "..", "..", "..", ".omo", "evidence", "quota-display-protocols")
	require.NoError(t, os.MkdirAll(evidenceDir, 0o755))
	evidence, err := json.MarshalIndent(entries, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "task-2-provider-coverage.json"), evidence, 0o644))
}
