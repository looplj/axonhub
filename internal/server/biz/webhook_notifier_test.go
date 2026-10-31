package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

func newTestSystemServiceWithWebhookConfig(t *testing.T, client *ent.Client, cfg WebhookNotifierConfig) *SystemService {
	t.Helper()

	service := &SystemService{
		AbstractService: &AbstractService{
			db: client,
		},
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)
	require.NoError(t, service.SetWebhookNotifierConfig(ctx, &cfg))

	return service
}

func TestWebhookNotifier_NotifyChannelAutoDisabled(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	var (
		receivedBody   string
		receivedHeader string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		receivedBody = string(body)
		receivedHeader = r.Header.Get("X-Axonhub-Event")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	cfg := WebhookNotifierConfig{
		Targets: []WebhookTarget{
			{
				Name:      "default",
				Enabled:   true,
				URL:       server.URL,
				TimeoutMs: 1000,
				Headers: []objects.HeaderEntry{
					{Key: "X-AxonHub-Event", Value: "{{.Event}}"},
				},
				Body: `{"event":"{{.Event}}","channel":"{{.Channel.Name}}","status_code":{{.Trigger.StatusCode}},"threshold":{{.Trigger.Threshold}},"actual_count":{{.Trigger.ActualCount}}}`,
			},
		},
		Subscriptions: []WebhookSubscription{
			{Event: EventChannelAutoDisabled, TargetNames: []string{"default"}},
		},
	}

	systemService := newTestSystemServiceWithWebhookConfig(t, client, cfg)
	notifier := NewWebhookNotifier(systemService, httpclient.NewHttpClient())

	notifier.NotifyChannelAutoDisabled(context.Background(), ChannelAutoDisabledEvent{
		ChannelID:       1,
		ChannelName:     "primary",
		ChannelProvider: "openai",
		ChannelBaseURL:  "https://api.openai.com",
		ChannelStatus:   "disabled",
		StatusCode:      429,
		Threshold:       3,
		ActualCount:     3,
		Reason:          "quota exhausted",
		OccurredAt:      time.Unix(1712812800, 0),
	})

	require.Equal(t, EventChannelAutoDisabled, receivedHeader)
	require.Contains(t, receivedBody, `"event":"channel.auto_disabled"`)
	require.Contains(t, receivedBody, `"channel":"primary"`)
	require.Contains(t, receivedBody, `"status_code":429`)
	require.Contains(t, receivedBody, `"threshold":3`)
	require.Contains(t, receivedBody, `"actual_count":3`)
}

func TestWebhookNotifier_OccurredAtUsesSystemTimezone(t *testing.T) {
	// A fixed instant, so each expectation below is just that instant in the
	// configured zone. April puts New York on daylight saving time, at -04:00.
	occurredAt := time.Date(2026, 4, 11, 4, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		timezone string
		want     string
	}{
		{
			name:     "configured timezone applies its offset",
			timezone: "Asia/Shanghai",
			want:     "2026-04-11T12:00:00+08:00",
		},
		{
			name:     "negative offset",
			timezone: "America/New_York",
			want:     "2026-04-11T00:00:00-04:00",
		},
		{
			name:     "UTC stays unchanged",
			timezone: "UTC",
			want:     "2026-04-11T04:00:00Z",
		},
		{
			name:     "unset timezone falls back to the default",
			timezone: "",
			want:     "2026-04-11T04:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			defer client.Close()

			var receivedBody string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				receivedBody = string(body)

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cfg := WebhookNotifierConfig{
				Targets: []WebhookTarget{
					{
						Name:      "default",
						Enabled:   true,
						URL:       server.URL,
						TimeoutMs: 1000,
						Body:      `{"occurred_at":"{{.OccurredAt}}"}`,
					},
				},
				Subscriptions: []WebhookSubscription{
					{Event: EventChannelAutoDisabled, TargetNames: []string{"default"}},
				},
			}

			systemService := newTestSystemServiceWithWebhookConfig(t, client, cfg)

			settingsCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			require.NoError(t, systemService.SetGeneralSettings(settingsCtx, SystemGeneralSettings{
				Timezone: tt.timezone,
			}))

			notifier := NewWebhookNotifier(systemService, httpclient.NewHttpClient())

			// A bare context, as the production callers pass: the notifier is
			// responsible for the system bypass that lets the timezone read succeed.
			notifier.NotifyChannelAutoDisabled(ent.NewContext(context.Background(), client), ChannelAutoDisabledEvent{
				ChannelID:  1,
				OccurredAt: occurredAt,
			})

			require.Equal(t, `{"occurred_at":"`+tt.want+`"}`, receivedBody)
		})
	}
}

