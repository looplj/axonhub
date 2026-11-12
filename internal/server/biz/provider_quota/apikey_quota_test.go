// 职责: 覆盖逐 key 额度采集与聚合的共享契约。
// 关联文件:
//
//	internal/server/biz/provider_quota/apikey_quota.go
//	internal/server/biz/provider_quota/normalization.go
//	internal/server/biz/provider_quota/opencode_go_checker.go
//	internal/server/biz/provider_quota/zhipu_checker.go
//	internal/server/biz/provider_quota/deepseek_checker.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	断言行为（状态、可用性、窗口选择、金额字段），不要断言 JSON 字段数量。
package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

// opencodeUsagePayload builds one OpenCode Go usage body. A nil percent for a
// window omits it, so a partial payload stays expressible.
func opencodeUsagePayload(rollingAt time.Time, rolling, weekly, monthly float64) string {
	return fmt.Sprintf(`{"usage":{
		"rolling":{"percent":%v,"resetsAt":%q},
		"weekly":{"percent":%v,"resetsAt":%q},
		"monthly":{"percent":%v,"resetsAt":%q}
	}}`,
		rolling, rollingAt.Format(time.RFC3339),
		weekly, rollingAt.Add(48*time.Hour).Format(time.RFC3339),
		monthly, rollingAt.Add(20*24*time.Hour).Format(time.RFC3339),
	)
}

