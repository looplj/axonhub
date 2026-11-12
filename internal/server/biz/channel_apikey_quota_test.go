// 职责: 覆盖额度服务与渠道生命周期之间的逐 key 禁用协调行为。
// 关联文件:
//
//	internal/server/biz/channel_apikey_quota.go
//	internal/server/biz/channel_apikey.go
//	internal/server/biz/provider_quota.go
//	internal/server/biz/provider_quota/apikey_quota.go
//
// 遵循的 Rules:
//
//	.agents/rules/biz-services.md
//	.agents/rules/go-test.md
//
// 注意事项:
//
//	这些用例断言真实的禁用记录、渠道状态与缓存重建结果，而不是内部函数调用次数。
package biz

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm/httpclient"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

const (
	quotaTestKeyA = "opencode-key-aaaa-1001"
	quotaTestKeyB = "opencode-key-bbbb-2002"
)

// quotaKeyTestServer is a mutable fake OpenCode Go usage endpoint keyed by the
// Authorization header. A key whose payload is unset fails the request, which is
// how an outage is reproduced.
type quotaKeyTestServer struct {
	mu       sync.Mutex
	payloads map[string]string
	failures map[string]bool
	requests []string
}

func newQuotaKeyTestServer() *quotaKeyTestServer {
	return &quotaKeyTestServer{
		payloads: make(map[string]string),
		failures: make(map[string]bool),
	}
}

// usage sets one key's rolling/weekly/monthly usage percentages.
func (s *quotaKeyTestServer) usage(key string, rolling, weekly, monthly float64) {
	resetAt := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.failures[key] = false
	s.payloads[key] = fmt.Sprintf(
		`{"usage":{"rolling":{"percent":%v,"resetsAt":%q},"weekly":{"percent":%v,"resetsAt":%q},"monthly":{"percent":%v,"resetsAt":%q}}}`,
		rolling, resetAt, weekly, resetAt, monthly, resetAt,
	)
}

// fail makes one key's usage request fail.
func (s *quotaKeyTestServer) fail(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.failures[key] = true
}

func (s *quotaKeyTestServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.requests)
}

func (s *quotaKeyTestServer) client() *httpclient.HttpClient {
	return httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			key := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")

			s.mu.Lock()
			s.requests = append(s.requests, key)
			failed := s.failures[key]
			body, known := s.payloads[key]
			s.mu.Unlock()

			if failed || !known {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":"provider unavailable"}`)),
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})
}

// quotaKeyTestEnv is the production wiring of both services over one SQLite
// fixture: the channel service owns the reconciler the quota service calls.
type quotaKeyTestEnv struct {
	client   *ent.Client
	ctx      context.Context
	channels *ChannelService
	quota    *ProviderQuotaService
	server   *quotaKeyTestServer
}

func setupQuotaKeyTestEnv(t *testing.T) *quotaKeyTestEnv {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	channels := newTestChannelService(client)
	server := newQuotaKeyTestServer()

	quotaSvc := &ProviderQuotaService{
		AbstractService: &AbstractService{db: client},
		SystemService:   channels.SystemService,
		WebhookNotifier: channels.WebhookNotifier,
		httpClient:      server.client(),
		checkers: map[string]provider_quota.QuotaChecker{
			"opencode_go": provider_quota.NewOpenCodeGoQuotaChecker(server.client()),
		},
		checkInterval: 5 * time.Minute,
	}
	quotaSvc.SetQuotaKeyReconciler(channels)

	return &quotaKeyTestEnv{
		client:   client,
		ctx:      ctx,
		channels: channels,
		quota:    quotaSvc,
		server:   server,
	}
}

