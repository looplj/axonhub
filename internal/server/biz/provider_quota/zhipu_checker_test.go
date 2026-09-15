package provider_quota

import (
	"context"
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

func TestZhipu_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "https://open.bigmodel.cn/api/monitor/usage/quota/limit", req.URL.String())
			require.Equal(t, "test-key", req.Header.Get("Authorization"))
			require.Equal(t, "en-US,en", req.Header.Get("Accept-Language"))

			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 25.5, "nextResetTime": 1784322000000},
						{"type": "TOKENS_LIMIT", "percentage": 10.0, "nextResetTime": 1784476800000},
						{"type": "TIME_LIMIT", "percentage": 50.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "zhipu", quota.ProviderType)
	require.Len(t, quota.Limits, 2)

	// First window (five_hour): 25.5% used
	require.InDelta(t, 0.255, quota.Limits[0].UsageRatio, 0.001)
	require.Equal(t, "available", quota.Limits[0].Status)

	// Second window (weekly_limit): 10.0% used
	require.InDelta(t, 0.10, quota.Limits[1].UsageRatio, 0.001)
	require.Equal(t, "available", quota.Limits[1].Status)

	rows, ok := quota.RawData["rows"].([]zhipuWindowRow)
	require.True(t, ok)
	require.Len(t, rows, 2)
	require.Equal(t, "five_hour", rows[0].Window)
	require.Equal(t, "weekly_limit", rows[1].Window)
	require.Equal(t, "standard", quota.RawData["level"])
}

func TestZhipu_CheckQuota_SingleWindow(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "basic",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 5.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "available", quota.Limits[0].Status)
	require.InDelta(t, 0.05, quota.Limits[0].UsageRatio, 0.001)
}

func TestZhipu_CheckQuota_Warning(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 90.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
}

func TestZhipu_CheckQuota_Exhausted(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 100.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestZhipu_CheckQuota_APIError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"success": false,
				"code": 500,
				"msg": "当前用户不存在coding plan"
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "当前用户不存在coding plan")
}

func TestZhipu_CheckQuota_MissingAPIKey(t *testing.T) {
	checker := NewZhipuQuotaChecker(nil)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{Type: channel.TypeZhipu})
	require.ErrorContains(t, err, "channel has no API key")
}

func TestZhipu_CheckQuota_APIKeysFallback(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "fallback-key", req.Header.Get("Authorization"))

			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 15.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{
			APIKey:  "",
			APIKeys: []string{"fallback-key"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestZhipu_CheckQuota_APIKeysFallbackSkipsBlankEntries(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "fallback-key", req.Header.Get("Authorization"))

			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 15.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{
			APIKey:  "",
			APIKeys: []string{"", "  ", "fallback-key"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestZhipu_CheckQuota_MalformedJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`not json`)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse zhipu quota response")
}

func TestZhipu_CheckQuota_NoData(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"success": true, "code": 200}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no data")
}

func TestZhipu_CheckQuota_NoTokensLimit(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TIME_LIMIT", "percentage": 50.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no TOKENS_LIMIT")
}

func TestZhipu_CheckQuota_NoResetIsFiveHour(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Weekly returned first but has a reset time; the entry without reset
			// must be the 5-hour bucket regardless of API order.
			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 96.0, "nextResetTime": 1784541710987},
						{"type": "TOKENS_LIMIT", "percentage": 0.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)

	rows, ok := quota.RawData["rows"].([]zhipuWindowRow)
	require.True(t, ok)
	// The no-reset entry is five_hour, even though it came second in the response.
	require.Equal(t, "five_hour", rows[0].Window)
	require.InDelta(t, 0.0, rows[0].UsedPercent, 0.001)
	require.Nil(t, rows[0].ResetAt)
	require.Equal(t, "weekly_limit", rows[1].Window)
	require.InDelta(t, 96.0, rows[1].UsedPercent, 0.001)
	require.NotNil(t, rows[1].ResetAt)
}

func TestZhipu_CheckQuota_BothHaveResetTime(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Both windows have reset time — mirrors real API when 5h has usage.
			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 1.0, "nextResetTime": 1784384374404},
						{"type": "TOKENS_LIMIT", "percentage": 96.0, "nextResetTime": 1784541710987}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)

	rows, ok := quota.RawData["rows"].([]zhipuWindowRow)
	require.True(t, ok)
	// First entry is five_hour, both have reset times.
	require.Equal(t, "five_hour", rows[0].Window)
	require.InDelta(t, 1.0, rows[0].UsedPercent, 0.001)
	require.NotNil(t, rows[0].ResetAt)
	require.Equal(t, "weekly_limit", rows[1].Window)
	require.InDelta(t, 96.0, rows[1].UsedPercent, 0.001)
	require.NotNil(t, rows[1].ResetAt)
}

