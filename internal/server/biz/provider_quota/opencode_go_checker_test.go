package provider_quota

import (
	"context"
	"encoding/json"
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

func TestOpenCodeGo_CheckQuota_Success(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "https://opencode.ai/zen/go/v1/usage", req.URL.String())
			require.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Accept"))

			body := `{"usage":{
				"rolling":{"percent":12,"resetsAt":"2026-06-25T15:00:00Z"},
				"weekly":{"percent":85,"resetsAt":"2026-07-02T10:00:00Z"},
				"monthly":{"percent":35,"resetsAt":"2026-07-25T10:00:00Z"}
			}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	checker.now = func() time.Time { return now }

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, opencodeGoProviderType, quota.ProviderType)
	require.Len(t, quota.Limits, 3)
	require.Equal(t, now.Add(5*time.Hour), *quota.NextResetAt)

	windows, ok := quota.RawData["windows"].(map[string]any)
	require.True(t, ok)
	rolling, ok := windows["rolling"].(map[string]any)
	require.True(t, ok)
	require.InDelta(t, 12, rolling["usage_percent"], 0.001)
	require.InDelta(t, 88, rolling["percent_remaining"], 0.001)
	require.Equal(t, "available", rolling["status"])
	require.Equal(t, now.Add(5*time.Hour).Format(time.RFC3339), rolling["reset_time"])

	weekly, ok := windows["weekly"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "warning", weekly["status"])
}

func TestOpenCodeGo_CheckQuota_UnixResetsAt(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"usage":{"rolling":{"percent":12,"resetsAt":1782370800}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	checker.now = func() time.Time { return now }

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.Limits, 1)

	windows := quota.RawData["windows"].(map[string]any)
	rolling := windows["rolling"].(map[string]any)
	require.InDelta(t, 12, rolling["usage_percent"], 0.001)
	require.Equal(t, time.Unix(1782370800, 0).Format(time.RFC3339), rolling["reset_time"])
}

func TestOpenCodeGo_CheckQuota_PartialWindows(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"usage":{"rolling":{"percent":100,"resetsAt":"2026-06-25T15:00:00Z"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	checker.now = func() time.Time { return now }

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, now.Add(5*time.Hour), *quota.NextResetAt)
}

func TestOpenCodeGo_CheckQuota_InvalidResponse(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.ErrorContains(t, err, "could not parse OpenCode Go usage windows")
}

func TestOpenCodeGo_CheckQuota_MillisecondResetsAt(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Live API shape: RFC3339 with fractional seconds (verified 2026-08-12).
			body := `{"usage":{"rolling":{"status":"ok","percent":0,"resetsAt":"2026-08-12T11:24:29.905Z"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	checker.now = func() time.Time { return now }

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, time.Date(2026, 8, 12, 11, 24, 29, 905_000_000, time.UTC), *quota.NextResetAt)

	windows := quota.RawData["windows"].(map[string]any)
	rolling := windows["rolling"].(map[string]any)
	require.InDelta(t, 0, rolling["usage_percent"], 0.001)
	require.Equal(t, "2026-08-12T11:24:29Z", rolling["reset_time"])
}

func TestOpenCodeGo_CheckQuota_MissingAPIKey(t *testing.T) {
	checker := NewOpenCodeGoQuotaChecker(nil)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeOpencodeGo,
	})
	require.ErrorContains(t, err, "channel has no API key")
}