// createOpenCodeChannel stores an OpenCode Go channel with the given keys.
func (e *quotaKeyTestEnv) createOpenCodeChannel(t *testing.T, keys ...string) *ent.Channel {
	t.Helper()

	ch, err := e.client.Channel.Create().
		SetName("OpenCode Go").
		SetType(channel.TypeOpencodeGo).
		SetBaseURL("https://opencode.ai/zen/go/v1").
		SetCredentials(objects.ChannelCredentials{APIKeys: keys}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SetStatus(channel.StatusEnabled).
		Save(e.ctx)
	require.NoError(t, err)

	return ch
}

// poll runs one forced quota collection pass, exactly like the scheduled task.
func (e *quotaKeyTestEnv) poll(t *testing.T) {
	t.Helper()

	e.quota.mu.Lock()
	e.quota.runQuotaCheck(e.ctx, true)
	e.quota.mu.Unlock()
}

func (e *quotaKeyTestEnv) reload(t *testing.T, id int) *ent.Channel {
	t.Helper()

	ch, err := e.client.Channel.Get(e.ctx, id)
	require.NoError(t, err)

	return ch
}

func (e *quotaKeyTestEnv) reloadCache(t *testing.T) {
	t.Helper()

	require.NoError(t, e.channels.enabledChannelsCache.Load(e.ctx, true))
}

// enabledKeysOf reports the keys the request path may select for the channel.
func enabledKeysOf(ch *ent.Channel) []string {
	return ch.Credentials.GetEnabledAPIKeys(ch.DisabledAPIKeys)
}

// quotaOwnedKeys reports the keys currently disabled by a quota observation.
func quotaOwnedKeys(ch *ent.Channel) []string {
	keys := make([]string, 0, len(ch.DisabledAPIKeys))
	for _, record := range ch.DisabledAPIKeys {
		if record.QuotaDisabled {
			keys = append(keys, record.Key)
		}
	}

	return keys
}

// apiKeyQuotaRows decodes the persisted per-key rows, accepting both the
// in-process and the JSON-round-tripped slice shapes.
func apiKeyQuotaRows(t *testing.T, data map[string]any) []map[string]any {
	t.Helper()

	raw, ok := data["_api_key_quotas"]
	require.True(t, ok, "the per-key view is persisted")

	if rows, ok := raw.([]map[string]any); ok {
		return rows
	}

	anyRows, ok := raw.([]any)
	require.True(t, ok, "unexpected _api_key_quotas shape %T", raw)

	rows := make([]map[string]any, 0, len(anyRows))
	for _, entry := range anyRows {
		row, ok := entry.(map[string]any)
		require.True(t, ok)
		rows = append(rows, row)
	}

	return rows
}

func TestQuotaKey_ExhaustedKeyIsDisabledAndOthersKeepServing(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	env.server.usage(quotaTestKeyA, 100, 10, 10)
	env.server.usage(quotaTestKeyB, 12, 30, 35)

	env.poll(t)

	reloaded := env.reload(t, ch.ID)
	require.Equal(t, channel.StatusEnabled, reloaded.Status)
	require.Nil(t, reloaded.AutoDisabledAt)
	require.Equal(t, []string{quotaTestKeyA}, quotaOwnedKeys(reloaded), "only the exhausted key is parked")
	require.Equal(t, []string{quotaTestKeyB}, enabledKeysOf(reloaded), "requests must select the healthy key")

	for _, record := range reloaded.DisabledAPIKeys {
		require.Equal(t, 0, record.ErrorCode)
		require.Equal(t, quotaKeyDisabledReason, record.Reason)
		require.Nil(t, record.ExpiresAt, "a quota disable never expires on its own")
	}

	// The serving cache is rebuilt from the same state: the exhausted key must
	// not come back through a refresh.
	env.reloadCache(t)
	cached := env.channels.GetEnabledChannel(ch.ID)
	require.NotNil(t, cached)
	require.Equal(t, []string{quotaTestKeyB}, cached.GetEnabledAPIKeys())

	persisted, err := env.client.ProviderQuotaStatus.Query().All(env.ctx)
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	require.Equal(t, "available", persisted[0].Status.String())
	accounts := apiKeyQuotaRows(t, persisted[0].QuotaData)
	require.Len(t, accounts, 2)
	for _, account := range accounts {
		require.NotEmpty(t, account["ref"])
		require.NotEmpty(t, account["suffix"])
		require.Contains(t, account, "index")
		require.Contains(t, account, "quotaDisabled")
		require.NotContains(t, account, "key")
	}
	require.NotContains(t, fmt.Sprint(persisted[0].QuotaData), quotaTestKeyA, "the payload never stores the configured key")
}

func TestQuotaKey_AllKeysExhaustedRecoversWithTheRecoveredKey(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	env.server.usage(quotaTestKeyA, 100, 10, 10)
	env.server.usage(quotaTestKeyB, 10, 100, 10)
	env.poll(t)

	disabled := env.reload(t, ch.ID)
	require.Equal(t, channel.StatusDisabled, disabled.Status)
	require.NotNil(t, disabled.AutoDisabledAt, "the all-keys auto-disable marker is what allows recovery")
	require.NotNil(t, disabled.ErrorMessage)
	require.True(t, strings.HasPrefix(*disabled.ErrorMessage, allKeysDisabledErrorPrefix))
	require.ElementsMatch(t, []string{quotaTestKeyA, quotaTestKeyB}, quotaOwnedKeys(disabled))
	require.True(t, isQuotaPollableChannel(disabled), "a quota-disabled channel stays pollable so it can recover")

	env.server.usage(quotaTestKeyB, 10, 20, 10)
	env.poll(t)

	recovered := env.reload(t, ch.ID)
	require.Equal(t, channel.StatusEnabled, recovered.Status)
	require.Nil(t, recovered.AutoDisabledAt)
	require.Nil(t, recovered.ErrorMessage)
	require.Equal(t, []string{quotaTestKeyA}, quotaOwnedKeys(recovered), "only the recovered key is released")
	require.Equal(t, []string{quotaTestKeyB}, enabledKeysOf(recovered))

	env.reloadCache(t)
	cached := env.channels.GetEnabledChannel(ch.ID)
	require.NotNil(t, cached)
	require.Equal(t, []string{quotaTestKeyB}, cached.GetEnabledAPIKeys())
}

func TestQuotaKey_ManualDisableSurvivesRecovery(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	env.server.usage(quotaTestKeyA, 10, 10, 10)
	env.server.usage(quotaTestKeyB, 10, 10, 10)

	require.NoError(t, env.channels.DisableAPIKey(env.ctx, ch.ID, quotaTestKeyA, 401, "invalid credential"))

	env.poll(t)

	reloaded := env.reload(t, ch.ID)
	require.Len(t, reloaded.DisabledAPIKeys, 1)
	require.Equal(t, 401, reloaded.DisabledAPIKeys[0].ErrorCode)
	require.False(t, reloaded.DisabledAPIKeys[0].QuotaDisabled, "a manual disable keeps its ownership")
	require.Empty(t, quotaOwnedKeys(reloaded), "a healthy funded key is never parked by quota")

	env.server.usage(quotaTestKeyA, 100, 100, 100)
	env.poll(t)

	afterExhaustion := env.reload(t, ch.ID)
	require.Len(t, afterExhaustion.DisabledAPIKeys, 1)
	require.False(t, afterExhaustion.DisabledAPIKeys[0].QuotaDisabled, "an exhausted but manually disabled key stays manual")

	env.server.usage(quotaTestKeyA, 10, 10, 10)
	env.poll(t)

	afterRecovery := env.reload(t, ch.ID)
	require.Len(t, afterRecovery.DisabledAPIKeys, 1, "quota recovery never clears a manual disable")
	require.Equal(t, quotaTestKeyA, afterRecovery.DisabledAPIKeys[0].Key)
	require.Equal(t, []string{quotaTestKeyB}, enabledKeysOf(afterRecovery))
}

func TestQuotaKey_KeepDisabledTransfersOwnership(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	env.server.usage(quotaTestKeyA, 100, 10, 10)
	env.server.usage(quotaTestKeyB, 10, 10, 10)
	env.poll(t)

	require.Equal(t, []string{quotaTestKeyA}, quotaOwnedKeys(env.reload(t, ch.ID)))

	// The operator asks to keep the key disabled for good.
	require.NoError(t, env.channels.DisableAPIKey(env.ctx, ch.ID, quotaTestKeyA, 0, "keep disabled"))

	kept := env.reload(t, ch.ID)
	require.Len(t, kept.DisabledAPIKeys, 1)
	require.False(t, kept.DisabledAPIKeys[0].QuotaDisabled, "the explicit request takes ownership")
	require.Equal(t, "keep disabled", kept.DisabledAPIKeys[0].Reason)

	env.server.usage(quotaTestKeyA, 10, 10, 10)
	env.poll(t)

	afterRecovery := env.reload(t, ch.ID)
	require.Len(t, afterRecovery.DisabledAPIKeys, 1, "quota recovery must not release a manual record")
	require.False(t, afterRecovery.DisabledAPIKeys[0].QuotaDisabled)
	require.Equal(t, []string{quotaTestKeyB}, enabledKeysOf(afterRecovery))
}

func TestQuotaKey_ManualChannelDisableSurvivesRecovery(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	env.server.usage(quotaTestKeyA, 100, 10, 10)
	env.server.usage(quotaTestKeyB, 100, 10, 10)
	env.poll(t)
	require.Equal(t, channel.StatusDisabled, env.reload(t, ch.ID).Status)

	// A later manual channel disable clears the auto-disable markers.
	_, err := env.client.Channel.UpdateOneID(ch.ID).
		SetStatus(channel.StatusDisabled).
		ClearErrorMessage().
		ClearAutoDisabledAt().
		Save(env.ctx)
	require.NoError(t, err)

	env.server.usage(quotaTestKeyA, 5, 5, 5)
	env.server.usage(quotaTestKeyB, 5, 5, 5)
	env.poll(t)

	manuallyDisabled := env.reload(t, ch.ID)
	require.Equal(t, channel.StatusDisabled, manuallyDisabled.Status)
	require.Nil(t, manuallyDisabled.AutoDisabledAt)
	require.False(t, isQuotaPollableChannel(manuallyDisabled), "a manually disabled channel is never polled")

	// Re-enabling the keys cannot resurrect a channel the operator turned off.
	require.NoError(t, env.channels.EnableAllAPIKeys(env.ctx, ch.ID))

	reenabled := env.reload(t, ch.ID)
	require.Equal(t, channel.StatusDisabled, reenabled.Status, "a manual channel disable outlives the key recovery")
}

func TestQuotaKey_OutageNeverReleasesAQuotaDisable(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	env.server.usage(quotaTestKeyA, 100, 10, 10)
	env.server.usage(quotaTestKeyB, 10, 10, 10)
	env.poll(t)
	require.Equal(t, []string{quotaTestKeyA}, quotaOwnedKeys(env.reload(t, ch.ID)))

	// An outage reports nothing: the parked key must stay parked rather than be
	// guessed back into service.
	env.server.fail(quotaTestKeyA)
	env.server.fail(quotaTestKeyB)
	env.poll(t)

	duringOutage := env.reload(t, ch.ID)
	require.Equal(t, []string{quotaTestKeyA}, quotaOwnedKeys(duringOutage))

	// One readable but unknown alternative must not release anything either.
	env.server.usage(quotaTestKeyB, 10, 10, 10)
	env.poll(t)
	require.Equal(t, []string{quotaTestKeyA}, quotaOwnedKeys(env.reload(t, ch.ID)))
}

func TestQuotaKey_SettingsOptOutReleasesOnlyQuotaRecords(t *testing.T) {
	tests := []struct {
		name string
		// disableCollection turns the global or the provider collection switch off,
		// or opts the effective routing mode out of quota gating.
		disableCollection func(t *testing.T, env *quotaKeyTestEnv, ch *ent.Channel)
		// collectionOff marks the settings that stop collection entirely, so the
		// cleanup cannot make a provider request. A routing opt-out keeps polling
		// for display; it just must not park a key.
		collectionOff bool
	}{
		{
			name:          "global collection disabled",
			collectionOff: true,
			disableCollection: func(t *testing.T, env *quotaKeyTestEnv, _ *ent.Channel) {
				t.Helper()
				disabled := false
				require.NoError(t, env.channels.SystemService.UpdateProviderQuotaCollectionSettings(env.ctx, &disabled, nil))
			},
		},
		{
			name:          "provider collection disabled",
			collectionOff: true,
			disableCollection: func(t *testing.T, env *quotaKeyTestEnv, _ *ent.Channel) {
				t.Helper()
				require.NoError(t, env.channels.SystemService.UpdateProviderQuotaCollectionSettings(env.ctx, nil,
					[]ProviderQuotaCollectionProvider{{Provider: "opencode_go", Enabled: false}}))
			},
		},
		{
			name: "inherited ignore_quota mode",
			disableCollection: func(t *testing.T, env *quotaKeyTestEnv, _ *ent.Channel) {
				t.Helper()
				require.NoError(t, env.channels.SystemService.SetQuotaRoutingSettings(env.ctx,
					QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeIgnoreQuota}))
			},
		},
		{
			name: "channel ignore_quota override",
			disableCollection: func(t *testing.T, env *quotaKeyTestEnv, ch *ent.Channel) {
				t.Helper()
				_, err := env.client.Channel.UpdateOneID(ch.ID).SetSettings(&objects.ChannelSettings{
					QuotaRoutingMode: objects.QuotaRoutingModeIgnoreQuota,
				}).Save(env.ctx)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupQuotaKeyTestEnv(t)
			ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

			env.server.usage(quotaTestKeyA, 100, 10, 10)
			env.server.usage(quotaTestKeyB, 100, 10, 10)
			env.poll(t)

			disabled := env.reload(t, ch.ID)
			require.Equal(t, channel.StatusDisabled, disabled.Status)
			require.Len(t, quotaOwnedKeys(disabled), 2)

			// A manual record has to survive the opt-out cleanup.
			require.NoError(t, env.channels.DisableAPIKey(env.ctx, ch.ID, quotaTestKeyB, 401, "invalid credential"))
			owned := env.reload(t, ch.ID)
			require.Equal(t, []string{quotaTestKeyA}, quotaOwnedKeys(owned), "the takeover left only the other key quota-owned")

			tt.disableCollection(t, env, ch)
			requestsBefore := env.server.requestCount()
			env.poll(t)
			if tt.collectionOff {
				require.Equal(t, requestsBefore, env.server.requestCount(), "a collection opt-out makes no provider request")
			}

			cleaned := env.reload(t, ch.ID)
			require.Empty(t, quotaOwnedKeys(cleaned), "only quota-owned records are released")
			require.Equal(t, channel.StatusEnabled, cleaned.Status, "the last key is usable again")
			require.Nil(t, cleaned.AutoDisabledAt)

			remaining := disabledKeyValues(cleaned.DisabledAPIKeys)
			require.Equal(t, []string{quotaTestKeyB}, remaining, "the manual record stays")
		})
	}
}