// opencodeKeyedHTTPClient answers each configured key with its own body and
// fails the keys registered in fail with a 401.
func opencodeKeyedHTTPClient(payloads map[string]string, fail map[string]bool) *httpclient.HttpClient {
	return httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			key := req.Header.Get("Authorization")
			if fail[key] {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"unauthorized"}}`)),
				}, nil
			}

			body, ok := payloads[key]
			if !ok {
				return nil, fmt.Errorf("unexpected API key %q", key)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})
}

func TestAPIKeyQuota_OpenCodeGoCollectsEveryKey(t *testing.T) {
	resetAt := time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC)
	client := opencodeKeyedHTTPClient(map[string]string{
		"Bearer aaaa-aaaa-1001": opencodeUsagePayload(resetAt, 100, 10, 20),
		"Bearer bbbb-bbbb-2002": opencodeUsagePayload(resetAt, 12, 30, 35),
	}, nil)

	checker := NewOpenCodeGoQuotaChecker(client)
	checker.now = func() time.Time { return resetAt.Add(-5 * time.Hour) }

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-aaaa-1001", "bbbb-bbbb-2002"}},
	})
	require.NoError(t, err)

	require.Len(t, quota.APIKeyQuotas, 2)
	first, second := quota.APIKeyQuotas[0], quota.APIKeyQuotas[1]
	require.Equal(t, 1, first.Index)
	require.Equal(t, 2, second.Index)
	require.Equal(t, "1001", first.Suffix)
	require.Equal(t, "2002", second.Suffix)
	require.Len(t, first.Ref, 64, "the ref is the full SHA-256 digest")
	require.NotEqual(t, first.Ref, second.Ref)
	require.NotContains(t, first.Ref, "1001", "the ref must not expose the key")

	// Every window of every account is reported.
	require.Len(t, first.Data.Limits, 3)
	require.Len(t, second.Data.Limits, 3)
	require.Equal(t, "exhausted", first.Data.Status, "one exhausted window exhausts the key")
	require.Equal(t, QuotaWindow5h, first.Data.Limits[0].Window)
	require.Equal(t, QuotaWindowWeekly, first.Data.Limits[1].Window)
	require.Equal(t, QuotaWindowMonthly, first.Data.Limits[2].Window)
	require.Equal(t, "available", second.Data.Status)
	require.Nil(t, first.Data.APIKeyQuotas, "an account result never nests account rows")

	// The channel stays available because the second key still has capacity.
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)

	// One binding limit per enabled key inside a single availability group.
	require.Len(t, quota.Limits, 2)
	for _, limit := range quota.Limits {
		require.Equal(t, QuotaLimitTypeToken, limit.Type)
		require.Equal(t, apiKeyAccountsAvailabilityGroup, limit.AvailabilityGroup)
	}
	require.Equal(t, "#1 ****1001", quota.Limits[0].Account)
	require.Equal(t, "#2 ****2002", quota.Limits[1].Account)
	require.Equal(t, "exhausted", quota.Limits[0].Status)
	require.Equal(t, "available", quota.Limits[1].Status)
	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window, "the exhausted window is the binding one")

	state, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingOpen, state, reason)

	// A multi-key channel does not masquerade as its first account.
	require.Nil(t, quota.RawData)
}

func TestAPIKeyQuota_SingleKeyKeepsWindowShape(t *testing.T) {
	resetAt := time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC)
	checker := NewOpenCodeGoQuotaChecker(opencodeKeyedHTTPClient(map[string]string{
		"Bearer only-one-9009": opencodeUsagePayload(resetAt, 10, 20, 30),
	}, nil))
	checker.now = func() time.Time { return resetAt.Add(-5 * time.Hour) }

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "only-one-9009"},
	})
	require.NoError(t, err)

	require.Len(t, quota.APIKeyQuotas, 1)
	require.Len(t, quota.Limits, 3, "a lone key keeps its per-window channel limits")
	for _, limit := range quota.Limits {
		require.Empty(t, limit.Account, "single-key limits keep the account-less shape")
		require.Empty(t, limit.AvailabilityGroup)
	}
	require.NotNil(t, quota.RawData["windows"], "the lone key keeps its provider payload")
}

func TestAPIKeyQuota_AllKeysExhausted(t *testing.T) {
	resetAt := time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC)
	checker := NewOpenCodeGoQuotaChecker(opencodeKeyedHTTPClient(map[string]string{
		"Bearer aaaa-aaaa-1001": opencodeUsagePayload(resetAt, 100, 100, 10),
		"Bearer bbbb-bbbb-2002": opencodeUsagePayload(resetAt, 10, 50, 100),
	}, nil))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-aaaa-1001", "bbbb-bbbb-2002"}},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)

	state, _ := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingExhausted, state)
}

func TestAPIKeyQuota_PartialFailureKeepsHealthyKey(t *testing.T) {
	resetAt := time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC)
	checker := NewOpenCodeGoQuotaChecker(opencodeKeyedHTTPClient(map[string]string{
		"Bearer bbbb-bbbb-2002": opencodeUsagePayload(resetAt, 12, 30, 35),
	}, map[string]bool{"Bearer aaaa-aaaa-1001": true}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-aaaa-1001", "bbbb-bbbb-2002"}},
	})
	require.NoError(t, err, "a partial failure is not a channel-wide failure")

	failed := quota.APIKeyQuotas[0]
	require.Equal(t, APIKeyQuotaErrorCheckFailed, failed.ErrorCode)
	require.Equal(t, "unknown", failed.Data.Status)
	require.Empty(t, failed.Data.Limits, "a failed key invents no window")
	require.Empty(t, failed.Data.RawData)

	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.Limits, 2)

	// The unreadable alternative is unknown capacity, not an exhausted one.
	require.Equal(t, "unknown", quota.Limits[0].Status)
	require.Equal(t, QuotaWindowPrimary, quota.Limits[0].Window)
	require.False(t, quota.Limits[0].Ready)

	state, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingOpen, state, reason)
}

func TestAPIKeyQuota_ExhaustedPlusUnknownStaysUnknown(t *testing.T) {
	resetAt := time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC)
	checker := NewOpenCodeGoQuotaChecker(opencodeKeyedHTTPClient(map[string]string{
		"Bearer aaaa-aaaa-1001": opencodeUsagePayload(resetAt, 100, 100, 100),
	}, map[string]bool{"Bearer bbbb-bbbb-2002": true}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-aaaa-1001", "bbbb-bbbb-2002"}},
	})
	require.NoError(t, err)
	require.Equal(t, "unknown", quota.Status, "an unreadable alternative is unknown, never exhausted")

	// The service normalizes every successful result once more; the account
	// derived status has to survive that call.
	renormalized := NormalizeQuotaData(quota)
	require.Equal(t, "unknown", renormalized.Status)
	require.False(t, renormalized.Ready)

	state, _ := EvaluateQuotaRouting(renormalized.Limits, renormalized.Status, QuotaLimitTypeToken, time.Now())
	require.NotEqual(t, RoutingExhausted, state)
}

func TestAPIKeyQuota_AllFailuresReturnRowsAndError(t *testing.T) {
	checker := NewOpenCodeGoQuotaChecker(opencodeKeyedHTTPClient(map[string]string{}, map[string]bool{
		"Bearer aaaa-aaaa-1001": true,
		"Bearer bbbb-bbbb-2002": true,
	}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-aaaa-1001", "bbbb-bbbb-2002"}},
	})
	require.Error(t, err, "an all-key failure keeps the caller's error backoff")
	require.Contains(t, err.Error(), "failed for all 2 keys")

	require.Len(t, quota.APIKeyQuotas, 2, "the rows still say which keys failed")
	for _, account := range quota.APIKeyQuotas {
		require.Equal(t, APIKeyQuotaErrorCheckFailed, account.ErrorCode)
	}
	require.Equal(t, "unknown", quota.Status)
	for _, account := range quota.APIKeyQuotas {
		require.Empty(t, account.Data.RawData, "a failed account keeps no upstream payload")
	}
}

func TestAPIKeyQuota_DuplicateBlankAndLegacyKeys(t *testing.T) {
	var requested []string

	client := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requested = append(requested, req.Header.Get("Authorization"))
			body := opencodeUsagePayload(time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC), 10, 10, 10)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	quota, err := NewOpenCodeGoQuotaChecker(client).CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{
			APIKey:  "legacy-1111",
			APIKeys: []string{"legacy-1111", "", "   ", "second-2222"},
		},
	})
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"Bearer legacy-1111", "Bearer second-2222"}, requested)
	require.Len(t, quota.APIKeyQuotas, 2)
	require.Equal(t, 1, quota.APIKeyQuotas[0].Index, "the legacy key keeps the first ordinal")
	require.Equal(t, 5, quota.APIKeyQuotas[1].Index, "blank and duplicate entries still consume an ordinal")
}

func TestAPIKeyQuota_CollidingSuffixesAndShortKeys(t *testing.T) {
	checker := NewOpenCodeGoQuotaChecker(opencodeKeyedHTTPClient(map[string]string{
		"Bearer aaaa-1234": opencodeUsagePayload(time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC), 10, 10, 10),
		"Bearer bbbb-1234": opencodeUsagePayload(time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC), 20, 20, 20),
		"Bearer abc":       opencodeUsagePayload(time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC), 30, 30, 30),
	}, nil))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-1234", "bbbb-1234", "abc"}},
	})
	require.NoError(t, err)

	require.Equal(t, quota.APIKeyQuotas[0].Suffix, quota.APIKeyQuotas[1].Suffix)
	require.NotEqual(t, quota.Limits[0].Account, quota.Limits[1].Account, "the index keeps colliding suffixes apart")
	require.Equal(t, "", quota.APIKeyQuotas[2].Suffix, "a short key gets no suffix")
	require.Equal(t, "#3", quota.Limits[2].Account)
}

func TestAPIKeyQuota_ManualDisabledFundedKeyDoesNotKeepChannelRoutable(t *testing.T) {
	resetAt := time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC)
	checker := NewOpenCodeGoQuotaChecker(opencodeKeyedHTTPClient(map[string]string{
		"Bearer funded-1001":    opencodeUsagePayload(resetAt, 5, 5, 5),
		"Bearer exhausted-2002": opencodeUsagePayload(resetAt, 100, 5, 5),
	}, nil))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"funded-1001", "exhausted-2002"}},
		DisabledAPIKeys: []objects.DisabledAPIKey{{
			Key:        "funded-1001",
			DisabledAt: time.Now(),
			ErrorCode:  429,
			Reason:     "rate limited",
		}},
	})
	require.NoError(t, err)

	require.True(t, quota.APIKeyQuotas[0].Disabled)
	require.False(t, quota.APIKeyQuotas[0].QuotaDisabled, "a manual disable owns the record")
	require.Equal(t, "available", quota.APIKeyQuotas[0].Data.Status, "a disabled key is still reported")

	require.Equal(t, "exhausted", quota.Status, "only enabled keys decide capacity")
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "#2 ****2002", quota.Limits[0].Account)

	state, _ := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingExhausted, state)
}

func TestAPIKeyQuota_ZhipuKeepsPerKeyCredits(t *testing.T) {
	checker := NewZhipuQuotaChecker(zhipuKeyedHTTPClient(map[string]string{
		"aaaa-account-0001": zhipuCreditLimitPayload("max", 10, 100),
		"bbbb-account-0002": zhipuCreditLimitPayload("max", 16, 31),
	}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-account-0001", "bbbb-account-0002"}},
	})
	require.NoError(t, err)

	require.Equal(t, "zhipu", quota.ProviderType)
	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.APIKeyQuotas, 2)

	first := quota.APIKeyQuotas[0]
	require.Equal(t, "exhausted", first.Data.Status)
	require.Equal(t, "max", first.Data.RawData["level"])
	rows, ok := first.Data.RawData["rows"].([]zhipuWindowRow)
	require.True(t, ok)
	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].Usage)
	require.NotNil(t, rows[0].Remaining, "the provider credit amounts stay per key")
	require.Len(t, first.Data.Limits, 2)

	require.Len(t, quota.Limits, 2)
	require.Equal(t, apiKeyAccountsAvailabilityGroup, quota.Limits[0].AvailabilityGroup)
	require.Equal(t, "#1 ****0001", quota.Limits[0].Account)
	require.Equal(t, "exhausted", quota.Limits[0].Status)
	require.Equal(t, QuotaWindowWeekly, quota.Limits[0].Window)

	state, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingOpen, state, reason)
}

func TestAPIKeyQuota_ZaiSharesZhipuBehavior(t *testing.T) {
	checker := NewZaiQuotaChecker(zhipuKeyedHTTPClient(map[string]string{
		"aaaa-account-0001": zhipuCreditLimitPayload("pro", 10, 10),
	}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZai,
		Credentials: objects.ChannelCredentials{APIKey: "aaaa-account-0001"},
	})
	require.NoError(t, err)
	require.Equal(t, "zai", quota.ProviderType)
	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.APIKeyQuotas, 1)
	require.Len(t, quota.Limits, 2, "a lone key keeps both windows")
}

func TestAPIKeyQuota_DeepSeekNeverInventsTokenLimits(t *testing.T) {
	client := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			key := req.Header.Get("Authorization")

			var body string
			switch key {
			case "Bearer empty-0001":
				body = `{"is_available": false, "balance_infos": [{"currency":"CNY","total_balance":"0.00"}]}`
			default:
				return nil, fmt.Errorf("unexpected API key %q", key)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	quota, err := NewDeepSeekQuotaChecker(client).CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeDeepseek,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"empty-0001", "unreadable-0002"}},
	})
	require.NoError(t, err)
	require.Equal(t, "unknown", quota.Status, "one empty and one unreadable key is unknown capacity")
	require.Nil(t, quota.Limits, "DeepSeek publishes no token limit, not even an unknown one")

	state, _ := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingUnknown, state)

	require.Len(t, quota.Balances, 1)
	require.Equal(t, "#1 ****0001", quota.Balances[0].Account)
}

func TestAPIKeyQuota_BindingLimitKeepsPeriodStartForBackpressure(t *testing.T) {
	now := time.Date(2099, 6, 25, 10, 0, 0, 0, time.UTC)
	// A rolling window that resets in 4h has used 20% of its elapsed time.
	resetAt := now.Add(4 * time.Hour)

	checker := NewOpenCodeGoQuotaChecker(opencodeKeyedHTTPClient(map[string]string{
		"Bearer first-1001":  opencodeUsagePayload(resetAt, 60, 10, 10),
		"Bearer second-2002": opencodeUsagePayload(resetAt, 60, 10, 10),
	}, nil))
	checker.now = func() time.Time { return now }

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"first-1001", "second-2002"}},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)

	for _, limit := range quota.Limits {
		require.NotNil(t, limit.PeriodStart, "the binding limit keeps its period bounds")
		require.NotNil(t, limit.NextResetAt)
		require.Equal(t, now.Add(-time.Hour).Unix(), limit.PeriodStart.Unix())
		require.Nil(t, limit.PeriodCost, "usage-log spend is channel scoped, never per key")
		require.Nil(t, limit.PeriodQuota)
	}

	state, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, now)
	require.Equal(t, RoutingStickyOnly, state)
	require.Equal(t, "window_pressure", reason)
}

func TestAPIKeyQuota_NoKeys(t *testing.T) {
	quota, err := NewOpenCodeGoQuotaChecker(nil).CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeOpencodeGo,
	})
	require.ErrorContains(t, err, "channel has no API key")
	require.Empty(t, quota.APIKeyQuotas)
}

func TestAPIKeyQuota_SerializedAccountsExposeNoKey(t *testing.T) {
	quota, err := NewOpenCodeGoQuotaChecker(opencodeKeyedHTTPClient(map[string]string{
		"Bearer secret-value-1001": opencodeUsagePayload(time.Date(2099, 6, 25, 15, 0, 0, 0, time.UTC), 10, 10, 10),
	}, nil)).CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"secret-value-1001"}},
	})
	require.NoError(t, err)

	encoded, err := json.Marshal(quota)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret-value-1001", "the in-process result never serializes the key")
	require.NotContains(t, string(encoded), "Authorization")
}
