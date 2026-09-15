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

func commandCodeChannel(cookie string) *ent.Channel {
	return &ent.Channel{
		Type: channel.TypeCommandcode,
		Settings: &objects.ChannelSettings{
			ProviderQuota: &objects.ChannelProviderQuotaSettings{
				CommandCode: &objects.CommandCodeQuotaSettings{AuthCookie: cookie},
			},
		},
	}
}

func commandCodeChannelWithKey(apiKey string) *ent.Channel {
	return &ent.Channel{
		Type:        channel.TypeCommandcode,
		Credentials: objects.ChannelCredentials{APIKey: apiKey},
	}
}

func TestNormalizeCommandCodeCookie(t *testing.T) {
	t.Run("keeps only the six allowlisted cookies", func(t *testing.T) {
		raw := "__Secure-commandcode_prod_.session_token=tok; commandcode_prod_.session_data=data; _ga=GA1.2.3; better-auth.session_token=other; stripe.mid=abc"
		got, err := NormalizeCommandCodeCookie(raw)
		require.NoError(t, err)
		require.Equal(t, "__Secure-commandcode_prod_.session_token=tok; commandcode_prod_.session_data=data", got)
	})
	t.Run("ignores unknown cookies before validating their values", func(t *testing.T) {
		got, err := NormalizeCommandCodeCookie("__Secure-commandcode_prod_.session_token=tok; unused=")
		require.NoError(t, err)
		require.Equal(t, "__Secure-commandcode_prod_.session_token=tok", got)
	})
	t.Run("ignores unknown cookies without a value", func(t *testing.T) {
		got, err := NormalizeCommandCodeCookie("__Secure-commandcode_prod_.session_token=tok; unused")
		require.NoError(t, err)
		require.Equal(t, "__Secure-commandcode_prod_.session_token=tok", got)
	})

	t.Run("host prefixed session token is kept", func(t *testing.T) {
		got, err := NormalizeCommandCodeCookie("__Host-commandcode_prod_.session_token=abc")
		require.NoError(t, err)
		require.Equal(t, "__Host-commandcode_prod_.session_token=abc", got)
	})

	t.Run("bare session_token value accepted", func(t *testing.T) {
		got, err := NormalizeCommandCodeCookie("commandcode_prod_.session_token=abc")
		require.NoError(t, err)
		require.Equal(t, "commandcode_prod_.session_token=abc", got)
	})

	t.Run("optional Cookie label stripped", func(t *testing.T) {
		got, err := NormalizeCommandCodeCookie("Cookie: __Secure-commandcode_prod_.session_token=abc; _ga=x")
		require.NoError(t, err)
		require.Equal(t, "__Secure-commandcode_prod_.session_token=abc", got)
	})

	t.Run("foreign cookies only rejected", func(t *testing.T) {
		_, err := NormalizeCommandCodeCookie("_ga=GA1.2.3; stripe.mid=abc")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no Command Code session_token")
	})

	t.Run("empty rejected", func(t *testing.T) {
		_, err := NormalizeCommandCodeCookie("   ")
		require.Error(t, err)
	})

	t.Run("empty session value rejected", func(t *testing.T) {
		_, err := NormalizeCommandCodeCookie("__Secure-commandcode_prod_.session_token=")
		require.Error(t, err)
	})

	t.Run("control chars rejected", func(t *testing.T) {
		_, err := NormalizeCommandCodeCookie("__Secure-commandcode_prod_.session_token=abc\x07def")
		require.Error(t, err)
	})

	t.Run("cURL multi-line text rejected", func(t *testing.T) {
		_, err := NormalizeCommandCodeCookie("commandcode_prod_.session_token=abc\nSet-Cookie: x=y")
		require.Error(t, err)
	})

	t.Run("session_data alone rejected (no token)", func(t *testing.T) {
		_, err := NormalizeCommandCodeCookie("commandcode_prod_.session_data=onlydata")
		require.Error(t, err)
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		got, err := NormalizeCommandCodeCookie("  __Secure-commandcode_prod_.session_token = tok  ")
		require.NoError(t, err)
		require.Equal(t, "__Secure-commandcode_prod_.session_token=tok", got)
	})
}

