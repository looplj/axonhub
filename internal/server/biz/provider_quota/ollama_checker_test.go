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
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

// ollamaSettingsHTML mirrors the real ollama.com/settings usage page the
// scraper was validated against: two data-usage-track meters (Session 11.7%,
// Weekly 62.5%) each with a data-time reset timestamp.
func ollamaSettingsHTML() string {
	return `<!DOCTYPE html><html><head><title>Usage &middot; Settings</title></head><body>` +
		`<h2>Cloud usage</h2>` +
		`<div>` +
		`<div class="flex justify-between mb-2">` +
		`<span class="text-sm">Session usage</span>` +
		`<span class="text-sm">11.7% used</span>` +
		`</div>` +
		`<div class="relative group" data-usage-meter>` +
		`<div class="relative h-3 overflow-hidden rounded-full bg-neutral-200" data-usage-track aria-label="Session usage 11.7% used">` +
		`<div class="flex h-full" style="width: 11.7%;"></div>` +
		`</div>` +
		`</div>` +
		`<div class="text-xs text-neutral-500 mt-1 local-time" data-time="2026-09-05T21:00:00Z">Resets in 3 hours.</div>` +
		`</div>` +
		`<div>` +
		`<div class="flex justify-between mb-2">` +
		`<span class="text-sm">Weekly usage</span>` +
		`<span class="text-sm">62.5% used</span>` +
		`</div>` +
		`<div class="relative group" data-usage-meter>` +
		`<div class="relative h-3 overflow-hidden rounded-full bg-neutral-200" data-usage-track aria-label="Weekly usage 62.5% used">` +
		`<div class="flex h-full" style="width: 62.5%;"></div>` +
		`</div>` +
		`</div>` +
		`<div class="text-xs text-neutral-500 mt-1 local-time" data-time="2026-09-07T00:00:00Z">Resets in 1 day.</div>` +
		`</div>` +
		`</body></html>`
}

func ollamaChannel(authCookie string) *ent.Channel {
	return &ent.Channel{
		Type: channel.TypeOllama,
		Settings: &objects.ChannelSettings{
			ProviderQuota: &objects.ChannelProviderQuotaSettings{
				Ollama: &objects.OllamaQuotaSettings{AuthCookie: authCookie},
			},
		},
	}
}

func TestOllama_CheckQuota_Success(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "https://ollama.com/settings", req.URL.String())
			require.Contains(t, req.Header.Get("Cookie"), "__Secure-session=")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(ollamaSettingsHTML())),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOllamaQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"))
	require.NoError(t, err)
	// Both windows are under the 80% warning threshold, so the overall status
	// is available (session 11.7%, weekly 62.5%).
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, ollamaProviderType, quota.ProviderType)
	require.Len(t, quota.Limits, 2)

	// Weekly (62.5%) is the heavier window; both stay available.
	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
	require.Equal(t, QuotaWindowWeekly, quota.Limits[1].Window)

	require.Equal(t, "2026-09-05T21:00:00Z", quota.NextResetAt.Format("2006-01-02T15:04:05Z"))

	windows, ok := quota.RawData["windows"].(map[string]any)
	require.True(t, ok)
	require.Len(t, windows, 2)

	session := windows[QuotaWindow5h].(map[string]any)
	require.InDelta(t, 11.7, session["usage_percent"], 0.001)
	require.Equal(t, "available", session["status"])

	weekly := windows[QuotaWindowWeekly].(map[string]any)
	require.InDelta(t, 62.5, weekly["usage_percent"], 0.001)
	require.Equal(t, "available", weekly["status"])
}

// TestOllama_CheckQuota_Warning exercises the warning escalation when a window
// crosses the 80% threshold.
func TestOllama_CheckQuota_Warning(t *testing.T) {
	// The parser reads the percent from the meter's aria-label, so replace the
	// weekly meter label (not only the text span).
	html := strings.Replace(ollamaSettingsHTML(),
		`aria-label="Weekly usage 62.5% used"`,
		`aria-label="Weekly usage 85% used"`, 1)

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOllamaQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"))
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)

	weekly := quota.RawData["windows"].(map[string]any)[QuotaWindowWeekly].(map[string]any)
	require.Equal(t, "warning", weekly["status"])
}