func TestWebhookNotifier_SkipWhenTemplateInvalid(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	called := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := WebhookNotifierConfig{
		Targets: []WebhookTarget{
			{
				Name:    "default",
				Enabled: true,
				URL:     server.URL,
				Body:    `{"event":"{{if .Event}}"}`,
			},
		},
		Subscriptions: []WebhookSubscription{
			{Event: EventChannelAutoDisabled, TargetNames: []string{"default"}},
		},
	}

	systemService := newTestSystemServiceWithWebhookConfig(t, client, cfg)
	notifier := NewWebhookNotifier(systemService, httpclient.NewHttpClient())
	notifier.NotifyChannelAutoDisabled(context.Background(), ChannelAutoDisabledEvent{OccurredAt: time.Now()})

	require.False(t, called)
}

func TestNormalizeWebhookNotifierConfig_InitializesDefaults(t *testing.T) {
	cfg := WebhookNotifierConfig{}

	normalizeWebhookNotifierConfig(&cfg)

	require.NotNil(t, cfg.Targets)
	require.NotNil(t, cfg.Subscriptions)
}

func TestWebhookNotifier_SelectTargetsSkipsInvalidTargets(t *testing.T) {
	notifier := &WebhookNotifier{}
	targets := notifier.selectTargets(WebhookNotifierConfig{
		Targets: []WebhookTarget{
			{Name: "a", Enabled: true, URL: "https://example.com"},
			{Name: "b", Enabled: false, URL: "https://example.com"},
			{Name: "c", Enabled: true, URL: ""},
		},
		Subscriptions: []WebhookSubscription{
			{Event: EventChannelAutoDisabled, TargetNames: []string{"a", "b", "c", "missing"}},
		},
	}, EventChannelAutoDisabled)

	require.Len(t, targets, 1)
	require.Equal(t, "a", targets[0].Name)
}

func TestWebhookNotifier_QuotaEvents(t *testing.T) {
	tests := []struct {
		name      string
		eventName string
		severity  string
		previous  string
		newStatus string
		notify    func(n *WebhookNotifier, evt QuotaStatusChangedEvent)
	}{
		{
			name:      "warning",
			eventName: EventProviderQuotaWarning,
			severity:  "warning",
			previous:  "available",
			newStatus: "warning",
			notify: func(n *WebhookNotifier, evt QuotaStatusChangedEvent) {
				n.NotifyProviderQuotaWarning(context.Background(), evt)
			},
		},
		{
			name:      "exhausted",
			eventName: EventProviderQuotaExhausted,
			severity:  "critical",
			previous:  "warning",
			newStatus: "exhausted",
			notify: func(n *WebhookNotifier, evt QuotaStatusChangedEvent) {
				n.NotifyProviderQuotaExhausted(context.Background(), evt)
			},
		},
		{
			name:      "recovered",
			eventName: EventProviderQuotaRecovered,
			severity:  "info",
			previous:  "exhausted",
			newStatus: "available",
			notify: func(n *WebhookNotifier, evt QuotaStatusChangedEvent) {
				n.NotifyProviderQuotaRecovered(context.Background(), evt)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			defer client.Close()

			var receivedBody string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				receivedBody = string(body)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cfg := WebhookNotifierConfig{
				Targets: []WebhookTarget{
					{
						Name:      "default",
						Enabled:   true,
						URL:       server.URL,
						TimeoutMs: 1000,
						Body: `{"event":"{{.Event}}","severity":"{{.Severity}}",` +
							`"previous_status":"{{.Quota.PreviousStatus}}","new_status":"{{.Quota.NewStatus}}",` +
							`"usage_percent":{{.Quota.UsagePercent}},"warning_threshold_ratio":{{.Quota.WarningThresholdRatio}}}`,
					},
				},
				Subscriptions: []WebhookSubscription{
					{Event: EventProviderQuotaWarning, TargetNames: []string{"default"}},
					{Event: EventProviderQuotaExhausted, TargetNames: []string{"default"}},
					{Event: EventProviderQuotaRecovered, TargetNames: []string{"default"}},
				},
			}

			systemService := newTestSystemServiceWithWebhookConfig(t, client, cfg)
			notifier := NewWebhookNotifier(systemService, httpclient.NewHttpClient())

			tt.notify(notifier, QuotaStatusChangedEvent{
				ChannelID:      1,
				ChannelName:    "primary",
				PreviousStatus: tt.previous,
				NewStatus:      tt.newStatus,
				UsageRatio:     0.85,
				OccurredAt:     time.Now(),
			})

			require.JSONEq(t, fmt.Sprintf(
				`{"event":"%s","severity":"%s","previous_status":"%s","new_status":"%s","usage_percent":85,"warning_threshold_ratio":0.8}`,
				tt.eventName, tt.severity, tt.previous, tt.newStatus,
			), receivedBody)
		})
	}

	// Negative ratio renders usage_percent 0.
	t.Run("usage_percent_is_zero_for_negative_ratio", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		var receivedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			receivedBody = string(body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := WebhookNotifierConfig{
			Targets: []WebhookTarget{
				{
					Name:      "default",
					Enabled:   true,
					URL:       server.URL,
					TimeoutMs: 1000,
					Body:      `{"usage_percent":{{.Quota.UsagePercent}}}`,
				},
			},
			Subscriptions: []WebhookSubscription{
				{Event: EventProviderQuotaRecovered, TargetNames: []string{"default"}},
			},
		}

		systemService := newTestSystemServiceWithWebhookConfig(t, client, cfg)
		notifier := NewWebhookNotifier(systemService, httpclient.NewHttpClient())
		notifier.NotifyProviderQuotaRecovered(context.Background(), QuotaStatusChangedEvent{UsageRatio: -1, OccurredAt: time.Now()})

		require.JSONEq(t, `{"usage_percent":0}`, receivedBody)
	})
}