// commandCodeCreditsBody builds a credits response with windowLimits nested
// under credits (the canonical production shape).
func commandCodeCreditsBody(monthlyRemaining, purchased string, windows string) string {
	credits := `"credits":{"monthly_remaining_usd":` + monthlyRemaining + `,"purchased_credits_usd":` + purchased
	if windows != "" {
		credits += `,"windowLimits":{` + windows + `}`
	}
	return `{` + credits + `}}`
}

func TestCommandCodeQuotaChecker_CheckQuota(t *testing.T) {
	newChecker := func(handler http.HandlerFunc) *CommandCodeQuotaChecker {
		server := &roundTripServer{handler: handler}
		hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(server.roundTrip)})
		return NewCommandCodeQuotaChecker(hc)
	}

	creditsOK := `{"credits":{"monthly_remaining_usd":60,"purchased_credits_usd":0,"windowLimits":{"five_hour":{"used_usd":2,"cap_usd":16,"usage_percent":12.5,"reset_time":2000000000},"weekly":{"used_usd":8,"cap_usd":40,"usage_percent":20,"reset_time":2000000000}}}}`
	subsOK := `{"planId":"individual-pro","status":"active","currentPeriodEnd":"2026-10-01T00:00:00Z"}`

	t.Run("happy path pro with trusted monthly limit", func(t *testing.T) {
		var paths []string
		var cookies []string
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			cookies = append(cookies, r.Header.Get("Cookie"))
			require.Equal(t, "https://commandcode.ai", r.Header.Get("Origin"))
			require.Equal(t, "https://commandcode.ai/", r.Header.Get("Referer"))
			require.Contains(t, r.Header.Get("User-Agent"), "Chrome")
			if r.URL.Path == "/internal/billing/subscriptions" {
				_, _ = io.WriteString(w, subsOK)
				return
			}
			_, _ = io.WriteString(w, creditsOK)
		})

		quota, err := checker.CheckQuota(context.Background(), commandCodeChannel("__Secure-commandcode_prod_.session_token=tok; _ga=drop"))
		require.NoError(t, err)
		require.Equal(t, []string{"/internal/billing/credits", "/internal/billing/subscriptions"}, paths)
		require.Equal(t, "__Secure-commandcode_prod_.session_token=tok", cookies[0])
		require.Equal(t, "available", quota.Status)
		require.True(t, quota.Ready)
		require.Equal(t, "commandcode", quota.ProviderType)

		// 5h (2/16=12.5%), weekly (8/40=20%), and trusted monthly (20/80=25%).
		require.Len(t, quota.Limits, 3)
		require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
		require.Equal(t, QuotaWindowWeekly, quota.Limits[1].Window)
		require.Equal(t, QuotaWindowMonthly, quota.Limits[2].Window)
		require.InDelta(t, 0.25, quota.Limits[2].UsageRatio, 0.001)

		creditsRaw := quota.RawData["credits"].(map[string]any)
		require.Equal(t, float64(60), creditsRaw["monthly_remaining_usd"])
		require.Equal(t, "individual-pro", quota.RawData["plan_id"])
	})

	t.Run("subscription endpoint failure degrades to credits only", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/internal/billing/subscriptions" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(w, creditsOK)
		})
		quota, err := checker.CheckQuota(context.Background(), commandCodeChannel("__Secure-commandcode_prod_.session_token=tok"))
		require.NoError(t, err)
		require.Equal(t, "available", quota.Status)
		// Without subscription the monthly denominator is not trusted.
		require.Len(t, quota.Limits, 2)
	})

	t.Run("plan mismatch omits monthly denominator but keeps balance", func(t *testing.T) {
		body := `{"credits":{"monthly_remaining_usd":5,"windowLimits":{"five_hour":{"used_usd":1,"cap_usd":16},"weekly":{"used_usd":2,"cap_usd":40}}}}`
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/internal/billing/subscriptions" {
				_, _ = io.WriteString(w, `{"planId":"individual-ultra","status":"active"}`)
				return
			}
			_, _ = io.WriteString(w, body)
		})
		quota, err := checker.CheckQuota(context.Background(), commandCodeChannel("commandcode_prod_.session_token=tok"))
		require.NoError(t, err)
		// Ultra table (90/180) does not match wire caps (16/40) -> no trusted
		// monthly denominator, only the two window limits remain.
		require.Len(t, quota.Limits, 2)
		require.Equal(t, float64(5), quota.RawData["credits"].(map[string]any)["monthly_remaining_usd"])
	})

	t.Run("top-up still shows balance when windows absent", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"credits":{"monthly_remaining_usd":0,"purchased_credits_usd":25}}`)
		})
		quota, err := checker.CheckQuota(context.Background(), commandCodeChannel("commandcode_prod_.session_token=tok"))
		require.NoError(t, err)
		require.Equal(t, "available", quota.Status)
		require.Len(t, quota.Limits, 1)
		require.Equal(t, QuotaWindowMonthly, quota.Limits[0].Window)
	})

	t.Run("no windows and no balance is exhausted", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"credits":{"monthly_remaining_usd":0}}`)
		})
		quota, err := checker.CheckQuota(context.Background(), commandCodeChannel("commandcode_prod_.session_token=tok"))
		require.NoError(t, err)
		require.Equal(t, "exhausted", quota.Status)
		require.False(t, quota.Ready)
	})

	t.Run("401 maps to diagnostic auth error", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
		})
		_, err := checker.CheckQuota(context.Background(), commandCodeChannel("__Secure-commandcode_prod_.session_token=expired"))
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidCredentials)
		require.Contains(t, err.Error(), "401")
		require.Contains(t, err.Error(), "expired session cookie")
	})

	t.Run("429 rate limit error preserved", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		})
		_, err := checker.CheckQuota(context.Background(), commandCodeChannel("commandcode_prod_.session_token=tok"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "429")
	})

	t.Run("malformed JSON errors", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `not json`)
		})
		_, err := checker.CheckQuota(context.Background(), commandCodeChannel("commandcode_prod_.session_token=tok"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "parse Command Code credits response")
	})

	t.Run("invalid stored cookie errors before any request", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			t.Error("no request should be made")
		})
		_, err := checker.CheckQuota(context.Background(), commandCodeChannel("_ga=only"))
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidCredentials)
		require.Contains(t, err.Error(), "invalid Command Code auth cookie")
	})
}