func TestOllama_CheckQuota_ExhaustedWeekly(t *testing.T) {
	html := strings.Replace(ollamaSettingsHTML(),
		`aria-label="Weekly usage 62.5% used"`,
		`aria-label="Weekly usage 100% used"`, 1)

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOllamaQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"))
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestOllama_CheckQuota_MissingCookie(t *testing.T) {
	checker := NewOllamaQuotaChecker(nil)
	ch := ollamaChannel("") // empty cookie -> invalid credentials
	_, err := checker.CheckQuota(context.Background(), ch)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestOllama_CheckQuota_ExpiredCookie(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Sign-in redirect / session expiry is commonly an auth page or a
			// 403; surface as invalid credentials.
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader("forbidden")),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOllamaQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

// A 200 login/redirect page (expired cookie that isn't a hard 403) must still be
// classified as invalid credentials so stale quota is invalidated, not retained.
func TestOllama_CheckQuota_ExpiredCookieLoginPage200(t *testing.T) {
	loginHTML := `<!DOCTYPE html><html><head><title>Sign in · Ollama</title></head><body>` +
		`<form action="/signin"><input name="email"/><button>Sign in</button></form></body></html>`

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Site returns 200 with the AuthKit sign-in page instead of 403.
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(loginHTML)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOllamaQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestOllama_LooksLikeLoginPage(t *testing.T) {
	require.True(t, ollamaLooksLikeLoginPage(`<html><head><title>Sign in · Ollama</title></head><body>...</body></html>`))
	require.True(t, ollamaLooksLikeLoginPage(`<form action="/signin">Sign in</form>`))
	require.True(t, ollamaLooksLikeLoginPage(`signin`))
	require.True(t, ollamaLooksLikeLoginPage(`Log in to your account`))
	// Normal settings page / genuinely broken page must NOT be flagged as login.
	require.False(t, ollamaLooksLikeLoginPage(ollamaSettingsHTML()))
	require.False(t, ollamaLooksLikeLoginPage(`<html><body><p>no data</p></body></html>`))
}

func TestOllama_CheckQuota_OnlySessionWindow(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Usage</title></head><body>` +
		`<span class="text-sm">Session usage</span>` +
		`<span class="text-sm">50% used</span>` +
		`<div data-usage-track aria-label="Session usage 50% used"><div style="width: 50%;"></div></div>` +
		`<div class="local-time" data-time="2026-09-05T21:00:00Z">Resets soon.</div>` +
		`</body></html>`

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOllamaQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"))
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
	require.Equal(t, "available", quota.Status)
}

func TestOllama_CheckQuota_NoWindows(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Usage</title></head><body><p>No data</p></body></html>`

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOllamaQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no Ollama usage windows found")
}

func TestOllama_SupportsChannel(t *testing.T) {
	checker := NewOllamaQuotaChecker(nil)

	require.True(t, checker.SupportsChannel(ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789")))

	// Anthropic variant is supported too.
	ch := ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789")
	ch.Type = channel.TypeOllamaAnthropic
	require.True(t, checker.SupportsChannel(ch))

	// Non-Ollama channel is unsupported.
	ch = ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789")
	ch.Type = channel.TypeOpenai
	require.False(t, checker.SupportsChannel(ch))

	// Ollama channel with no cookie is unsupported.
	require.False(t, checker.SupportsChannel(ollamaChannel("")))
}

func TestOllama_NormalizeCookie(t *testing.T) {
	longVal := "__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"
	// Only the __Secure-session cookie is kept; extras like aid / cf_clearance
	// are dropped.
	cookie, err := NormalizeOllamaCookie(longVal + "; aid=38e79ba9; cf_clearance=xyz")
	require.NoError(t, err)
	require.Equal(t, longVal, cookie)

	// Leading "Cookie:" label is tolerated (devtools paste).
	cookie, err = NormalizeOllamaCookie("Cookie: " + longVal)
	require.NoError(t, err)
	require.Equal(t, longVal, cookie)

	// Case-insensitive cookie name is normalized back to the canonical name.
	cookie, err = NormalizeOllamaCookie("__secure-session=" + "aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789")
	require.NoError(t, err)
	require.Equal(t, longVal, cookie)

	// Missing session cookie rejected.
	_, err = NormalizeOllamaCookie("aid=38e79ba9")
	require.Error(t, err)
	require.Contains(t, err.Error(), "__Secure-session")

	// Too short rejected.
	_, err = NormalizeOllamaCookie("__Secure-session=abc123")
	require.Error(t, err)

	// Multi-line rejected.
	_, err = NormalizeOllamaCookie(longVal + "\ncf_clearance=x")
	require.Error(t, err)

	// Empty / garbage rejected.
	_, err = NormalizeOllamaCookie("")
	require.Error(t, err)

	// Segment without a value rejected.
	_, err = NormalizeOllamaCookie("__Secure-session=")
	require.Error(t, err)
}

// The checker must reject HTTPS-to-HTTP redirects so the session cookie is
// never forwarded over a downgraded (cleartext) connection. We simulate a real
// 302 redirect from https://ollama.com/settings to an http:// target; the
// client's CheckRedirect (set by WithRejectHTTPSDowngrade) must refuse it.
func TestOllama_CheckQuota_RejectsHTTPSDowngrade(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// First request to the HTTPS settings URL returns a 302 to an HTTP
			// target. The client must refuse to follow it.
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://ollama.com/settings"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	})

	checker := NewOllamaQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), ollamaChannel("__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"))
	require.Error(t, err)
	require.ErrorContains(t, err, "refusing HTTPS to HTTP redirect")
}

const ollamaTestCookie = "__Secure-session=aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"

// ollamaMonthlyBalanceJSON is the documented /api/balance example for a
// monthly allowance plan with purchased credits.
const ollamaMonthlyBalanceJSON = `{
  "included": {
    "balance_usd": 72.5,
    "allowance_usd": 100,
    "period": {"from": "2026-09-15T09:30:00Z", "until": "2026-10-15T09:30:00Z"}
  },
  "purchased": {"balance_usd": 25}
}`

func ollamaAPIKeyChannel(apiKey, authCookie string) *ent.Channel {
	ch := ollamaChannel(authCookie)
	ch.Credentials = objects.ChannelCredentials{APIKey: apiKey}
	return ch
}

func ollamaBalanceChecker(t *testing.T, handler func(req *http.Request) (*http.Response, error)) *OllamaQuotaChecker {
	t.Helper()

	checker := NewOllamaQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(handler),
	}))
	checker.now = func() time.Time { return time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC) }

	return checker
}

func ollamaJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func ollamaLimitByWindow(t *testing.T, limits []QuotaLimitStatus, window string) QuotaLimitStatus {
	t.Helper()

	for _, limit := range limits {
		if limit.Window == window {
			return limit
		}
	}
	require.Failf(t, "limit not found", "window %q", window)

	return QuotaLimitStatus{}
}

func TestOllama_CheckQuota_BalanceMonthly(t *testing.T) {
	checker := ollamaBalanceChecker(t, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "https://ollama.com/api/balance", req.URL.String())
		require.Equal(t, "Bearer ollama-key", req.Header.Get("Authorization"))
		require.Empty(t, req.Header.Get("Cookie"))
		return ollamaJSONResponse(http.StatusOK, ollamaMonthlyBalanceJSON), nil
	})

	quota, err := checker.CheckQuota(context.Background(), ollamaAPIKeyChannel("ollama-key", ""))
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, ollamaProviderType, quota.ProviderType)
	require.Len(t, quota.Limits, 2)

	monthly := ollamaLimitByWindow(t, quota.Limits, QuotaWindowMonthly)
	require.InDelta(t, 0.275, monthly.UsageRatio, 0.0001)
	require.Equal(t, "available", monthly.Status)
	require.Equal(t, time.Date(2026, 10, 15, 9, 30, 0, 0, time.UTC), *monthly.NextResetAt)
	require.Equal(t, time.Date(2026, 9, 15, 9, 30, 0, 0, time.UTC), *monthly.PeriodStart)

	credits := ollamaLimitByWindow(t, quota.Limits, QuotaWindowCredits)
	require.True(t, IsBalanceLimit(credits))
	require.Equal(t, "available", credits.Status)

	windows := quota.RawData["windows"].(map[string]any)
	require.InDelta(t, 27.5, windows[QuotaWindowMonthly].(map[string]any)["usage_percent"], 0.001)
	require.InDelta(t, 25.0, quota.RawData["credits"].(map[string]any)["purchased_usd"], 0.001)
}

func TestOllama_CheckQuota_BalanceFreeAllowanceExhausted(t *testing.T) {
	checker := ollamaBalanceChecker(t, func(req *http.Request) (*http.Response, error) {
		return ollamaJSONResponse(http.StatusOK, `{
  "included": {
    "balance_usd": 0,
    "allowance_usd": 2.5,
    "period": {"from": "2026-09-22T14:45:33.799639Z", "until": "2026-10-22T14:45:33.799639Z"}
  },
  "purchased": {"balance_usd": 0}
}`), nil
	})

	quota, err := checker.CheckQuota(context.Background(), ollamaAPIKeyChannel("ollama-key", ""))
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, QuotaWindowMonthly, quota.Limits[0].Window)
	require.NotContains(t, quota.RawData, "credits")
}

// Ollama spends purchased credits once the included allowance runs out, so an
// exhausted allowance with credits left keeps the channel usable.
func TestOllama_CheckQuota_BalanceExhaustedAllowanceFallsBackToCredits(t *testing.T) {
	checker := ollamaBalanceChecker(t, func(req *http.Request) (*http.Response, error) {
		return ollamaJSONResponse(http.StatusOK, `{
  "included": {
    "balance_usd": 0,
    "allowance_usd": 100,
    "period": {"from": "2026-09-15T09:30:00Z", "until": "2026-10-15T09:30:00Z"}
  },
  "purchased": {"balance_usd": 5}
}`), nil
	})

	quota, err := checker.CheckQuota(context.Background(), ollamaAPIKeyChannel("ollama-key", ""))
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.Equal(t, "exhausted", ollamaLimitByWindow(t, quota.Limits, QuotaWindowMonthly).Status)

	state, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, checker.now())
	require.Equal(t, RoutingStickyOnly, state)
	require.Equal(t, "window_exhausted_balance_fallback", reason)
}

