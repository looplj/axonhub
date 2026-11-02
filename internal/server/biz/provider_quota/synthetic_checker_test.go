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

func TestSynthetic_CheckQuota_SearchWarningThresholdBelowExhaustion(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"weeklyTokenLimit":{"nextRegenAt":"2099-04-25T04:48:26.000Z","percentRemaining":50.0},"search":{"hourly":{"limit":250,"requests":200,"renewsAt":"2099-04-25T05:00:00.000Z"}}}`

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

	// 200/250 sits exactly at WarningThresholdRatio: the search limit warns
	// without exhausting, and the channel stays available with search ready
	// (a warning still counts as ready).
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Len(t, quota.Limits, 2)

	require.Equal(t, QuotaLimitTypeSearch, quota.Limits[1].Type)
	require.Equal(t, "search_hourly", quota.Limits[1].Window)
	require.Equal(t, "warning", quota.Limits[1].Status)
	require.InDelta(t, 0.8, quota.Limits[1].UsageRatio, 1e-9)
	require.True(t, quota.Limits[1].Ready)
	require.NotNil(t, quota.Limits[1].NextResetAt)

	// The warning must not escalate the channel: search is listed, never blocking.
	routing, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingOpen, routing, reason)
	routing, reason = EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeImage, time.Now())
	require.Equal(t, RoutingOpen, routing, reason)

	status, ready := EffectiveStatus(quota.Limits, providerquotastatus.Status(quota.Status), quota.Ready, QuotaLimitTypeToken)
	require.Equal(t, providerquotastatus.StatusAvailable, status)
	require.True(t, ready)
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

func TestSynthetic_CheckQuota_WeeklyCreditAmounts(t *testing.T) {
	type testCase struct {
		name             string
		maxCredits       string
		remainingCredits string
		wantMoney        bool
		wantCost         float64
		wantQuota        float64
	}

	tests := []testCase{
		{name: "dollar amounts", maxCredits: "$10.50", remainingCredits: "$10.47", wantMoney: true, wantCost: 0.03, wantQuota: 10.50},
		{name: "bare amounts", maxCredits: "36", remainingCredits: "13.9", wantMoney: true, wantCost: 22.1, wantQuota: 36},
		{name: "trimmed amounts", maxCredits: " $10.50 ", remainingCredits: " $10.47 ", wantMoney: true, wantCost: 0.03, wantQuota: 10.50},
		{name: "signed exponent amounts", maxCredits: "+1.05e1", remainingCredits: "+1.047e1", wantMoney: true, wantCost: 0.03, wantQuota: 10.50},
		{name: "zero remaining", maxCredits: "$10.50", remainingCredits: "$0", wantMoney: true, wantCost: 10.50, wantQuota: 10.50},
		{name: "negative remaining", maxCredits: "$10.50", remainingCredits: "-1.50", wantMoney: true, wantCost: 12, wantQuota: 10.50},
		{name: "remaining above maximum", maxCredits: "$10.50", remainingCredits: "$11", wantMoney: true, wantCost: 0, wantQuota: 10.50},
		// Each amount parses as finite, but their subtraction overflows to
		// +Inf; the derived money must be omitted, keeping the payload
		// JSON-serializable (asserted below for every case).
		{name: "subtraction overflows to infinity", maxCredits: "1e308", remainingCredits: "-1e308"},
		{name: "zero maximum", maxCredits: "0", remainingCredits: "0"},
		{name: "negative maximum", maxCredits: "-1", remainingCredits: "0"},
	}
	for _, value := range []string{"NaN", "Inf", "+Inf", "-Inf", "Infinity", "+Infinity", "-Infinity", "$NaN", "$+Inf", "$-Inf"} {
		tests = append(tests,
			testCase{name: "invalid maximum " + value, maxCredits: value, remainingCredits: "$5.00"},
			testCase{name: "invalid remaining " + value, maxCredits: "$10.50", remainingCredits: value},
		)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"weeklyTokenLimit":{"percentRemaining":50,"maxCredits":%q,"remainingCredits":%q}}`, tt.maxCredits, tt.remainingCredits)
			httpClient := httpclient.NewHttpClientWithClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			require.Equal(t, "available", quota.Status)
			require.True(t, quota.Ready)
			require.Len(t, quota.Limits, 1)
			require.Equal(t, QuotaWindowWeekly, quota.Limits[0].Window)

			serialized, err := json.Marshal(quota)
			require.NoError(t, err)
			require.True(t, json.Valid(serialized))

			if tt.wantMoney {
				require.NotNil(t, quota.Limits[0].PeriodCost)
				require.InDelta(t, tt.wantCost, *quota.Limits[0].PeriodCost, 1e-9)
				require.NotNil(t, quota.Limits[0].PeriodQuota)
				require.InDelta(t, tt.wantQuota, *quota.Limits[0].PeriodQuota, 1e-9)
			} else {
				require.Nil(t, quota.Limits[0].PeriodCost)
				require.Nil(t, quota.Limits[0].PeriodQuota)
			}
		})
	}
}