func TestCommandCodeQuotaChecker_SupportsChannel(t *testing.T) {
	checker := &CommandCodeQuotaChecker{}
	require.True(t, checker.SupportsChannel(commandCodeChannel("commandcode_prod_.session_token=tok")))
	require.True(t, checker.SupportsChannel(commandCodeChannelWithKey("user_key")))
	require.False(t, checker.SupportsChannel(commandCodeChannel("")))
	require.False(t, checker.SupportsChannel(commandCodeChannelWithKey("")))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeCommandcode}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeAnthropic}))
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:        channel.TypeCommandcodeAnthropic,
		Credentials: objects.ChannelCredentials{},
	}))
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:        channel.TypeCommandcodeAnthropic,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"", "user_key"}},
	}))
	// Legacy credentials.apiKey still counts, and OAuth JSON is not an API key.
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:        channel.TypeCommandcode,
		Credentials: objects.ChannelCredentials{APIKey: "legacy_key"},
	}))
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:        channel.TypeCommandcode,
		Credentials: objects.ChannelCredentials{APIKey: `{"access_token":"oauth"}`},
	}))
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:        channel.TypeCommandcode,
		Credentials: objects.ChannelCredentials{APIKey: `{"access_token":"oauth"}`, APIKeys: []string{"user_key"}},
	}))
}

