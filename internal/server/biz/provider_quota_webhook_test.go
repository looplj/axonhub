package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm/httpclient"
)

// newTestQuotaWebhookHarness wires a real webhook sink, a real
// ProviderQuotaService with a real notifier, and returns helpers to save
// statuses and await deliveries.
func newTestQuotaWebhookHarness(t *testing.T, client *ent.Client) (*ProviderQuotaService, chan string, func(ch *ent.Channel) *ent.Channel) {
	t.Helper()

	notifications := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		notifications <- string(body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := WebhookNotifierConfig{
		Targets: []WebhookTarget{
			{
				Name:      "default",
				Enabled:   true,
				URL:       server.URL,
				TimeoutMs: 1000,
				Body:      `{"event":"{{.Event}}","prev":"{{.Quota.PreviousStatus}}","new":"{{.Quota.NewStatus}}"}`,
			},
		},
		Subscriptions: []WebhookSubscription{
			{Event: EventProviderQuotaWarning, TargetNames: []string{"default"}},
			{Event: EventProviderQuotaExhausted, TargetNames: []string{"default"}},
			{Event: EventProviderQuotaRecovered, TargetNames: []string{"default"}},
		},
	}

	systemService := newTestSystemServiceWithWebhookConfig(t, client, cfg)

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	svc := NewProviderQuotaService(ProviderQuotaServiceParams{
		Ent:             client,
		SystemService:   systemService,
		HttpClient:      httpclient.NewHttpClient(),
		WebhookNotifier: NewWebhookNotifier(systemService, httpclient.NewHttpClient()),
	})

	// saveQuotaStatus does not refresh channel edges after its upsert, so a
	// sequence of saves must re-query the loaded edge between saves, mirroring
	// the production poll loop.
	requery := func(ch *ent.Channel) *ent.Channel {
		t.Helper()
		updated, err := client.Channel.Query().Where(channel.IDEQ(ch.ID)).WithProviderQuotaStatus().Only(ctx)
		require.NoError(t, err)

		return updated
	}

	return svc, notifications, requery
}

func expectQuotaWebhook(t *testing.T, notifications chan string, wantEvent, wantPrev, wantNew string) {
	t.Helper()

	select {
	case body := <-notifications:
		require.JSONEq(t, fmt.Sprintf(`{"event":"%s","prev":"%s","new":"%s"}`, wantEvent, wantPrev, wantNew), body)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for quota webhook %s (%s→%s)", wantEvent, wantPrev, wantNew)
	}
}

func expectNoQuotaWebhook(t *testing.T, notifications chan string) {
	t.Helper()

	select {
	case body := <-notifications:
		t.Fatalf("unexpected quota webhook: %s", body)
	case <-time.After(100 * time.Millisecond):
	}
}

func seedQuotaRow(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	channelID int,
	pt providerquotastatus.ProviderType,
	status providerquotastatus.Status,
	quotaData map[string]any,
) {
	t.Helper()

	_, err := client.ProviderQuotaStatus.Create().
		SetChannelID(channelID).
		SetProviderType(pt).
		SetStatus(status).
		SetReady(status == providerquotastatus.StatusAvailable || status == providerquotastatus.StatusWarning).
		SetQuotaData(quotaData).
		SetNextCheckAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)
}

func createQuotaWebhookTestChannel(t *testing.T, ctx context.Context, client *ent.Client) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetName("Claude Code").
		SetType(channel.TypeClaudecode).
		SetStatus(channel.StatusEnabled).
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	return ch
}