func TestQuotaKey_StaleSnapshotIsDiscarded(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA)
	env.server.usage(quotaTestKeyA, 100, 10, 10)

	snapshot := env.reload(t, ch.ID)

	// The credentials change while the quota request is in flight.
	_, err := env.client.Channel.UpdateOneID(ch.ID).
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{quotaTestKeyB}}).
		Save(env.ctx)
	require.NoError(t, err)

	accounts := []provider_quota.APIKeyQuota{{
		Key:   quotaTestKeyA,
		Ref:   "stale-ref",
		Index: 1,
		Data:  provider_quota.QuotaData{Status: "exhausted", ProviderType: "opencode_go"},
	}}

	_, err = env.channels.ReconcileQuotaKeys(env.ctx, snapshot, accounts)
	require.ErrorIs(t, err, errQuotaKeySnapshotStale)

	current := env.reload(t, ch.ID)
	require.Empty(t, current.DisabledAPIKeys, "a stale result persists nothing")
	require.Equal(t, channel.StatusEnabled, current.Status)
}

func TestQuotaKey_ReconciliationIsIdempotent(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)
	env.server.usage(quotaTestKeyA, 100, 10, 10)
	env.server.usage(quotaTestKeyB, 10, 10, 10)

	env.poll(t)
	first := env.reload(t, ch.ID)
	require.Len(t, first.DisabledAPIKeys, 1)

	// A second pass over the same observation writes nothing and changes nothing.
	env.poll(t)
	second := env.reload(t, ch.ID)
	require.Equal(t, first.DisabledAPIKeys, second.DisabledAPIKeys)
	require.Equal(t, first.UpdatedAt, second.UpdatedAt, "an unchanged batch skips the write")
}