func TestHasCommandCodeQuotaCredentials(t *testing.T) {
	require.False(t, HasCommandCodeQuotaCredentials(nil))
	require.True(t, HasCommandCodeQuotaCredentials(commandCodeChannelWithKey("user_key")))
	require.True(t, HasCommandCodeQuotaCredentials(commandCodeChannel("commandcode_prod_.session_token=tok")))
	require.False(t, HasCommandCodeQuotaCredentials(commandCodeChannel("")))
	// A malformed cookie is still a configured credential: CheckQuota reports the
	// format error instead of collection being silently disabled.
	require.True(t, HasCommandCodeQuotaCredentials(commandCodeChannel("_ga=foreign")))
	// ...and it never masks a usable API key.
	ch := commandCodeChannel("_ga=foreign")
	ch.Credentials = objects.ChannelCredentials{APIKey: "user_key"}
	require.True(t, HasCommandCodeQuotaCredentials(ch))
	// Whitespace-only values do not count.
	require.False(t, HasCommandCodeQuotaCredentials(&ent.Channel{
		Type:        channel.TypeCommandcode,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"   ", ""}},
	}))
	require.False(t, HasCommandCodeQuotaCredentials(&ent.Channel{
		Type: channel.TypeCommandcode,
		Settings: &objects.ChannelSettings{
			ProviderQuota: &objects.ChannelProviderQuotaSettings{
				CommandCode: &objects.CommandCodeQuotaSettings{AuthCookie: "  "},
			},
		},
	}))
}

// roundTripServer bridges a plain http.HandlerFunc onto roundTripFunc.
type roundTripServer struct {
	handler http.HandlerFunc
}

func (s *roundTripServer) roundTrip(req *http.Request) (*http.Response, error) {
	rec := &responseRecorder{header: make(http.Header)}
	s.handler(rec, req)
	return &http.Response{
		StatusCode: rec.status,
		Header:     rec.header,
		Body:       io.NopCloser(strings.NewReader(rec.body.String())),
	}, nil
}

type responseRecorder struct {
	status int
	header http.Header
	body   strings.Builder
}

func (r *responseRecorder) Header() http.Header { return r.header }
func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}
func (r *responseRecorder) WriteHeader(status int) { r.status = status }

func TestCommandCodeResetVariants(t *testing.T) {
	// window reset seconds (2000000000), milliseconds (1767225600000), RFC3339
	// string, and <=0 (ignored) are all covered via parseCommandCodeReset directly.
	sec := parseCommandCodeReset(float64(2000000000))
	require.NotNil(t, sec)
	require.Equal(t, int64(2000000000), sec.Unix())

	ms := parseCommandCodeReset(float64(1767225600000))
	require.NotNil(t, ms)
	require.Equal(t, int64(1767225600), ms.Unix())

	rfc := parseCommandCodeReset("2026-01-01T00:00:00Z")
	require.NotNil(t, rfc)
	require.Equal(t, int64(1767225600), rfc.Unix())

	require.Nil(t, parseCommandCodeReset(float64(0)))
	require.Nil(t, parseCommandCodeReset(float64(-5)))
	require.Nil(t, parseCommandCodeReset("garbage"))
}

func TestCommandCodeCamelSnakeAndNestedWindowLimits(t *testing.T) {
	// windowLimits under root (camelCase keys); credits fields snake_case.
	body := `{"windowLimits":{"fiveHour":{"usedUsd":1,"capUsd":16,"usagePercent":6.25,"reset":2000000000},"weekly":{"usedUsd":2,"capUsd":40,"usagePercent":5,"reset":2000000000}},"monthly_remaining_usd":60,"purchased_credits_usd":0}`
	quota, err := parseCommandCodeCredits([]byte(body), nil)
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.Limits, 2)
	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
	require.InDelta(t, 0.0625, quota.Limits[0].UsageRatio, 0.0001)
	// No subscription -> no monthly denominator.
	require.Equal(t, float64(60), quota.RawData["credits"].(map[string]any)["monthly_remaining_usd"])
}

