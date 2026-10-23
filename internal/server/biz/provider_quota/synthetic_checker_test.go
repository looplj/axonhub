package provider_quota

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestSynthetic_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))

			body := `{
				"subscription": {"limit": 750, "requests": 0, "renewsAt": "2099-04-25T08:47:39.947Z"},
				"search": {"hourly": {"limit": 250, "requests": 0, "renewsAt": "2099-04-25T05:00:00.000Z"}},
				"weeklyTokenLimit": {
					"nextRegenAt": "2099-04-25T04:48:26.000Z",
					"percentRemaining": 38.6,
					"maxCredits": "$36.00",
					"remainingCredits": "$13.90"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2099-04-25T04:00:01.000Z",
					"tickPercent": 0.05,
					"remaining": 750,
					"max": 750,
					"limited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "synthetic", quota.ProviderType)

	require.Len(t, quota.Limits, 3)

	require.Equal(t, QuotaLimitTypeToken, quota.Limits[0].Type)
	require.Equal(t, "available", quota.Limits[0].Status)
	require.InDelta(t, 0.05, quota.Limits[0].UsageRatio, 0.001)
	require.True(t, quota.Limits[0].Ready)
	require.NotNil(t, quota.Limits[0].NextResetAt)

	require.Equal(t, QuotaLimitTypeToken, quota.Limits[1].Type)
	require.Equal(t, "available", quota.Limits[1].Status)
	require.InDelta(t, 0.614, quota.Limits[1].UsageRatio, 0.001)
	require.True(t, quota.Limits[1].Ready)
	require.NotNil(t, quota.Limits[1].NextResetAt)
	require.NotNil(t, quota.Limits[1].PeriodQuota)
	require.InDelta(t, 36.0, *quota.Limits[1].PeriodQuota, 0.001)
	require.NotNil(t, quota.Limits[1].PeriodCost)
	require.InDelta(t, 22.1, *quota.Limits[1].PeriodCost, 0.001)
	require.Nil(t, quota.Limits[1].PeriodStart)

	require.Equal(t, QuotaLimitTypeSearch, quota.Limits[2].Type)
	require.Equal(t, "search_hourly", quota.Limits[2].Window)
	require.Equal(t, "available", quota.Limits[2].Status)
	require.Equal(t, 0.0, quota.Limits[2].UsageRatio)
	require.True(t, quota.Limits[2].Ready)
	require.NotNil(t, quota.Limits[2].NextResetAt)
	require.Equal(t, time.Date(2099, 4, 25, 5, 0, 0, 0, time.UTC), *quota.Limits[2].NextResetAt)
	require.Nil(t, quota.Limits[2].PeriodStart)
}

func TestSynthetic_CheckQuota_SearchQuotaIsListedButNeverBlocks(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"weeklyTokenLimit":{"nextRegenAt":"2099-04-25T04:48:26.000Z","percentRemaining":50.0,"maxCredits":"$36.00","remainingCredits":"$18.00"},"search":{"hourly":{"limit":250,"requests":250,"renewsAt":"2099-04-25T05:00:00.000Z"}}}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Len(t, quota.Limits, 2)

	require.Equal(t, QuotaLimitTypeSearch, quota.Limits[1].Type)
	require.Equal(t, "exhausted", quota.Limits[1].Status)
	require.Equal(t, 1.0, quota.Limits[1].UsageRatio)
	require.False(t, quota.Limits[1].Ready)

	routing, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingOpen, routing, reason)
	routing, reason = EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeImage, time.Now())
	require.Equal(t, RoutingOpen, routing, reason)

	status, ready := EffectiveStatus(quota.Limits, providerquotastatus.Status(quota.Status), quota.Ready, QuotaLimitTypeToken)
	require.Equal(t, providerquotastatus.StatusAvailable, status)
	require.True(t, ready)
}

func TestSynthetic_CheckQuota_SearchOnlyPayloadDoesNotBlock(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"search":{"hourly":{"limit":250,"requests":250,"renewsAt":"2099-04-25T05:00:00.000Z"}}}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "unknown", quota.Status)
	require.False(t, quota.Ready)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "exhausted", quota.Limits[0].Status)
	require.Nil(t, quota.NextResetAt)

	routing, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingUnknown, routing, reason)
}

func TestSynthetic_CheckQuota_SkipsUnusableSearchLimit(t *testing.T) {
	for name, body := range map[string]string{
		"zero limit":       `{"weeklyTokenLimit":{"nextRegenAt":"2099-04-25T04:48:26.000Z","percentRemaining":50.0},"search":{"hourly":{"limit":0,"requests":0,"renewsAt":"2099-04-25T05:00:00.000Z"}}}`,
		"missing requests": `{"weeklyTokenLimit":{"nextRegenAt":"2099-04-25T04:48:26.000Z","percentRemaining":50.0},"search":{"hourly":{"limit":250,"renewsAt":"2099-04-25T05:00:00.000Z"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			httpClient := httpclient.NewHttpClientWithClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(body)),
					}, nil
				}),
			})

			checker := NewSyntheticQuotaChecker(httpClient)

			quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
				Credentials: objects.ChannelCredentials{
					APIKey: "test-api-key",
				},
			})
			require.NoError(t, err)
			require.Equal(t, "available", quota.Status)
			require.Len(t, quota.Limits, 1)
			require.Equal(t, QuotaWindowWeekly, quota.Limits[0].Window)
			require.Contains(t, quota.RawData, "search")
			require.Nil(t, quota.Limits[0].PeriodCost)
			require.Nil(t, quota.Limits[0].PeriodQuota)
		})
	}
}