// TestQuotaKey_CompareAndSetDiscardsAConcurrentEdit pins the write predicate the
// reconciler uses: a writer whose channel row moved since it was read matches no
// row, so a concurrent manual edit wins and the next poll retries.
func TestQuotaKey_CompareAndSetDiscardsAConcurrentEdit(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	manual := []objects.DisabledAPIKey{{Key: quotaTestKeyB, DisabledAt: time.Now(), ErrorCode: 401, Reason: "invalid credential"}}
	_, err := env.client.Channel.UpdateOneID(ch.ID).SetDisabledAPIKeys(manual).Save(env.ctx)
	require.NoError(t, err)

	current := env.reload(t, ch.ID)

	affected, err := env.client.Channel.Update().
		Where(channel.IDEQ(ch.ID), channel.UpdatedAtEQ(current.UpdatedAt.Add(-time.Hour))).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: quotaTestKeyA, DisabledAt: time.Now(), QuotaDisabled: true}}).
		Save(env.ctx)
	require.NoError(t, err)
	require.Zero(t, affected, "a writer holding a stale row matches nothing")

	final := env.reload(t, ch.ID)
	require.Len(t, final.DisabledAPIKeys, 1, "the concurrent manual record survives")
	require.Equal(t, quotaTestKeyB, final.DisabledAPIKeys[0].Key)
	require.Equal(t, 401, final.DisabledAPIKeys[0].ErrorCode)
	require.False(t, final.DisabledAPIKeys[0].QuotaDisabled)
	require.Equal(t, channel.StatusEnabled, final.Status)
}