func TestZhipu_SupportsChannel(t *testing.T) {
	checker := NewZhipuQuotaChecker(nil)

	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeZhipu}))
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeZhipuAnthropic}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeZai}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeZaiAnthropic}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai}))
}

func TestBuildZhipuQuotaURL(t *testing.T) {
	require.Equal(t, "https://open.bigmodel.cn/api/monitor/usage/quota/limit", buildZhipuQuotaURL("https://open.bigmodel.cn/api/paas/v4"))
	require.Equal(t, "https://open.bigmodel.cn/api/monitor/usage/quota/limit", buildZhipuQuotaURL("https://open.bigmodel.cn/api/anthropic"))
	require.Equal(t, "https://open.bigmodel.cn/api/monitor/usage/quota/limit", buildZhipuQuotaURL(""))
	require.Equal(t, "https://open.bigmodel.cn/api/monitor/usage/quota/limit", buildZhipuQuotaURL("not a URL"))
}

func TestZhipu_CheckQuota_WithResetTime(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 50.0, "nextResetTime": 4087947600000},
						{"type": "TOKENS_LIMIT", "percentage": 30.0, "nextResetTime": 4088534400000}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.NotNil(t, quota.NextResetAt)
	// Earlier reset time
	require.Equal(t, int64(4087947600), quota.NextResetAt.Unix())
}

func TestZhipu_CheckQuota_OverallStatusWorst(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 50.0},
						{"type": "TOKENS_LIMIT", "percentage": 95.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZhipuQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	// five_hour=available, weekly=warning → overall=warning
	require.Equal(t, "warning", quota.Status)
}

// zhipuCreditLimitPayload builds the GLM coding plan payload shape, where every
// window is a CREDIT_LIMIT carrying its own unit/number pair.
func zhipuCreditLimitPayload(level string, fiveHourPercent, weeklyPercent float64) string {
	return fmt.Sprintf(`{"success":true,"code":200,"data":{"level":%q,"limits":[
		{"type":"CREDIT_LIMIT","percentage":%v,"unit":3,"number":5,"usage":28000,"currentValue":2896,"remaining":25103,"nextResetTime":4102731600000},
		{"type":"CREDIT_LIMIT","percentage":%v,"unit":6,"number":1,"usage":140000,"currentValue":140019,"remaining":0,"nextResetTime":4103300000000}]}}`, level, fiveHourPercent, weeklyPercent)
}

// zhipuKeyedHTTPClient answers every request with the payload registered for
// the Authorization header the checker used.
func zhipuKeyedHTTPClient(payloads map[string]string) *httpclient.HttpClient {
	return httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, ok := payloads[req.Header.Get("Authorization")]
			if !ok {
				body = `{"success":false,"code":401,"msg":"Authentication parameter not received"}`
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})
}

func TestZhipu_CheckQuota_CreditLimitCodingPlan(t *testing.T) {
	checker := NewZhipuQuotaChecker(zhipuKeyedHTTPClient(map[string]string{
		"coding-plan-key": zhipuCreditLimitPayload("max", 10, 100),
	}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKey: "coding-plan-key"},
	})
	require.NoError(t, err)

	// The weekly window is at 100%, so the single-key channel is exhausted.
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	require.Equal(t, "max", quota.RawData["level"])

	rows, ok := quota.RawData["rows"].([]zhipuWindowRow)
	require.True(t, ok)
	require.Len(t, rows, 2)

	require.Equal(t, zhipuWindowFiveHour, rows[0].Window)
	require.InDelta(t, 10.0, rows[0].UsedPercent, 0.001)
	require.Equal(t, "available", rows[0].Status)
	require.NotNil(t, rows[0].Usage)
	require.Equal(t, int64(28000), *rows[0].Usage)
	require.NotNil(t, rows[0].Remaining)
	require.Equal(t, int64(25103), *rows[0].Remaining)
	require.NotNil(t, rows[0].ResetAt)

	require.Equal(t, zhipuWindowWeekly, rows[1].Window)
	require.InDelta(t, 100.0, rows[1].UsedPercent, 0.001)
	require.Equal(t, "exhausted", rows[1].Status)

	require.Len(t, quota.Limits, 2)
	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
	require.Equal(t, QuotaWindowWeekly, quota.Limits[1].Window)
}