func TestProviderQuotaService_QuotaStatusChangeWebhooks(t *testing.T) {
	t.Run("available_to_warning_to_exhausted_to_available", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusAvailable, map[string]any{})

		svc, notifications, requery := newTestQuotaWebhookHarness(t, client)

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusWarning),
			Ready:        true,
		}, time.Now(), false)
		expectQuotaWebhook(t, notifications, EventProviderQuotaWarning, "available", "warning")

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusExhausted),
			Ready:        false,
		}, time.Now(), false)
		// exhausted only: no separate warning event.
		expectQuotaWebhook(t, notifications, EventProviderQuotaExhausted, "warning", "exhausted")
		expectNoQuotaWebhook(t, notifications)

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusAvailable),
			Ready:        true,
		}, time.Now(), false)
		expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "exhausted", "available")

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)
		require.True(t, persisted.Ready)
	})

	t.Run("warning_to_available", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusWarning, map[string]any{})

		svc, notifications, requery := newTestQuotaWebhookHarness(t, client)

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusAvailable),
			Ready:        true,
		}, time.Now(), false)
		expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "warning", "available")
	})

	t.Run("unknown_interlude_recovers_from_row", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		// The row was last known warning; the check then failed, leaving the
		// status unknown with _previous_status carried in quota_data.
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusUnknown, map[string]any{
			"_previous_status": "warning",
		})

		svc, notifications, requery := newTestQuotaWebhookHarness(t, client)

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusAvailable),
			Ready:        true,
		}, time.Now(), false)
		expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "unknown", "available")

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)
		// The carried status is only written when the previous status is
		// non-unknown; an unknown previous leaves no _previous_status behind.
		require.NotContains(t, persisted.QuotaData, "_previous_status")
	})

	t.Run("unknown_interlude_exhausts_from_row", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusUnknown, map[string]any{
			"_previous_status": "available",
		})

		svc, notifications, requery := newTestQuotaWebhookHarness(t, client)

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusExhausted),
			Ready:        false,
		}, time.Now(), false)
		expectQuotaWebhook(t, notifications, EventProviderQuotaExhausted, "unknown", "exhausted")
	})

	t.Run("first_ever_status_is_silent", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		// No status row: the service loads an empty cache, so the first save
		// has no previous status of any kind.
		svc, notifications, _ := newTestQuotaWebhookHarness(t, client)

		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusWarning),
			Ready:        true,
		}, time.Now(), false)
		expectNoQuotaWebhook(t, notifications)

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.StatusWarning, persisted.Status)
	})

	t.Run("same_status_save_is_silent", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusWarning, map[string]any{})

		svc, notifications, requery := newTestQuotaWebhookHarness(t, client)

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusWarning),
			Ready:        true,
		}, time.Now(), false)
		expectNoQuotaWebhook(t, notifications)
	})

	t.Run("consecutive_unknown_rows_keep_carry", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusWarning, map[string]any{})

		svc, notifications, requery := newTestQuotaWebhookHarness(t, client)

		// Two consecutive checks return unknown: the first one leaves the
		// row at unknown carrying the previous status, the second must not
		// drop the carry.
		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusUnknown),
			Ready:        false,
		}, time.Now(), false)
		expectNoQuotaWebhook(t, notifications)

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusUnknown),
			Ready:        false,
		}, time.Now(), false)
		expectNoQuotaWebhook(t, notifications)

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, "warning", persisted.QuotaData["_previous_status"])

		// Recovery after the degraded interlude fires the event.
		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusAvailable),
			Ready:        true,
		}, time.Now(), false)
		expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "unknown", "available")
	})

	t.Run("credential_reset_preserves_carry", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusExhausted, map[string]any{})

		svc, notifications, requery := newTestQuotaWebhookHarness(t, client)

		// An invalid-credentials failure resets the row to unknown; the last
		// known status must survive for the post-re-auth recovery event.
		ch = requery(ch)
		svc.saveQuotaError(ctx, ch, "claudecode", "", fmt.Errorf("bad key: %w", provider_quota.ErrInvalidCredentials), 1, time.Now())
		expectNoQuotaWebhook(t, notifications)

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, "exhausted", persisted.QuotaData["_previous_status"])

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
			ProviderType: "claudecode",
			Status:       string(providerquotastatus.StatusAvailable),
			Ready:        true,
		}, time.Now(), false)
		expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "unknown", "available")
	})

	t.Run("check_failure_is_silent_and_preserves_status", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusExhausted, map[string]any{})

		svc, notifications, requery := newTestQuotaWebhookHarness(t, client)

		ch = requery(ch)
		svc.saveQuotaError(ctx, ch, "claudecode", "", errors.New("boom"), 1, time.Now())
		expectNoQuotaWebhook(t, notifications)

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.StatusExhausted, persisted.Status)
		require.Equal(t, float64(1), persisted.QuotaData["error_count"])
		require.Equal(t, "check_failed", persisted.QuotaData["error_code"])
	})

	t.Run("provider_identity_change_does_not_fire_transition", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
		ch := createQuotaWebhookTestChannel(t, ctx, client)
		// The channel switches providers while the stored row still carries
		// the old provider's exhausted status: the new provider's first
		// status is a first-ever detection for that provider, so nothing fires.
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusExhausted, map[string]any{})

		svc, notifications, requery := newTestQuotaWebhookHarness(t, client)

		ch = requery(ch)
		svc.saveQuotaStatus(ctx, ch, "nanogpt", "", provider_quota.QuotaData{
			ProviderType: "nanogpt",
			Status:       string(providerquotastatus.StatusAvailable),
			Ready:        true,
		}, time.Now(), false)
		expectNoQuotaWebhook(t, notifications)

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.ProviderTypeNanogpt, persisted.ProviderType)
		require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)
	})
}

