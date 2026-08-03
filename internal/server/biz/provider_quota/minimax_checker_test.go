package provider_quota

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestMinimax_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "https://www.minimaxi.com/v1/token_plan/remains", req.URL.String())
			require.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))

			body := `{
				"model_remains": [
					{
						"model_name": "general",
						"start_time": 1784304000000,
						"end_time": 1784322000000,
						"current_interval_status": 1,
						"current_interval_remaining_percent": 98,
						"current_interval_boost_permille": 0,
						"weekly_start_time": 1783872000000,
						"weekly_end_time": 1784476800000,
						"current_weekly_status": 1,
						"current_weekly_remaining_percent": 95,
						"weekly_boost_permille": 0
					},
					{
						"model_name": "video",
						"current_interval_status": 1,
						"current_interval_remaining_percent": 50,
						"current_weekly_status": 1,
						"current_weekly_remaining_percent": 50
					}
				],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "minimax", quota.ProviderType)
	require.Len(t, quota.Limits, 2) // 5h + weekly

	// 5h: used=2%, total=100%, bar=2%, ratio=0.02
	require.InDelta(t, 0.02, quota.Limits[0].UsageRatio, 0.001)
	require.Equal(t, "available", quota.Limits[0].Status)

	// Weekly: used=5%, total=100%, bar=5%, ratio=0.05
	require.InDelta(t, 0.05, quota.Limits[1].UsageRatio, 0.001)
	require.Equal(t, "available", quota.Limits[1].Status)

	rows, ok := quota.RawData["rows"].([]minimaxModelRow)
	require.True(t, ok)
	require.Len(t, rows, 1) // only "general"
	require.Equal(t, "general", rows[0].ModelName)
}

func TestMinimax_CheckQuota_WithBoost(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [{
					"model_name": "general",
					"start_time": 1784304000000,
					"end_time": 1784322000000,
					"current_interval_status": 1,
					"current_interval_remaining_percent": 100,
					"current_interval_boost_permille": 0,
					"weekly_start_time": 1783872000000,
					"weekly_end_time": 1784476800000,
					"current_weekly_status": 1,
					"current_weekly_remaining_percent": 97,
					"weekly_boost_permille": 1500
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.Limits, 2)

	// 5h: remaining=100, no boost → used=0, bar=0%, ratio=0
	require.InDelta(t, 0.0, quota.Limits[0].UsageRatio, 0.001)
	require.Equal(t, "available", quota.Limits[0].Status)

	// Weekly: remaining=97, boost=1500 → total=150, used=3, bar=3/150=2%, ratio=0.02
	require.InDelta(t, 0.02, quota.Limits[1].UsageRatio, 0.001)
	require.Equal(t, "available", quota.Limits[1].Status)

	rows, ok := quota.RawData["rows"].([]minimaxModelRow)
	require.True(t, ok)
	require.InDelta(t, 3.0, rows[0].WeeklyUsedPercent, 0.001)
	require.InDelta(t, 150.0, rows[0].WeeklyTotalPercent, 0.001)
	require.InDelta(t, 2.0, rows[0].WeeklyPercent, 0.001)
	require.Equal(t, 1500, rows[0].WeeklyBoostPermille)
}

func TestMinimax_CheckQuota_5hBoost(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [{
					"model_name": "general",
					"current_interval_status": 1,
					"current_interval_remaining_percent": 50,
					"current_interval_boost_permille": 2000,
					"current_weekly_status": 3,
					"current_weekly_remaining_percent": 100,
					"weekly_boost_permille": 0
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1) // weekly skipped (status=3)

	// 5h: remaining=50, boost=2000 → total=200, used=50, bar=50/200=25%, ratio=0.25
	require.InDelta(t, 0.25, quota.Limits[0].UsageRatio, 0.001)
	require.Equal(t, "available", quota.Limits[0].Status)

	rows, ok := quota.RawData["rows"].([]minimaxModelRow)
	require.True(t, ok)
	require.InDelta(t, 200.0, rows[0].IntervalTotalPercent, 0.001)
	require.InDelta(t, 25.0, rows[0].IntervalPercent, 0.001)
}

func TestMinimax_CheckQuota_WeeklyStatus3_SkipsWeekly(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [{
					"model_name": "general",
					"current_interval_status": 1,
					"current_interval_remaining_percent": 80,
					"current_weekly_status": 3,
					"current_weekly_remaining_percent": 100
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1) // only 5h, weekly skipped
	require.Equal(t, "available", quota.Limits[0].Status)

	rows, ok := quota.RawData["rows"].([]minimaxModelRow)
	require.True(t, ok)
	require.Equal(t, "", rows[0].WeeklyStatus)
}

func TestMinimax_CheckQuota_Warning(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [{
					"model_name": "general",
					"current_interval_status": 1,
					"current_interval_remaining_percent": 10,
					"current_weekly_status": 1,
					"current_weekly_remaining_percent": 90
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	// 5h: remaining=10 → used=90, ratio=0.9 → warning (>=0.8)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
}

func TestMinimax_CheckQuota_Exhausted(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [{
					"model_name": "general",
					"current_interval_status": 1,
					"current_interval_remaining_percent": 0,
					"current_weekly_status": 1,
					"current_weekly_remaining_percent": 100
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	// 5h: remaining=0 → used=100, ratio=1.0 → exhausted
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestMinimax_CheckQuota_ExhaustedByBoost(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [{
					"model_name": "general",
					"current_interval_status": 1,
					"current_interval_remaining_percent": 100,
					"current_weekly_status": 1,
					"current_weekly_remaining_percent": 30,
					"weekly_boost_permille": 1500
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	// Weekly: remaining=30, boost=1500 → total=150, used=70, bar=70/150=46.7%, ratio=0.467
	// 5h: remaining=100 → ratio=0 → available
	// Overall: worse(available, available) → available
	require.Equal(t, "available", quota.Status)
}

func TestMinimax_CheckQuota_ExhaustedByAPIStatus(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [{
					"model_name": "general",
					"current_interval_status": 3,
					"current_interval_remaining_percent": 100,
					"current_weekly_status": 1,
					"current_weekly_remaining_percent": 100
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	// 5h: apiStatus=3 → exhausted regardless of remaining
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestMinimax_CheckQuota_OnlyVideo_ReturnsError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [{
					"model_name": "video",
					"current_interval_status": 1,
					"current_interval_remaining_percent": 50,
					"current_weekly_status": 1,
					"current_weekly_remaining_percent": 50
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no general model")
}

func TestMinimax_CheckQuota_APIError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [],
				"base_resp": {"status_code": 1001, "status_msg": "invalid api key"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid api key")
}

func TestMinimax_CheckQuota_MissingAPIKey(t *testing.T) {
	checker := NewMinimaxQuotaChecker(nil)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{Type: channel.TypeMinimax})
	require.ErrorContains(t, err, "channel has no API key")
}

func TestMinimax_CheckQuota_APIKeysFallback(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer fallback-key", req.Header.Get("Authorization"))

			body := `{
				"model_remains": [{
					"model_name": "general",
					"current_interval_status": 1,
					"current_interval_remaining_percent": 100,
					"current_weekly_status": 1,
					"current_weekly_remaining_percent": 100
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{
			APIKey:  "",
			APIKeys: []string{"fallback-key"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestMinimax_CheckQuota_MalformedJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`not json`)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse minimax quota response")
}

func TestBuildMinimaxQuotaURL(t *testing.T) {
	require.Equal(t, "https://www.minimaxi.com/v1/token_plan/remains", buildMinimaxQuotaURL(""))
	require.Equal(t, "https://custom.example.com/v1/token_plan/remains", buildMinimaxQuotaURL("https://custom.example.com"))
	require.Equal(t, "https://custom.example.com/v1/token_plan/remains", buildMinimaxQuotaURL("https://custom.example.com/v1"))
	require.Equal(t, "https://www.minimaxi.com/v1/token_plan/remains", buildMinimaxQuotaURL("not a URL"))
}

func TestMinimax_SupportsChannel(t *testing.T) {
	checker := NewMinimaxQuotaChecker(nil)

	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeMinimax}))
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeMinimaxAnthropic}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeMoonshotCoding}))
}

func TestMinimax_CheckQuota_WithResetTime(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"model_remains": [{
					"model_name": "general",
					"start_time": 1784304000000,
					"end_time": 1784322000000,
					"current_interval_status": 1,
					"current_interval_remaining_percent": 100,
					"weekly_start_time": 1783872000000,
					"weekly_end_time": 1784476800000,
					"current_weekly_status": 1,
					"current_weekly_remaining_percent": 100
				}],
				"base_resp": {"status_code": 0, "status_msg": "success"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewMinimaxQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMinimax,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.NotNil(t, quota.NextResetAt)
	// end_time=1784322000000 is earlier than weekly_end_time=1784476800000
	require.Equal(t, int64(1784322000), quota.NextResetAt.Unix())
}