func TestParseSyntheticCredits(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		credits, ok := parseSyntheticCredits(nil)
		require.False(t, ok)
		require.Zero(t, credits)
	})

	for _, tt := range []struct {
		value string
		want  float64
		ok    bool
	}{
		{value: "$10.50", want: 10.50, ok: true},
		{value: " 10.50 ", want: 10.50, ok: true},
		{value: "+1.05e1", want: 10.50, ok: true},
		{value: "-1.5", want: -1.5, ok: true},
		{value: "0", ok: true},
		{value: ""},
		{value: "$"},
		{value: "not a credit amount"},
		{value: "NaN"},
		{value: "Inf"},
		{value: "+Inf"},
		{value: "-Inf"},
		{value: "Infinity"},
		{value: "+Infinity"},
		{value: "-Infinity"},
		{value: "$NaN"},
		{value: "$+Inf"},
		{value: "$-Inf"},
	} {
		t.Run(tt.value, func(t *testing.T) {
			credits, ok := parseSyntheticCredits(&tt.value)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, credits)
		})
	}
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

// TestSynthetic_CheckQuota_MixedWeeklyShapesIndependentFields pins the contract
// that planWeeklyLimit and weeklyTokenLimit fall back to each other
// independently per field — not per object — and that precedence is resolved
// through the real CheckQuota response, including routing decisions.
func TestSynthetic_CheckQuota_MixedWeeklyShapesIndependentFields(t *testing.T) {
	planReset := time.Date(2099, 10, 23, 0, 8, 27, 0, time.UTC)
	legacyReset := time.Date(2099, 10, 23, 4, 0, 0, 0, time.UTC)
	fiveHourReset := time.Date(2099, 10, 20, 6, 0, 0, 0, time.UTC)

	check := func(t *testing.T, body string) QuotaData {
		t.Helper()

		httpClient := httpclient.NewHttpClientWithClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
		return quota
	}

	// Healthy legacy pool shared by the mixed cases: 50% of weekly tokens
	// used with a legacy regeneration tick at 04:00.
	legacy := `{"weeklyTokenLimit":{"nextRegenAt":"2099-10-23T04:00:00.000Z","percentRemaining":50.0,"maxCredits":"$10.50","remainingCredits":"$5.25"}`
	// Five-hour window that regens before every weekly reset, so it owns the
	// channel-level reset in all cases; the weekly-limit reset is where the
	// plan/legacy interplay is observable.
	fiveHour := `,"rollingFiveHourLimit":{"nextTickAt":"2099-10-20T06:00:00.000Z","tickPercent":0.05,"remaining":700,"max":750,"limited":false}}`

	type testCase struct {
		name            string
		planWeekly      string // the planWeeklyLimit fragment appended to the legacy prefix
		wantUsage       float64
		wantWeekly      string // status of the merged weekly limit
		wantChannel     string
		wantReady       bool
		wantWeeklyReset time.Time // reset carried by the merged weekly limit
		wantRouting     map[QuotaLimitType]RoutingState
		wantStatus      map[QuotaLimitType]providerquotastatus.Status
	}

	tests := []testCase{
		{
			name:            "plan percent + legacy reset",
			planWeekly:      `,"planWeeklyLimit":{"percentRemaining":90.0,"nextRegenAt":"2099-10-23T00:08:27.000Z"}`,
			wantUsage:       0.1,
			wantWeekly:      "available",
			wantChannel:     "available",
			wantReady:       true,
			wantWeeklyReset: planReset,
			wantRouting: map[QuotaLimitType]RoutingState{
				QuotaLimitTypeToken: RoutingOpen,
				QuotaLimitTypeImage: RoutingOpen,
			},
			wantStatus: map[QuotaLimitType]providerquotastatus.Status{
				QuotaLimitTypeToken: providerquotastatus.StatusAvailable,
			},
		},
		{
			name:            "plan reset + legacy percent",
			planWeekly:      `,"planWeeklyLimit":{"nextRegenAt":"2099-10-23T00:08:27.000Z"}`,
			wantUsage:       0.5,
			wantWeekly:      "available",
			wantChannel:     "available",
			wantReady:       true,
			wantWeeklyReset: planReset,
			wantRouting: map[QuotaLimitType]RoutingState{
				QuotaLimitTypeToken: RoutingOpen,
			},
			wantStatus: map[QuotaLimitType]providerquotastatus.Status{
				QuotaLimitTypeToken: providerquotastatus.StatusAvailable,
			},
		},
		{
			name: "explicit plan percent zero overrides healthy legacy",
			// A 0% plan percentage must not be treated as "absent" and
			// dropped into the legacy 50%; the weekly pool is exhausted and
			// so is the whole channel.
			planWeekly:      `,"planWeeklyLimit":{"percentRemaining":0.0,"nextRegenPercent":2}`,
			wantUsage:       1.0,
			wantWeekly:      "exhausted",
			wantChannel:     "exhausted",
			wantReady:       false,
			wantWeeklyReset: legacyReset,
			wantRouting: map[QuotaLimitType]RoutingState{
				QuotaLimitTypeToken: RoutingExhausted,
				QuotaLimitTypeImage: RoutingExhausted,
			},
			wantStatus: map[QuotaLimitType]providerquotastatus.Status{
				QuotaLimitTypeToken: providerquotastatus.StatusExhausted,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quota := check(t, legacy+tt.planWeekly+fiveHour)

			require.Equal(t, tt.wantChannel, quota.Status)
			require.Equal(t, tt.wantReady, quota.Ready)
			require.NotNil(t, quota.NextResetAt)
			// The five-hour window owns the channel reset in every case.
			require.Equal(t, fiveHourReset, *quota.NextResetAt)

			var weekly *QuotaLimitStatus
			for i := range quota.Limits {
				if quota.Limits[i].Type == QuotaLimitTypeToken && quota.Limits[i].Window == QuotaWindowWeekly {
					weekly = &quota.Limits[i]
				}
			}
			require.NotNil(t, weekly, "merged weekly token limit must be present")
			require.Equal(t, tt.wantWeekly, weekly.Status)
			require.InDelta(t, tt.wantUsage, weekly.UsageRatio, 1e-9)
			require.NotNil(t, weekly.NextResetAt)
			require.Equal(t, tt.wantWeeklyReset, *weekly.NextResetAt)
			// Credits stay on the weekly pool even when the plan shape owns
			// the percentage/reset fields — finite-credit program.
			require.NotNil(t, weekly.PeriodQuota)
			require.InDelta(t, 10.50, *weekly.PeriodQuota, 1e-9)
			require.NotNil(t, weekly.PeriodCost)
			require.InDelta(t, 5.25, *weekly.PeriodCost, 1e-9)

			for limitType, want := range tt.wantRouting {
				got, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, limitType, time.Now())
				require.Equal(t, want, got, "routing for %s: %s", limitType, reason)
			}

			for limitType, want := range tt.wantStatus {
				got, ready := EffectiveStatus(quota.Limits, providerquotastatus.Status(quota.Status), quota.Ready, limitType)
				require.Equal(t, want, got)
				require.Equal(t, tt.wantReady, ready)
			}
		})
	}
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