func TestSynthetic_CheckQuota_PersistsNewCounterRawFields(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"subscription":{"limit":500,"requests":0,"renewsAt":"2099-10-17T09:26:59.603Z"},"search":{"hourly":{"limit":250,"requests":0,"renewsAt":"2099-10-17T05:26:59.604Z"}},"freeToolCalls":{"limit":0,"requests":0,"renewsAt":"2099-10-18T04:26:59.606Z"},"weeklyTokenLimit":{"nextRegenAt":"2099-10-17T07:19:01.000Z","percentRemaining":99.73476194285715,"maxCredits":"$10.50","remainingCredits":"$10.47"}}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.Limits, 2)
	require.Equal(t, QuotaLimitTypeSearch, quota.Limits[1].Type)

	require.NotNil(t, quota.Limits[0].PeriodQuota)
	require.InDelta(t, 10.50, *quota.Limits[0].PeriodQuota, 1e-9)
	require.NotNil(t, quota.Limits[0].PeriodCost)
	require.InDelta(t, 0.03, *quota.Limits[0].PeriodCost, 1e-9)

	subscription, ok := quota.RawData["subscription"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, int64(500), subscription["limit"])
	require.Equal(t, "2099-10-17T09:26:59.603Z", subscription["renewsAt"])

	hourly := quota.RawData["search"].(map[string]any)["hourly"].(map[string]any)
	require.Equal(t, "2099-10-17T05:26:59.604Z", hourly["renewsAt"])

	freeToolCalls, ok := quota.RawData["freeToolCalls"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, int64(0), freeToolCalls["limit"])
	require.Equal(t, "2099-10-18T04:26:59.606Z", freeToolCalls["renewsAt"])
}

func TestSynthetic_CheckQuota_PlanWeeklyLimitShape(t *testing.T) {
	weeklyReset, err := time.Parse(time.RFC3339, "2099-10-23T00:08:27.000Z")
	require.NoError(t, err)

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"subscription":{"limit":500,"requests":0,"renewsAt":"2099-10-23T03:40:12.700Z"},"search":{"hourly":{"limit":250,"requests":13,"renewsAt":"2099-10-22T23:11:12.702Z"}},"planWeeklyLimit":{"nextRegenAt":"2099-10-23T00:08:27.000Z","percentRemaining":90.19385924897959,"nextRegenPercent":2}}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	quota, err := NewSyntheticQuotaChecker(httpClient).CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)

	// Big/pro plans report the weekly pool as planWeeklyLimit only; it must
	// still become the blocking weekly limit with its own reset.
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Len(t, quota.Limits, 2)

	require.Equal(t, QuotaLimitTypeToken, quota.Limits[0].Type)
	require.Equal(t, QuotaWindowWeekly, quota.Limits[0].Window)
	require.Equal(t, "available", quota.Limits[0].Status)
	require.InDelta(t, 0.0980614075102041, quota.Limits[0].UsageRatio, 1e-9)
	require.NotNil(t, quota.Limits[0].NextResetAt)
	require.Equal(t, weeklyReset, *quota.Limits[0].NextResetAt)
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, weeklyReset, *quota.NextResetAt)
	require.Nil(t, quota.Limits[0].PeriodCost)
	require.Nil(t, quota.Limits[0].PeriodQuota)

	require.Equal(t, QuotaLimitTypeSearch, quota.Limits[1].Type)
	require.Equal(t, "search_hourly", quota.Limits[1].Window)
	require.InDelta(t, 0.052, quota.Limits[1].UsageRatio, 1e-9)

	planWeekly, ok := quota.RawData["planWeeklyLimit"].(map[string]any)
	require.True(t, ok)
	require.InDelta(t, 90.19385924897959, planWeekly["percentRemaining"], 1e-9)
	require.InDelta(t, 2.0, planWeekly["nextRegenPercent"], 1e-9)
	require.NotContains(t, quota.RawData, "weeklyTokenLimit")
}