// TestQuotaKey_ExpiredManualRecordDoesNotBlockQuotaOwnership pins the ownership
// rule: an expired manual disable disables nothing, so it must not stop quota
// from parking the same exhausted key. The expired record itself survives until
// the expiry cleanup removes it.
func TestQuotaKey_ExpiredManualRecordDoesNotBlockQuotaOwnership(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	expiredAt := time.Now().Add(-time.Hour)
	_, err := env.client.Channel.UpdateOneID(ch.ID).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{{
			Key:        quotaTestKeyA,
			DisabledAt: time.Now().Add(-2 * time.Hour),
			ErrorCode:  429,
			Reason:     "temporary",
			ExpiresAt:  &expiredAt,
		}}).
		Save(env.ctx)
	require.NoError(t, err)

	env.server.usage(quotaTestKeyA, 100, 10, 10)
	env.server.usage(quotaTestKeyB, 10, 10, 10)
	env.poll(t)

	reloaded := env.reload(t, ch.ID)
	require.Equal(t, []string{quotaTestKeyA}, quotaOwnedKeys(reloaded), "the exhausted key is parked despite the expired record")
	require.Len(t, reloaded.DisabledAPIKeys, 2, "the expired record is preserved for the expiry cleanup")
	require.Equal(t, []string{quotaTestKeyB}, enabledKeysOf(reloaded))
}

// TestQuotaKey_FailureRowsRequireTheSameChannelIdentity pins the error-path
// guard: failed key rows are only persisted for the identity they describe.
func TestQuotaKey_FailureRowsRequireTheSameChannelIdentity(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	accounts := []provider_quota.APIKeyQuota{{
		Key:       quotaTestKeyA,
		Ref:       "ref-a",
		Index:     1,
		ErrorCode: provider_quota.APIKeyQuotaErrorCheckFailed,
		Data:      provider_quota.QuotaData{Status: "unknown", ProviderType: "opencode_go"},
	}}

	require.Len(t, env.quota.failureAccountsFor(env.ctx, ch, accounts), 1, "an unchanged channel keeps its failure rows")

	// The credentials change while the request is in flight.
	_, err := env.client.Channel.UpdateOneID(ch.ID).
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{quotaTestKeyB}}).
		Save(env.ctx)
	require.NoError(t, err)

	require.Empty(t, env.quota.failureAccountsFor(env.ctx, ch, accounts), "a changed identity drops the stale rows")
	require.Empty(t, env.quota.failureAccountsFor(env.ctx, ch, nil))
}