func TestZhipu_CheckQuota_MultipleKeysReportEveryAccount(t *testing.T) {
	checker := NewZhipuQuotaChecker(zhipuKeyedHTTPClient(map[string]string{
		"aaaa-account-0001": zhipuCreditLimitPayload("max", 10, 100),
		"bbbb-account-0002": zhipuCreditLimitPayload("max", 16, 31),
	}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-account-0001", "bbbb-account-0002"}},
	})
	require.NoError(t, err)

	// One account is exhausted but the other still has quota, so the channel
	// stays available instead of being dropped from routing.
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)

	accounts, ok := quota.RawData["accounts"].([]zhipuAccountQuota)
	require.True(t, ok)
	require.Len(t, accounts, 2)
	require.Equal(t, "0001", accounts[0].Suffix)
	require.Equal(t, "exhausted", accounts[0].Status)
	require.Len(t, accounts[0].Rows, 2)
	require.Equal(t, "0002", accounts[1].Suffix)
	require.Equal(t, "available", accounts[1].Status)
	require.NotEmpty(t, accounts[0].Ref)
	require.NotEqual(t, accounts[0].Ref, accounts[1].Ref)

	// The channel keeps one binding limit per account inside one availability
	// group so the routing evaluator ORs the accounts.
	require.Len(t, quota.Limits, 2)
	require.Equal(t, zhipuAccountsAvailabilityGroup, quota.Limits[0].AvailabilityGroup)
	require.Equal(t, zhipuAccountsAvailabilityGroup, quota.Limits[1].AvailabilityGroup)
	require.Equal(t, QuotaWindowWeekly, quota.Limits[0].Window)
	require.Equal(t, "exhausted", quota.Limits[0].Status)
	require.Equal(t, "0001", quota.Limits[0].Account)
	require.Equal(t, "available", quota.Limits[1].Status)
	require.Equal(t, "0002", quota.Limits[1].Account)
	require.InDelta(t, 0.31, quota.Limits[1].UsageRatio, 0.001)

	// The fallback rows stay available for the badge and the channel list.
	rows, ok := quota.RawData["rows"].([]zhipuWindowRow)
	require.True(t, ok)
	require.Len(t, rows, 2)
	require.InDelta(t, 16.0, rows[0].UsedPercent, 0.001)
	require.InDelta(t, 100.0, rows[1].UsedPercent, 0.001)

	state, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingOpen, state, reason)
}

func TestZhipu_CheckQuota_MultipleKeysAllExhausted(t *testing.T) {
	checker := NewZhipuQuotaChecker(zhipuKeyedHTTPClient(map[string]string{
		"aaaa-account-0001": zhipuCreditLimitPayload("max", 10, 100),
		"bbbb-account-0002": zhipuCreditLimitPayload("max", 0, 100),
	}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-account-0001", "bbbb-account-0002"}},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)

	state, _ := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingExhausted, state)
}

func TestZhipu_CheckQuota_DisabledAccountKeepsRowsWithoutCapacity(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)

	checker := NewZhipuQuotaChecker(zhipuKeyedHTTPClient(map[string]string{
		"aaaa-account-0001": zhipuCreditLimitPayload("max", 0, 100),
		"bbbb-account-0002": zhipuCreditLimitPayload("max", 16, 31),
	}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-account-0001", "bbbb-account-0002"}},
		DisabledAPIKeys: []objects.DisabledAPIKey{{
			Key:        "aaaa-account-0001",
			DisabledAt: time.Now(),
			ErrorCode:  429,
			Reason:     "rate limited",
			ExpiresAt:  &expiresAt,
		}},
	})
	require.NoError(t, err)

	accounts, ok := quota.RawData["accounts"].([]zhipuAccountQuota)
	require.True(t, ok)
	require.Len(t, accounts, 2)

	// Serving keys are checked first, so parked keys can never hide them.
	require.Equal(t, "0002", accounts[0].Suffix)
	require.False(t, accounts[0].Disabled)
	require.Equal(t, "0001", accounts[1].Suffix)
	require.True(t, accounts[1].Disabled)
	require.Len(t, accounts[1].Rows, 2, "a disabled key still reports what the provider says")

	// Only the serving account contributes a limit.
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "0002", quota.Limits[0].Account)
	require.Equal(t, "available", quota.Status)
}

func TestZhipu_CheckQuota_OneAccountFailureKeepsTheRest(t *testing.T) {
	checker := NewZhipuQuotaChecker(zhipuKeyedHTTPClient(map[string]string{
		"bbbb-account-0002": zhipuCreditLimitPayload("max", 16, 31),
	}))

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZhipu,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"aaaa-account-0001", "bbbb-account-0002"}},
	})
	require.NoError(t, err)

	accounts, ok := quota.RawData["accounts"].([]zhipuAccountQuota)
	require.True(t, ok)
	require.Len(t, accounts, 2)
	require.NotEmpty(t, accounts[0].Error)
	require.Empty(t, accounts[1].Error)

	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "0002", quota.Limits[0].Account)
}