func TestOpenCodeGo_CheckQuota_SecondAPIKey(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer key-two", req.Header.Get("Authorization"))
			body := `{"usage":{"rolling":{"percent":12,"resetsAt":"2026-06-25T15:00:00Z"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKeys: []string{"", "key-two"}},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestOpenCodeGo_CheckQuota_EpochMilliseconds(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"usage":{"rolling":{"percent":12,"resetsAt":1782370800000}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.Equal(t, time.UnixMilli(1782370800000), *quota.NextResetAt)
}

func TestOpenCodeGo_CheckQuota_AllWindowsUnparseable(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"usage":{
				"rolling":{"percent":12,"resetsAt":"not-a-time"},
				"weekly":{"percent":8,"resetsAt":null},
				"monthly":{"percent":35,"resetsAt":-5}
			}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.ErrorContains(t, err, "could not parse OpenCode Go usage windows")
}

func TestOpenCodeGo_CheckQuota_PercentOver100(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"usage":{"rolling":{"percent":120,"resetsAt":"2026-06-25T15:00:00Z"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)

	windows := quota.RawData["windows"].(map[string]any)
	rolling := windows["rolling"].(map[string]any)
	require.InDelta(t, 120, rolling["usage_percent"], 0.001)
	require.InDelta(t, -20, rolling["percent_remaining"], 0.001)
}

func TestOpenCodeGo_CheckQuota_Unauthorized(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`{"type":"error","error":{"type":"AuthError","message":"Unauthorized"}}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.ErrorContains(t, err, "opencode go usage request failed")
}

func TestOpenCodeGo_CheckQuota_SurfacesAPISubStatus(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"usage":{"weekly":{"status":"ok","percent":21,"resetsAt":"2026-06-25T15:00:00Z"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)

	windows := quota.RawData["windows"].(map[string]any)
	weekly := windows["weekly"].(map[string]any)
	// Percent-derived status remains the source of truth; API status is surfaced.
	require.Equal(t, "available", weekly["status"])
	require.Equal(t, "ok", weekly["api_status"])
}

func TestOpenCodeGo_ParseResetsAt(t *testing.T) {
	now := time.Date(2026, 8, 12, 11, 24, 29, 0, time.UTC)

	tests := []struct {
		name   string
		raw    string
		want   time.Time
		wantOK bool
	}{
		{name: "rfc3339", raw: `"2026-08-12T11:24:29Z"`, want: now, wantOK: true},
		{name: "rfc3339 millis", raw: `"2026-08-12T11:24:29.905Z"`, want: now.Add(905 * time.Millisecond), wantOK: true},
		{name: "unix seconds", raw: `1782370800`, want: time.Unix(1782370800, 0), wantOK: true},
		{name: "unix millis", raw: `1782370800000`, want: time.UnixMilli(1782370800000), wantOK: true},
		{name: "seconds below ms threshold", raw: `999999999999`, want: time.Unix(999999999999, 0), wantOK: true},
		{name: "ms at threshold", raw: `1000000000000`, want: time.UnixMilli(1000000000000), wantOK: true},
		{name: "overflowing literal", raw: `1e300`, wantOK: false},
		{name: "garbage string", raw: `"not-a-time"`, wantOK: false},
		{name: "null", raw: `null`, wantOK: false},
		{name: "negative", raw: `-5`, wantOK: false},
		{name: "empty", raw: ``, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseOpenCodeGoResetsAt(json.RawMessage(tt.raw))
			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestOpenCodeGo_CheckQuota_NegativePercent(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"usage":{"rolling":{"percent":-5,"resetsAt":"2026-06-25T15:00:00Z"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeOpencodeGo,
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)

	windows := quota.RawData["windows"].(map[string]any)
	rolling := windows["rolling"].(map[string]any)
	require.InDelta(t, -5, rolling["usage_percent"], 0.001)
	require.InDelta(t, 105, rolling["percent_remaining"], 0.001)
}

func TestOpenCodeGo_CheckQuota_ExcludesOAuthAPIKey(t *testing.T) {
	checker := NewOpenCodeGoQuotaChecker(nil)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeOpencodeGo,
		// APIKey holds OAuth JSON, which GetAllAPIKeys excludes — so no usable key.
		Credentials: objects.ChannelCredentials{APIKey: `{"access_token":"oauth-token"}`},
	})
	require.ErrorContains(t, err, "channel has no API key")
}

func TestOpenCodeGo_SupportsChannel(t *testing.T) {
	checker := NewOpenCodeGoQuotaChecker(nil)

	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpencodeGo}))
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpencodeGoAnthropic}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai}))
}