// newTestQuotaWebhookLimitsHarness is newTestQuotaWebhookHarness with a body
// template that captures the rendered {{.Quota.Limits | json}} array.
func newTestQuotaWebhookLimitsHarness(t *testing.T, client *ent.Client) (*ProviderQuotaService, chan string) {
	t.Helper()

	notifications := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		notifications <- string(body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := WebhookNotifierConfig{
		Targets: []WebhookTarget{
			{
				Name:      "default",
				Enabled:   true,
				URL:       server.URL,
				TimeoutMs: 1000,
				Body:      `{"event":"{{.Event}}","limits":{{.Quota.Limits | json}}}`,
			},
		},
		Subscriptions: []WebhookSubscription{
			{Event: EventProviderQuotaWarning, TargetNames: []string{"default"}},
			{Event: EventProviderQuotaExhausted, TargetNames: []string{"default"}},
			{Event: EventProviderQuotaRecovered, TargetNames: []string{"default"}},
		},
	}

	systemService := newTestSystemServiceWithWebhookConfig(t, client, cfg)

	svc := NewProviderQuotaService(ProviderQuotaServiceParams{
		Ent:             client,
		SystemService:   systemService,
		HttpClient:      httpclient.NewHttpClient(),
		WebhookNotifier: NewWebhookNotifier(systemService, httpclient.NewHttpClient()),
	})

	return svc, notifications
}

// TestProviderQuotaService_QuotaWebhookLimitsLines locks the rendered
// `limits` array in quota webhook payloads: multi-account limits carry an
// `[account: X]` suffix keyed for dedup, and account-less limits stay
// byte-identical to the pre-suffix format.
func TestProviderQuotaService_QuotaWebhookLimitsLines(t *testing.T) {
	tests := []struct {
		name   string
		limits []provider_quota.QuotaLimitStatus
		want   []string
	}{
		{
			name: "single_account_limit_carrying_account_suffix",
			limits: []provider_quota.QuotaLimitStatus{
				{
					Type:       provider_quota.QuotaLimitTypeToken,
					Window:     provider_quota.QuotaWindowWeekly,
					UsageRatio: 0.90,
					Status:     string(providerquotastatus.StatusWarning),
					Account:    "0001",
				},
			},
			want: []string{"token (weekly) 0.90 warning [account: 0001]"},
		},
		{
			name: "same_window_distinct_accounts_stay_separate_in_input_order",
			limits: []provider_quota.QuotaLimitStatus{
				{
					Type:       provider_quota.QuotaLimitTypeToken,
					Window:     provider_quota.QuotaWindowWeekly,
					UsageRatio: 0.90,
					Status:     string(providerquotastatus.StatusWarning),
					Account:    "0001",
				},
				{
					Type:       provider_quota.QuotaLimitTypeToken,
					Window:     provider_quota.QuotaWindowWeekly,
					UsageRatio: 0.90,
					Status:     string(providerquotastatus.StatusWarning),
					Account:    "0002",
				},
			},
			want: []string{
				"token (weekly) 0.90 warning [account: 0001]",
				"token (weekly) 0.90 warning [account: 0002]",
			},
		},
		{
			name: "accountless_limit_keeps_legacy_line_format",
			limits: []provider_quota.QuotaLimitStatus{
				{
					Type:       provider_quota.QuotaLimitTypeToken,
					Window:     provider_quota.QuotaWindowWeekly,
					UsageRatio: 0.90,
					Status:     string(providerquotastatus.StatusWarning),
					Account:    "",
				},
			},
			want: []string{"token (weekly) 0.90 warning"},
		},
		{
			name: "identical_accountless_limits_deduplicate",
			limits: []provider_quota.QuotaLimitStatus{
				{
					Type:       provider_quota.QuotaLimitTypeToken,
					Window:     provider_quota.QuotaWindowWeekly,
					UsageRatio: 0.90,
					Status:     string(providerquotastatus.StatusWarning),
					Account:    "",
				},
				{
					Type:       provider_quota.QuotaLimitTypeToken,
					Window:     provider_quota.QuotaWindowWeekly,
					UsageRatio: 0.90,
					Status:     string(providerquotastatus.StatusWarning),
					Account:    "",
				},
			},
			want: []string{"token (weekly) 0.90 warning"},
		},
		{
			name: "identical_suffixed_limits_deduplicate",
			limits: []provider_quota.QuotaLimitStatus{
				{
					Type:       provider_quota.QuotaLimitTypeToken,
					Window:     provider_quota.QuotaWindowWeekly,
					UsageRatio: 0.90,
					Status:     string(providerquotastatus.StatusWarning),
					Account:    "0001",
				},
				{
					Type:       provider_quota.QuotaLimitTypeToken,
					Window:     provider_quota.QuotaWindowWeekly,
					UsageRatio: 0.90,
					Status:     string(providerquotastatus.StatusWarning),
					Account:    "0001",
				},
			},
			want: []string{"token (weekly) 0.90 warning [account: 0001]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			defer client.Close()

			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
			ch := createQuotaWebhookTestChannel(t, ctx, client)
			// An available previous status makes the warning save an
			// available → warning transition, so every case renders a payload.
			seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeClaudecode, providerquotastatus.StatusAvailable, map[string]any{})
			updated, err := client.Channel.Query().Where(channel.IDEQ(ch.ID)).WithProviderQuotaStatus().Only(ctx)
			require.NoError(t, err)
			ch = updated

			svc, notifications := newTestQuotaWebhookLimitsHarness(t, client)

			svc.saveQuotaStatus(ctx, ch, "claudecode", "", provider_quota.QuotaData{
				ProviderType: "claudecode",
				Status:       string(providerquotastatus.StatusWarning),
				Ready:        true,
				Limits:       tt.limits,
			}, time.Now(), false)

			var received struct {
				Event  string   `json:"event"`
				Limits []string `json:"limits"`
			}
			select {
			case body := <-notifications:
				require.NoError(t, json.Unmarshal([]byte(body), &received))
				require.Equal(t, EventProviderQuotaWarning, received.Event)
				require.Equal(t, tt.want, received.Limits)
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for quota webhook %s", EventProviderQuotaWarning)
			}
		})
	}
}