func TestOllama_CheckQuota_BalanceLegacyWindows(t *testing.T) {
	checker := ollamaBalanceChecker(t, func(req *http.Request) (*http.Response, error) {
		return ollamaJSONResponse(http.StatusOK, `{
  "included": {
    "session": {"remaining_percent": 75, "resets_at": "2026-10-08T05:00:00Z"},
    "weekly": {"remaining_percent": 15, "resets_at": "2026-10-12T00:00:00Z"}
  },
  "purchased": {"balance_usd": 0}
}`), nil
	})

	quota, err := checker.CheckQuota(context.Background(), ollamaAPIKeyChannel("ollama-key", ""))
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.Len(t, quota.Limits, 2)

	session := ollamaLimitByWindow(t, quota.Limits, QuotaWindow5h)
	require.InDelta(t, 0.25, session.UsageRatio, 0.0001)
	require.Equal(t, time.Date(2026, 10, 8, 5, 0, 0, 0, time.UTC), *session.NextResetAt)
	require.Equal(t, time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), *session.PeriodStart)

	weekly := ollamaLimitByWindow(t, quota.Limits, QuotaWindowWeekly)
	require.InDelta(t, 0.85, weekly.UsageRatio, 0.0001)
	require.Equal(t, "warning", weekly.Status)
	require.Equal(t, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), *weekly.NextResetAt)
}

func TestOllama_CheckQuota_BalanceNoQuota(t *testing.T) {
	checker := ollamaBalanceChecker(t, func(req *http.Request) (*http.Response, error) {
		return ollamaJSONResponse(http.StatusOK, `{"included": {}, "purchased": {"balance_usd": 0}}`), nil
	})

	_, err := checker.CheckQuota(context.Background(), ollamaAPIKeyChannel("ollama-key", ""))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no Ollama quota found")
}

func TestOllama_CheckQuota_BalanceUnauthorized(t *testing.T) {
	checker := ollamaBalanceChecker(t, func(req *http.Request) (*http.Response, error) {
		return ollamaJSONResponse(http.StatusUnauthorized, `{"error":"invalid credentials"}`), nil
	})

	_, err := checker.CheckQuota(context.Background(), ollamaAPIKeyChannel("bad-key", ""))
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestOllama_CheckQuota_BalanceUnauthorizedFallsBackToCookie(t *testing.T) {
	var requested []string
	checker := ollamaBalanceChecker(t, func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())
		if req.URL.String() == "https://ollama.com/api/balance" {
			return ollamaJSONResponse(http.StatusUnauthorized, `{"error":"invalid credentials"}`), nil
		}
		require.Contains(t, req.Header.Get("Cookie"), "__Secure-session=")
		require.Empty(t, req.Header.Get("Authorization"))
		return ollamaJSONResponse(http.StatusOK, ollamaSettingsHTML()), nil
	})

	quota, err := checker.CheckQuota(context.Background(), ollamaAPIKeyChannel("bad-key", ollamaTestCookie))
	require.NoError(t, err)
	require.Equal(t, []string{"https://ollama.com/api/balance", "https://ollama.com/settings"}, requested)
	require.Len(t, quota.Limits, 2)
	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
}

func TestOllama_CheckQuota_BalanceServerErrorDoesNotFallBack(t *testing.T) {
	calls := 0
	checker := ollamaBalanceChecker(t, func(req *http.Request) (*http.Response, error) {
		calls++
		return ollamaJSONResponse(http.StatusBadGateway, "bad gateway"), nil
	})

	_, err := checker.CheckQuota(context.Background(), ollamaAPIKeyChannel("ollama-key", ollamaTestCookie))
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrInvalidCredentials)
	require.Contains(t, err.Error(), "502")
	require.Equal(t, 1, calls)
}

func TestOllama_SupportsChannel_APIKey(t *testing.T) {
	checker := NewOllamaQuotaChecker(nil)

	require.True(t, checker.SupportsChannel(ollamaAPIKeyChannel("ollama-key", "")))

	ch := ollamaAPIKeyChannel("ollama-key", "")
	ch.Settings = nil
	require.True(t, checker.SupportsChannel(ch))

	require.False(t, checker.SupportsChannel(ollamaAPIKeyChannel("  ", "")))
}