// TestCommandCodeRealWirePayload locks the parser to the actual production
// payloads observed 2026-09: credits uses monthlyCredits/purchasedCredits and
// windowLimits.{fiveHour,weekly}.{used,cap,resetAt(ms)}; subscriptions wraps
// the subscription under "data" with planId/currentPeriodEnd. Reset epochs are
// generated relative to now because NormalizeQuotaData drops resets that are
// already in the past.
func TestCommandCodeRealWirePayload(t *testing.T) {
	// Keep resets in the future: normalization clears expired timestamps.
	// Preserve the wire format and millisecond precision of the captured payload.
	now := time.Now().Truncate(time.Second)
	fiveHourReset := now.Add(2*time.Hour + 798*time.Millisecond)
	weeklyReset := now.Add(3*24*time.Hour + 618*time.Millisecond)
	credits := fmt.Sprintf(`{"credits":{"belowThreshold":false,"creditThreshold":0,"monthlyCredits":24.7335198407,"purchasedCredits":0,"premiumMonthlyCredits":0,"opensourceMonthlyCredits":24.7335198407},"windowLimits":{"limited":true,"exceeded":null,"fiveHour":{"used":0.3185113898,"cap":14,"exceeded":false,"resetAt":%d},"weekly":{"used":1.2796088158,"cap":35,"exceeded":false,"resetAt":%d}}}`, fiveHourReset.UnixMilli(), weeklyReset.UnixMilli())
	subs := `{"success":true,"data":{"id":"sub_x","status":"active","planId":"individual-goat","currentPeriodEnd":"2026-09-19T12:02:05.000Z","metadata":{"commandCode":"true"}}}`

	quota, err := parseCommandCodeCredits([]byte(credits), []byte(subs))
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)

	// Both windows plus the trusted monthly window (goat: 14/35/70).
	require.Len(t, quota.Limits, 3)
	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
	require.Equal(t, QuotaWindowWeekly, quota.Limits[1].Window)
	require.Equal(t, QuotaWindowMonthly, quota.Limits[2].Window)

	creditsRaw := quota.RawData["credits"].(map[string]any)
	require.InDelta(t, 24.7335198407, creditsRaw["monthly_remaining_usd"], 0.0001)
	require.Equal(t, float64(0), creditsRaw["purchased_credits_usd"])
	// Trusted monthly denominator from the matched plan.
	require.InDelta(t, float64(70), creditsRaw["monthly_limit_usd"], 0.0001)

	require.Equal(t, "individual-goat", quota.RawData["plan_id"])
	require.Equal(t, "active", quota.RawData["subscription_status"])
	require.Equal(t, "2026-09-19T12:02:05.000Z", quota.RawData["current_period_end"])

	// Window reset times survive the millisecond epoch → RFC3339 round trip.
	windows := quota.RawData["windows"].(map[string]any)
	fiveHour := windows["five_hour"].(map[string]any)
	require.Equal(t, fiveHourReset.Format(time.RFC3339), fiveHour["reset_time"])
	weekly := windows["weekly"].(map[string]any)
	require.Equal(t, weeklyReset.Format(time.RFC3339), weekly["reset_time"])
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, fiveHourReset.UnixMilli(), quota.NextResetAt.UnixMilli())
}

func TestCommandCodeGoPlanAllowance(t *testing.T) {
	// Verbatim Go-plan payloads observed 2026-09-12: the Go plan has no monthly
	// limit field, so the $10 denominator comes from the local plan table once
	// the 5h/weekly caps (3/6) match.
	credits := `{"credits":{"belowThreshold":false,"creditThreshold":0,"monthlyCredits":9.09842701,"purchasedCredits":0,"freeCredits":0},"windowLimits":{"limited":true,"exceeded":null,"fiveHour":{"used":0.104066178,"cap":3,"exceeded":false,"resetAt":1789272260599},"weekly":{"used":0.90157299,"cap":6,"exceeded":false,"resetAt":1789839833234}}}`
	subscriptions := `{"success":true,"data":{"id":"sub_x","status":"active","planId":"individual-go","currentPeriodEnd":"2026-10-10T08:20:47.000Z"}}`

	quota, err := parseCommandCodeCredits([]byte(credits), []byte(subscriptions))
	require.NoError(t, err)
	require.Len(t, quota.Limits, 3)
	require.Equal(t, QuotaWindowMonthly, quota.Limits[2].Window)
	require.InDelta(t, 0.09016, quota.Limits[2].UsageRatio, 0.0001)
	require.InDelta(t, float64(10), quota.RawData["credits"].(map[string]any)["monthly_limit_usd"], 0.0001)
	require.InDelta(t, 9.09842701, quota.RawData["credits"].(map[string]any)["monthly_remaining_usd"], 0.0001)
	require.NotNil(t, quota.NextResetAt)
}