func TestSynthetic_CheckQuota_BothWeeklyShapesPreferPlanPercentKeepCredits(t *testing.T) {
	planReset, err := time.Parse(time.RFC3339, "2099-10-23T00:08:27.000Z")
	require.NoError(t, err)

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"weeklyTokenLimit":{"nextRegenAt":"2099-10-23T04:00:00.000Z","percentRemaining":50.0,"maxCredits":"$10.50","remainingCredits":"$10.47"},"planWeeklyLimit":{"nextRegenAt":"2099-10-23T00:08:27.000Z","percentRemaining":90.0,"nextRegenPercent":2}}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	quota, err := NewSyntheticQuotaChecker(httpClient).CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)

	// planWeeklyLimit wins the shared fields; the credit amounts still come
	// from weeklyTokenLimit.
	require.InDelta(t, 0.1, quota.Limits[0].UsageRatio, 1e-9)
	require.Equal(t, planReset, *quota.Limits[0].NextResetAt)
	require.NotNil(t, quota.Limits[0].PeriodQuota)
	require.InDelta(t, 10.50, *quota.Limits[0].PeriodQuota, 1e-9)
	require.NotNil(t, quota.Limits[0].PeriodCost)
	require.InDelta(t, 0.03, *quota.Limits[0].PeriodCost, 1e-9)
	require.Contains(t, quota.RawData, "weeklyTokenLimit")
	require.Contains(t, quota.RawData, "planWeeklyLimit")
}

func TestSynthetic_CheckQuota_PartialPlanShapeFallsBackPerField(t *testing.T) {
	legacyReset, err := time.Parse(time.RFC3339, "2099-10-23T04:00:00.000Z")
	require.NoError(t, err)

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"weeklyTokenLimit":{"nextRegenAt":"2099-10-23T04:00:00.000Z","percentRemaining":50.0,"maxCredits":"$10.50","remainingCredits":"$10.47"},"planWeeklyLimit":{"nextRegenPercent":2}}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	quota, err := NewSyntheticQuotaChecker(httpClient).CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)

	// A plan object without its own percentage or reset must not shadow the
	// weeklyTokenLimit fields.
	require.InDelta(t, 0.5, quota.Limits[0].UsageRatio, 1e-9)
	require.NotNil(t, quota.Limits[0].NextResetAt)
	require.Equal(t, legacyReset, *quota.Limits[0].NextResetAt)
	require.NotNil(t, quota.Limits[0].PeriodQuota)
	require.InDelta(t, 10.50, *quota.Limits[0].PeriodQuota, 1e-9)
}

func TestSynthetic_CheckQuota_WarningState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"subscription": {"limit": 750, "requests": 0, "renewsAt": "2026-04-25T08:47:39.947Z"},
				"weeklyTokenLimit": {
					"nextRegenAt": "2026-04-25T04:48:26.000Z",
					"percentRemaining": 15.0,
					"maxCredits": "$36.00",
					"remainingCredits": "$5.40"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2026-04-25T04:00:01.000Z",
					"tickPercent": 0.85,
					"remaining": 112,
					"max": 750,
					"limited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)

	require.Len(t, quota.Limits, 2)

	require.Equal(t, "warning", quota.Limits[0].Status)
	require.InDelta(t, 0.85, quota.Limits[0].UsageRatio, 0.001)
	require.True(t, quota.Limits[0].Ready)

	require.Equal(t, "warning", quota.Limits[1].Status)
	require.InDelta(t, 0.85, quota.Limits[1].UsageRatio, 0.001)
	require.True(t, quota.Limits[1].Ready)
}

func TestSynthetic_WarningAtUsageThreshold(t *testing.T) {
	tickPercent := 0.8
	percentRemaining := 20.0

	limits := buildSyntheticLimitStatuses(
		&SyntheticWeeklyTokenLimit{PercentRemaining: &percentRemaining},
		&SyntheticRollingFiveHourLimit{TickPercent: &tickPercent},
	)

	require.Len(t, limits, 2)
	require.Equal(t, "warning", limits[0].Status)
	require.Equal(t, "warning", limits[1].Status)
}