func TestRenderWebhookTemplate_NoTemplate(t *testing.T) {
	result, err := renderWebhookTemplate("plain text", WebhookRenderContext{})
	require.NoError(t, err)
	require.Equal(t, "plain text", result)
}

// defaultWebhookBodyTemplate mirrors DEFAULT_WEBHOOK_BODY_TEMPLATE in
// frontend/src/features/system/components/webhook-settings.tsx; change both
// together.
func TestRenderWebhookTemplate_DefaultTemplateBodyIsValidJSON(t *testing.T) {
	const body = `{
  "event": "{{.Event}}",
  "severity": "{{.Severity}}",
  "occurred_at": "{{.OccurredAt}}",
  "channel": {
    "id": {{.Channel.ID}},
    "name": "{{.Channel.Name}}",
    "provider": "{{.Channel.Provider}}",
    "status": "{{.Channel.Status}}"
  },
  "trigger": {
    "status_code": {{.Trigger.StatusCode}},
    "threshold": {{.Trigger.Threshold}},
    "actual_count": {{.Trigger.ActualCount}},
    "reason": "{{.Trigger.Reason}}"
  },
  "quota": {
    "previous_status": "{{.Quota.PreviousStatus}}",
    "new_status": "{{.Quota.NewStatus}}",
    "usage_ratio": {{.Quota.UsageRatio}},
    "usage_percent": {{.Quota.UsagePercent}},
    "next_reset_at": "{{.Quota.NextResetAt}}",
    "limits": {{.Quota.Limits | json}},
    "warning_threshold_ratio": {{.Quota.WarningThresholdRatio}}
  }
}`

	renderCtx := WebhookRenderContext{
		Event:      EventProviderQuotaWarning,
		Severity:   "warning",
		OccurredAt: "2026-04-11T04:00:00Z",
	}
	renderCtx.Channel.ID = 7
	renderCtx.Channel.Name = "primary"
	renderCtx.Channel.Provider = "anthropic"
	renderCtx.Channel.Status = "enabled"
	renderCtx.Quota.PreviousStatus = "available"
	renderCtx.Quota.NewStatus = "warning"
	renderCtx.Quota.UsageRatio = 0.9
	renderCtx.Quota.UsagePercent = 90
	renderCtx.Quota.NextResetAt = "2026-04-11T09:00:00Z"
	renderCtx.Quota.Limits = []string{"tokens (5h) 0.90 warning", "tokens (7d) 0.30 available"}
	renderCtx.Quota.WarningThresholdRatio = 0.8

	result, err := renderWebhookTemplate(body, renderCtx)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &payload), "rendered body must be valid JSON: %s", result)

	limits, ok := payload["quota"].(map[string]any)["limits"].([]any)
	require.True(t, ok, "limits must decode as a JSON array: %s", result)
	require.Equal(t, renderCtx.Quota.Limits, decodedLimits(t, limits))
}

// decodedLimits converts the unmarshaled array to []string, asserting every
// element is a JSON string.
func decodedLimits(t *testing.T, values []any) []string {
	t.Helper()

	decoded := make([]string, 0, len(values))
	for _, value := range values {
		s, ok := value.(string)
		require.True(t, ok, "limit entry must be a JSON string: %v", value)
		decoded = append(decoded, s)
	}

	return decoded
}