// TestCommandCodePlanAllowanceRows pins the legacy/re-issued plan split: the
// wire caps, not the plan id alone, select the monthly denominator.
func TestCommandCodePlanAllowanceRows(t *testing.T) {
	parse := func(planID, credits string) QuotaData {
		t.Helper()
		subs := `{"success":true,"data":{"planId":"` + planID + `","status":"active"}}`
		quota, err := parseCommandCodeCredits([]byte(credits), []byte(subs))
		require.NoError(t, err)

		return quota
	}

	t.Run("legacy pro keeps the 30 dollar allowance", func(t *testing.T) {
		credits := `{"credits":{"monthlyCredits":29.5},"windowLimits":{"fiveHour":{"used":1,"cap":6},"weekly":{"used":2,"cap":15}}}`
		quota := parse("individual-pro", credits)
		require.Len(t, quota.Limits, 3)
		require.InDelta(t, float64(30), quota.RawData["credits"].(map[string]any)["monthly_limit_usd"], 0.0001)
		require.InDelta(t, 0.5/30, quota.Limits[2].UsageRatio, 0.0001)
	})

	for _, planID := range []string{"individual-pro", "individual-pro-v1"} {
		t.Run("new pro reports 80 dollars for "+planID, func(t *testing.T) {
			credits := `{"credits":{"monthlyCredits":60},"windowLimits":{"fiveHour":{"used":2,"cap":16},"weekly":{"used":8,"cap":40}}}`
			quota := parse(planID, credits)
			require.Len(t, quota.Limits, 3)
			require.InDelta(t, float64(80), quota.RawData["credits"].(map[string]any)["monthly_limit_usd"], 0.0001)
		})
	}

	t.Run("team pro is matched by label", func(t *testing.T) {
		credits := `{"credits":{"monthlyCredits":30},"windowLimits":{"fiveHour":{"used":3,"cap":12},"weekly":{"used":6,"cap":24}}}`
		subs := `{"success":true,"data":{"planId":"","planLabel":"Team Pro","status":"active"}}`
		quota, err := parseCommandCodeCredits([]byte(credits), []byte(subs))
		require.NoError(t, err)
		require.Len(t, quota.Limits, 3)
		require.InDelta(t, float64(40), quota.RawData["credits"].(map[string]any)["monthly_limit_usd"], 0.0001)
	})

	t.Run("pay as you go provider plan has no denominator", func(t *testing.T) {
		credits := `{"credits":{"monthlyCredits":15,"purchasedCredits":0}}`
		quota := parse("individual-provider", credits)
		require.Len(t, quota.Limits, 1)
		require.NotContains(t, quota.RawData["credits"].(map[string]any), "monthly_limit_usd")
	})

	t.Run("unknown caps degrade to windows without a denominator", func(t *testing.T) {
		// Plan id known, wire caps drifted: the balance stays visible but no
		// monthly bar is rendered.
		credits := `{"credits":{"monthlyCredits":25},"windowLimits":{"fiveHour":{"used":1,"cap":8},"weekly":{"used":2,"cap":20}}}`
		quota := parse("individual-pro", credits)
		require.Len(t, quota.Limits, 2)
		require.NotContains(t, quota.RawData["credits"].(map[string]any), "monthly_limit_usd")
		require.InDelta(t, 25, quota.RawData["credits"].(map[string]any)["monthly_remaining_usd"], 0.0001)
	})
}