func TestSynthetic_CheckQuota_ExhaustedState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"subscription": {"limit": 750, "requests": 750, "renewsAt": "2026-04-25T08:47:39.947Z"},
				"weeklyTokenLimit": {
					"nextRegenAt": "2026-04-25T04:48:26.000Z",
					"percentRemaining": 0.0,
					"maxCredits": "$36.00",
					"remainingCredits": "$0.00"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2026-04-25T04:00:01.000Z",
					"tickPercent": 1.0,
					"remaining": 0,
					"max": 750,
					"limited": true
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)

	require.Len(t, quota.Limits, 2)

	require.Equal(t, "exhausted", quota.Limits[0].Status)
	require.Equal(t, 1.0, quota.Limits[0].UsageRatio)
	require.False(t, quota.Limits[0].Ready)

	require.Equal(t, "exhausted", quota.Limits[1].Status)
	require.InDelta(t, 1.0, quota.Limits[1].UsageRatio, 0.001)
	require.False(t, quota.Limits[1].Ready)
}

func TestSynthetic_CheckQuota_MissingCredentials(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no API key")
}

func TestSynthetic_CheckQuota_MalformedJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`not json`)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse synthetic usage response")
}

func TestSynthetic_CheckQuota_HTTPError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestSynthetic_CheckQuota_CustomBaseURL(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://us-east.api.synthetic.new/v2/quotas", req.URL.String())

			body := `{
				"subscription": {"limit": 750, "requests": 0, "renewsAt": "2026-04-25T08:47:39.947Z"},
				"weeklyTokenLimit": {
					"nextRegenAt": "2026-04-25T04:48:26.000Z",
					"percentRemaining": 50.0,
					"maxCredits": "$36.00",
					"remainingCredits": "$18.00"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2026-04-25T04:00:01.000Z",
					"tickPercent": 0.5,
					"remaining": 375,
					"max": 750,
					"limited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: "https://us-east.api.synthetic.new",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
}

func TestSynthetic_SupportsChannel(t *testing.T) {
	checker := NewSyntheticQuotaChecker(nil)

	// TypeOpenai + Synthetic URL → true
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.synthetic.new",
	}))
	// TypeOpenaiResponses + Synthetic URL → true
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenaiResponses,
		BaseURL: "https://api.synthetic.new",
	}))
	// TypeOpenai + non-Synthetic URL → false
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.openai.com",
	}))
	// Non-OpenAI type + Synthetic URL → false
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeNanogpt,
		BaseURL: "https://api.synthetic.new",
	}))
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.synthetic.new:443",
	}))
}

func TestSynthetic_CheckQuota_NextResetAt(t *testing.T) {
	expectedTime, _ := time.Parse(time.RFC3339, "2099-04-25T04:00:01.000Z")

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// rollingFiveHourLimit.nextTickAt is earliest; the search counter
			// renews even earlier but must not move the channel reset.
			body := `{
				"subscription": {"limit": 750, "requests": 0, "renewsAt": "2099-04-25T08:47:39.947Z"},
				"search": {"hourly": {"limit": 250, "requests": 0, "renewsAt": "2099-04-25T03:00:00.000Z"}},
				"weeklyTokenLimit": {
					"nextRegenAt": "2099-04-25T04:48:26.000Z",
					"percentRemaining": 38.6,
					"maxCredits": "$36.00",
					"remainingCredits": "$13.90"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2099-04-25T04:00:01.000Z",
					"tickPercent": 0.05,
					"remaining": 750,
					"max": 750,
					"limited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, expectedTime, *quota.NextResetAt)
}

func TestBuildSyntheticQuotaURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "empty base URL uses default",
			baseURL:  "",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "whitespace-only base URL uses default",
			baseURL:  "  ",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "valid https URL extracts scheme and host",
			baseURL:  "https://api.synthetic.new",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "URL with path strips path",
			baseURL:  "https://api.synthetic.new/v1/chat",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "URL with trailing slash strips path",
			baseURL:  "https://api.synthetic.new/",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "http URL is upgraded to https",
			baseURL:  "http://api.synthetic.new",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "invalid URL uses default",
			baseURL:  "://invalid",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "subdomain URL",
			baseURL:  "https://us-east.api.synthetic.new",
			expected: "https://us-east.api.synthetic.new/v2/quotas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSyntheticQuotaURL(tt.baseURL)
			require.Equal(t, tt.expected, result)
		})
	}
}
