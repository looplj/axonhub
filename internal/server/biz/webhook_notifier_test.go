package biz

import (
	"context"
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

func TestRenderWebhookTemplate_NoTemplate(t *testing.T) {
	result, err := renderWebhookTemplate("plain text", WebhookRenderContext{})
	require.NoError(t, err)
	require.Equal(t, "plain text", result)
}