// TestCommandCodeQuotaChecker_APIKey covers the /alpha surface the official CLI
// uses: it authenticates with the account API key, so plans without Provider API
// access (Go) can still report quota.
func TestCommandCodeQuotaChecker_APIKey(t *testing.T) {
	newChecker := func(handler http.HandlerFunc) *CommandCodeQuotaChecker {
		server := &roundTripServer{handler: handler}
		hc := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(server.roundTrip)})
		return NewCommandCodeQuotaChecker(hc)
	}

	creditsOK := `{"credits":{"monthlyCredits":9.09842701,"purchasedCredits":0,"freeCredits":0},"windowLimits":{"fiveHour":{"used":0.104066178,"cap":3},"weekly":{"used":0.90157299,"cap":6}}}`
	subsOK := `{"success":true,"data":{"planId":"individual-go","status":"active"}}`

	t.Run("go plan quota via api key", func(t *testing.T) {
		var paths []string
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			require.Equal(t, "Bearer user_key", r.Header.Get("Authorization"))
			require.Empty(t, r.Header.Get("Cookie"))
			if r.URL.Path == "/alpha/billing/subscriptions" {
				_, _ = io.WriteString(w, subsOK)
				return
			}
			_, _ = io.WriteString(w, creditsOK)
		})

		quota, err := checker.CheckQuota(context.Background(), commandCodeChannelWithKey("user_key"))
		require.NoError(t, err)
		require.Equal(t, []string{"/alpha/billing/credits", "/alpha/billing/subscriptions"}, paths)
		require.Equal(t, "available", quota.Status)
		require.Equal(t, "individual-go", quota.RawData["plan_id"])
		require.Len(t, quota.Limits, 3)
		require.InDelta(t, 0.0347, quota.Limits[0].UsageRatio, 0.001)
		require.InDelta(t, 0.1503, quota.Limits[1].UsageRatio, 0.001)
		require.InDelta(t, 0.09016, quota.Limits[2].UsageRatio, 0.0001)
	})

	t.Run("api key wins over an unusable cookie", func(t *testing.T) {
		var paths []string
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			_, _ = io.WriteString(w, creditsOK)
		})
		ch := commandCodeChannel("_ga=foreign-cookie")
		ch.Credentials = objects.ChannelCredentials{APIKey: "user_key"}

		_, err := checker.CheckQuota(context.Background(), ch)
		require.NoError(t, err)
		require.Equal(t, []string{"/alpha/billing/credits", "/alpha/billing/subscriptions"}, paths)
	})

	t.Run("rejected key falls back to the stored cookie", func(t *testing.T) {
		var paths []string
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			if strings.HasPrefix(r.URL.Path, "/alpha/") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.URL.Path == "/internal/billing/subscriptions" {
				_, _ = io.WriteString(w, subsOK)
				return
			}
			_, _ = io.WriteString(w, creditsOK)
		})
		ch := commandCodeChannel("__Secure-commandcode_prod_.session_token=tok")
		ch.Credentials = objects.ChannelCredentials{APIKey: "surface_only_key"}

		quota, err := checker.CheckQuota(context.Background(), ch)
		require.NoError(t, err)
		require.Equal(t, []string{"/alpha/billing/credits", "/internal/billing/credits", "/internal/billing/subscriptions"}, paths)
		require.Equal(t, "individual-go", quota.RawData["plan_id"])
	})

	t.Run("rejected key without a cookie surfaces the key error", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
		_, err := checker.CheckQuota(context.Background(), commandCodeChannelWithKey("bad_key"))
		require.ErrorIs(t, err, ErrInvalidCredentials)
		require.Contains(t, err.Error(), "401")
		require.Contains(t, err.Error(), "invalid API key")
	})

	t.Run("no key and no cookie errors before any request", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			t.Error("no request should be made")
		})
		_, err := checker.CheckQuota(context.Background(), commandCodeChannelWithKey(""))
		require.ErrorIs(t, err, ErrInvalidCredentials)
		require.Contains(t, err.Error(), "no Command Code API key or quota cookie")
	})

	t.Run("bad api key on a 403 is a credentials error", func(t *testing.T) {
		checker := newChecker(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		})
		_, err := checker.CheckQuota(context.Background(), commandCodeChannelWithKey("bad_key"))
		require.ErrorIs(t, err, ErrInvalidCredentials)
		require.Contains(t, err.Error(), "invalid API key")
	})
}
