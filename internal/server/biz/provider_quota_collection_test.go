package biz

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

// expectQuotaWebhooksSet asserts that exactly the expected set of
// {event, prev, new} bodies arrives now, in any order. Each quota alert is
// dispatched in its own goroutine, so arrival order on the channel is
// timing-dependent; comparing sequentially (via expectQuotaWebhook) is the
// documented flake.
func expectQuotaWebhooksSet(t *testing.T, notifications chan string, want ...[3]string) {
	t.Helper()

	wantBodies := make(map[string]struct{}, len(want))
	for _, w := range want {
		wantBodies[fmt.Sprintf(`{"event":"%s","prev":"%s","new":"%s"}`, w[0], w[1], w[2])] = struct{}{}
	}

	seen := make(map[string]struct{}, len(wantBodies))
	for range len(wantBodies) {
		select {
		case body := <-notifications:
			if _, ok := wantBodies[body]; !ok {
				t.Fatalf("unexpected quota webhook: %s", body)
			}
			seen[body] = struct{}{}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for quota webhooks; missing %d of %d expected", len(wantBodies)-len(seen), len(wantBodies))
		}
	}

	if len(seen) != len(wantBodies) {
		t.Fatalf("expected quota webhooks contained duplicates; wanted %d distinct bodies", len(wantBodies))
	}
}

type countingQuotaChecker struct {
	calls        atomic.Int32
	providerType string
}

func (c *countingQuotaChecker) CheckQuota(context.Context, *ent.Channel) (provider_quota.QuotaData, error) {
	c.calls.Add(1)
	return provider_quota.QuotaData{Status: "available", ProviderType: c.providerType, Ready: true}, nil
}

func (c *countingQuotaChecker) SupportsChannel(*ent.Channel) bool {
	return true
}

type failingQuotaChecker struct {
	err error
}

func (c *failingQuotaChecker) CheckQuota(context.Context, *ent.Channel) (provider_quota.QuotaData, error) {
	return provider_quota.QuotaData{}, c.err
}

func (c *failingQuotaChecker) SupportsChannel(*ent.Channel) bool {
	return true
}

// statusQuotaChecker deterministically returns a fixed quota status (e.g.
// `unknown`) from CheckQuota, for exercising saveQuotaStatus paths that the
// always-available countingQuotaChecker cannot reach.
type statusQuotaChecker struct {
	calls        atomic.Int32
	providerType string
	status       string
}

func (c *statusQuotaChecker) CheckQuota(context.Context, *ent.Channel) (provider_quota.QuotaData, error) {
	c.calls.Add(1)
	return provider_quota.QuotaData{Status: c.status, ProviderType: c.providerType}, nil
}

func (c *statusQuotaChecker) SupportsChannel(*ent.Channel) bool {
	return true
}

// statusQuotaResetter is statusQuotaChecker with a passing reset, for
// ResetChannelQuotaNow refreshes that must observe a non-`available` status.
// When client is set, the reset also soft-deletes the channel row so the
// post-reset reload reports NotFound, reproducing a reload failure while the
// snapshot keeps its loaded edge.
type statusQuotaResetter struct {
	statusQuotaChecker

	client *ent.Client
}

func (c *statusQuotaResetter) ListResets(context.Context, *ent.Channel) (provider_quota.ResetList, error) {
	return provider_quota.ResetList{Supported: true}, nil
}

func (c *statusQuotaResetter) Reset(ctx context.Context, ch *ent.Channel) error {
	if c.client != nil {
		return c.client.Channel.DeleteOne(ch).Exec(ctx)
	}
	return nil
}

type countingQuotaResetter struct {
	countingQuotaChecker

	listCalls  atomic.Int32
	resetCalls atomic.Int32
	resets     provider_quota.ResetList
}

func (c *countingQuotaResetter) ListResets(context.Context, *ent.Channel) (provider_quota.ResetList, error) {
	c.listCalls.Add(1)
	return c.resets, nil
}

func (c *countingQuotaResetter) Reset(context.Context, *ent.Channel) error {
	c.resetCalls.Add(1)
	return nil
}

func setupProviderQuotaCollectionService(t *testing.T) (*ProviderQuotaService, *SystemService, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	systemService := NewSystemService(SystemServiceParams{
		Ent:         client,
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
	})
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	service := &ProviderQuotaService{
		AbstractService: &AbstractService{db: client},
		SystemService:   systemService,
		checkers:        make(map[string]provider_quota.QuotaChecker),
	}

	return service, systemService, ctx, client
}

func createProviderQuotaCollectionChannel(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	name string,
	channelType channel.Type,
) *ent.Channel {
	t.Helper()

	result, err := client.Channel.Create().
		SetName(name).
		SetType(channelType).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)
	return result
}

func TestProviderQuotaService_CheckChannelQuota_PersistsSanitizedErrorForInvalidCommandCodeCookie(t *testing.T) {
	service, _, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	channelEntity, err := client.Channel.Create().
		SetName("Command Code").
		SetType(channel.TypeCommandcode).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSettings(&objects.ChannelSettings{
			ProviderQuota: &objects.ChannelProviderQuotaSettings{
				CommandCode: &objects.CommandCodeQuotaSettings{
					AuthCookie: "commandcode_prod_.session_token=expired",
				},
			},
		}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ProviderQuotaStatus.Create().
		SetChannelID(channelEntity.ID).
		SetProviderType(providerquotastatus.ProviderTypeCommandcode).
		SetStatus(providerquotastatus.StatusAvailable).
		SetReady(true).
		SetQuotaData(map[string]any{"old": "quota"}).
		SetNextCheckAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	service.checkers["commandcode"] = &failingQuotaChecker{err: provider_quota.ErrInvalidCredentials}

	service.mu.Lock()
	service.runQuotaCheck(ctx, true)
	service.mu.Unlock()

	status, err := client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(channelEntity.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, providerquotastatus.StatusUnknown, status.Status)
	require.False(t, status.Ready)
	require.Equal(t, "check_failed", status.QuotaData["error_code"])
	require.NotContains(t, status.QuotaData, "error")
	require.NotContains(t, status.QuotaData, "old")
	cached, ok := service.quotaCache.Load(channelEntity.ID)
	require.True(t, ok)
	cachedStatus, ok := cached.(*QuotaChannelStatus)
	require.True(t, ok)
	require.Equal(t, providerquotastatus.StatusUnknown, cachedStatus.Status)
	require.False(t, cachedStatus.Ready)
	require.Empty(t, cachedStatus.Limits)
}

func TestProviderQuotaService_CheckChannelQuota_RemovesStaleStatusWithoutCommandCodeCredentials(t *testing.T) {
	service, _, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	channelEntity, err := client.Channel.Create().
		SetName("Command Code").
		SetType(channel.TypeCommandcode).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ProviderQuotaStatus.Create().
		SetChannelID(channelEntity.ID).
		SetProviderType(providerquotastatus.ProviderTypeCommandcode).
		SetStatus(providerquotastatus.StatusAvailable).
		SetReady(true).
		SetQuotaData(map[string]any{"old": "quota"}).
		SetNextCheckAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	service.mu.Lock()
	service.checkChannelQuota(ctx, quotaCheckGroup{channels: []*ent.Channel{channelEntity}}, time.Now())
	service.mu.Unlock()

	_, err = client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(channelEntity.ID)).
		Only(ctx)
	require.True(t, ent.IsNotFound(err))
	_, ok := service.quotaCache.Load(channelEntity.ID)
	require.False(t, ok)
}

// TestProviderQuotaService_CheckChannelQuota_CommandCodeCredentials locks the
// credential gate for Command Code: the account API key (/alpha/*) and the Studio
// session cookie (/internal/billing/*) are both accepted, so such channels are
// collected instead of having their quota invalidated.
func TestProviderQuotaService_CheckChannelQuota_CommandCodeCredentials(t *testing.T) {
	credentialCases := []struct {
		name        string
		credentials objects.ChannelCredentials
		settings    *objects.ChannelSettings
	}{
		{
			name:        "account api key",
			credentials: objects.ChannelCredentials{APIKey: "user_key"},
		},
		{
			name: "studio session cookie",
			settings: &objects.ChannelSettings{
				ProviderQuota: &objects.ChannelProviderQuotaSettings{
					CommandCode: &objects.CommandCodeQuotaSettings{
						AuthCookie: "commandcode_prod_.session_token=tok",
					},
				},
			},
		},
	}

	for _, tc := range credentialCases {
		t.Run(tc.name, func(t *testing.T) {
			service, _, ctx, client := setupProviderQuotaCollectionService(t)
			defer client.Close()

			builder := client.Channel.Create().
				SetName("Command Code").
				SetType(channel.TypeCommandcode).
				SetStatus(channel.StatusEnabled).
				SetCredentials(tc.credentials).
				SetSupportedModels([]string{"test-model"}).
				SetDefaultTestModel("test-model")
			if tc.settings != nil {
				builder = builder.SetSettings(tc.settings)
			}
			channelEntity, err := builder.Save(ctx)
			require.NoError(t, err)

			checker := &countingQuotaChecker{providerType: "commandcode"}
			service.checkers["commandcode"] = checker

			service.mu.Lock()
			service.checkChannelQuota(ctx, quotaCheckGroup{channels: []*ent.Channel{channelEntity}}, time.Now())
			service.mu.Unlock()

			require.Equal(t, int32(1), checker.calls.Load())

			status, err := client.ProviderQuotaStatus.Query().
				Where(providerquotastatus.ChannelIDEQ(channelEntity.ID)).
				Only(ctx)
			require.NoError(t, err)
			require.Equal(t, providerquotastatus.StatusAvailable, status.Status)
			require.True(t, status.Ready)
		})
	}
}

func TestProviderQuotaService_RunQuotaCheck_CollectionDisabledGlobally(t *testing.T) {
	service, systemService, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	minimaxChecker := &countingQuotaChecker{providerType: "minimax"}
	zhipuChecker := &countingQuotaChecker{providerType: "zhipu"}
	service.checkers["minimax"] = minimaxChecker
	service.checkers["zhipu"] = zhipuChecker
	createProviderQuotaCollectionChannel(t, ctx, client, "MiniMax", channel.TypeMinimax)
	createProviderQuotaCollectionChannel(t, ctx, client, "BigModel", channel.TypeZhipu)

	disabled := false
	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, &disabled, nil))

	service.runQuotaCheck(ctx, true)

	require.Zero(t, minimaxChecker.calls.Load())
	require.Zero(t, zhipuChecker.calls.Load())
}

func TestProviderQuotaService_RegisteredCheckersMatchSupportedProviderTypes(t *testing.T) {
	service := &ProviderQuotaService{
		checkers: make(map[string]provider_quota.QuotaChecker),
	}
	service.registerProviderQuotaSupport()

	registeredProviderTypes := make([]string, 0, len(service.checkers))
	for providerType := range service.checkers {
		registeredProviderTypes = append(registeredProviderTypes, providerType)
	}

	require.ElementsMatch(t, SupportedProviderQuotaTypes(), registeredProviderTypes)
}

func TestProviderQuotaService_RunQuotaCheck_CollectionDisabledForSelectedProviders(t *testing.T) {
	service, systemService, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	minimaxChecker := &countingQuotaChecker{providerType: "minimax"}
	zhipuChecker := &countingQuotaChecker{providerType: "zhipu"}
	codexChecker := &countingQuotaChecker{providerType: "codex"}
	service.checkers["minimax"] = minimaxChecker
	service.checkers["zhipu"] = zhipuChecker
	service.checkers["codex"] = codexChecker
	createProviderQuotaCollectionChannel(t, ctx, client, "MiniMax", channel.TypeMinimax)
	createProviderQuotaCollectionChannel(t, ctx, client, "BigModel", channel.TypeZhipu)
	codexChannel := createProviderQuotaCollectionChannel(t, ctx, client, "Codex", channel.TypeCodex)
	require.NoError(t, client.Channel.UpdateOne(codexChannel).
		SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token\"}"}).
		Exec(ctx))

	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, nil, []ProviderQuotaCollectionProvider{
		{Provider: "minimax", Enabled: false},
		{Provider: "zhipu", Enabled: false},
	}))

	service.runQuotaCheck(ctx, true)

	require.Zero(t, minimaxChecker.calls.Load())
	require.Zero(t, zhipuChecker.calls.Load())
	require.EqualValues(t, 1, codexChecker.calls.Load())
}

func TestProviderQuotaService_GetQuotaStatus_CollectionDisabledForProvider(t *testing.T) {
	service, systemService, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	service.quotaCache.Store(1, &QuotaChannelStatus{
		ProviderType: "minimax",
		Status:       "unknown",
		Ready:        false,
	})
	service.quotaCache.Store(2, &QuotaChannelStatus{
		ProviderType: "codex",
		Status:       "available",
		Ready:        true,
	})
	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, nil, []ProviderQuotaCollectionProvider{
		{Provider: "minimax", Enabled: false},
	}))

	require.Nil(t, service.GetQuotaStatus(ctx, 1))
	require.NotNil(t, service.GetQuotaStatus(ctx, 2))
}

func TestProviderQuotaService_ResetChannelQuotaNow_CollectionDisabledForCodex(t *testing.T) {
	service, systemService, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	codexChecker := &countingQuotaResetter{
		countingQuotaChecker: countingQuotaChecker{providerType: "codex"},
	}
	service.checkers["codex"] = codexChecker
	channelEntity := createProviderQuotaCollectionChannel(t, ctx, client, "Codex", channel.TypeCodex)
	require.NoError(t, client.Channel.UpdateOne(channelEntity).
		SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token\"}"}).
		Exec(ctx))
	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, nil, []ProviderQuotaCollectionProvider{
		{Provider: "codex", Enabled: false},
	}))

	err := service.ResetChannelQuotaNow(ctx, channelEntity.ID)

	require.ErrorContains(t, err, "provider quota collection is disabled for codex")
	require.Zero(t, codexChecker.calls.Load())
	require.Zero(t, codexChecker.resetCalls.Load())
}

func TestProviderQuotaService_ListResets_ReturnsUnsupportedForCheckerWithoutResetter(t *testing.T) {
	service, _, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	service.checkers["minimax"] = &countingQuotaChecker{providerType: "minimax"}
	channelEntity := createProviderQuotaCollectionChannel(t, ctx, client, "MiniMax", channel.TypeMinimax)

	resets, err := service.ListResets(ctx, channelEntity.ID)

	require.NoError(t, err)
	require.False(t, resets.Supported)
	require.Empty(t, resets.Resets)
}

func TestProviderQuotaService_ResetChannelQuotaNow_ReturnsUnsupportedForCheckerWithoutResetter(t *testing.T) {
	service, _, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	service.checkers["minimax"] = &countingQuotaChecker{providerType: "minimax"}
	channelEntity := createProviderQuotaCollectionChannel(t, ctx, client, "MiniMax", channel.TypeMinimax)

	err := service.ResetChannelQuotaNow(ctx, channelEntity.ID)

	require.Error(t, err)
	require.True(t, errors.Is(err, provider_quota.ErrResetUnsupported))
}

func TestProviderQuotaService_ListResets_UsesOptionalResetter(t *testing.T) {
	service, _, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	resetter := &countingQuotaResetter{
		countingQuotaChecker: countingQuotaChecker{providerType: "minimax"},
		resets: provider_quota.ResetList{
			Resets: []provider_quota.Reset{
				{ID: "reset-1", Status: "available"},
			},
		},
	}
	service.checkers["minimax"] = resetter
	channelEntity := createProviderQuotaCollectionChannel(t, ctx, client, "MiniMax", channel.TypeMinimax)

	resets, err := service.ListResets(ctx, channelEntity.ID)

	require.NoError(t, err)
	require.True(t, resets.Supported)
	require.Len(t, resets.Resets, 1)
	require.EqualValues(t, 1, resetter.listCalls.Load())
}

// resettingQuotaResetter simulates a provider reset whose effect lands before
// the service can observe it: it soft-deletes the channel row before returning
// nil, so the post-reset channel reload in ResetChannelQuotaNow reports
// NotFound and the refresh proceeds with the stale pre-reset snapshot.
type resettingQuotaResetter struct {
	countingQuotaResetter

	client *ent.Client
}

func (c *resettingQuotaResetter) Reset(ctx context.Context, ch *ent.Channel) error {
	c.resetCalls.Add(1)
	return c.client.Channel.DeleteOne(ch).Exec(ctx)
}

// TestProviderQuotaService_ResetChannelQuotaNow_SucceedsWhenReloadFails locks
// the log-and-proceed behavior when the post-reset channel reload fails: the
// reset is reported as successful, the refresh still runs, the fresh status
// is persisted, and the duplicate transition webhook caused by the stale
// pre-reset edge is suppressed — a genuine transition away from the
// last-known status is still delivered (see
// quotaCheckGroup.suppressWebhooks).
func TestProviderQuotaService_ResetChannelQuotaNow_SucceedsWhenReloadFails(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	svc, notifications, _ := newTestQuotaWebhookHarness(t, client)
	svc.checkers["codex"] = &resettingQuotaResetter{
		countingQuotaResetter: countingQuotaResetter{
			countingQuotaChecker: countingQuotaChecker{providerType: "codex"},
			resets:               provider_quota.ResetList{Supported: true},
		},
		client: client,
	}

	ch := createProviderQuotaCollectionChannel(t, ctx, client, "Codex", channel.TypeCodex)
	require.NoError(t, client.Channel.UpdateOne(ch).
		SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token\"}"}).
		Exec(ctx))

	// A pre-primed exhausted row: the reset restored the quota upstream, so
	// the post-reset refresh reports a genuine exhausted → available
	// transition the failed-reload path must still deliver.
	seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusExhausted, map[string]any{})

	require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch.ID))

	resetter := svc.checkers["codex"].(*resettingQuotaResetter)
	require.Equal(t, int32(1), resetter.resetCalls.Load())
	require.Equal(t, int32(1), resetter.calls.Load())

	persisted, err := client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)
	require.True(t, persisted.Ready)

	// The refresh recomputed exhausted → available against the last-known
	// (stale) status: a genuine transition is not a duplicate, so the
	// recovery fires exactly once even on the failed-reload path.
	expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "exhausted", "available")
	expectNoQuotaWebhook(t, notifications)

	// Suppression must skip only the notification: the fresh status was
	// persisted above and must also be in the in-memory cache, proving
	// updateQuotaCache ran.
	cached := svc.GetQuotaStatus(ctx, ch.ID)
	require.NotNil(t, cached)
	require.Equal(t, providerquotastatus.StatusAvailable, cached.Status)
	require.True(t, cached.Ready)

	t.Run("same_status_refresh_stays_silent", func(t *testing.T) {
		// A same-status refresh (available → available) on the failed-reload
		// path emits nothing: suppression drops the duplicate edge but has
		// no transition to keep, and a same-status save fires no event.
		ch2 := createProviderQuotaCollectionChannel(t, ctx, client, "Codex 2", channel.TypeCodex)
		require.NoError(t, client.Channel.UpdateOne(ch2).
			SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token2\"}"}).
			Exec(ctx))

		checker := &statusQuotaResetter{
			statusQuotaChecker: statusQuotaChecker{providerType: "codex", status: "available"},
			client:             client,
		}
		svc.checkers["codex"] = checker

		seedQuotaRow(t, ctx, client, ch2.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusAvailable, map[string]any{})
		require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch2.ID))

		require.Equal(t, int32(1), checker.calls.Load())

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch2.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)
		require.True(t, persisted.Ready)

		expectNoQuotaWebhook(t, notifications)
	})

	t.Run("unknown_refresh_carries_previous_status", func(t *testing.T) {
		ch3 := createProviderQuotaCollectionChannel(t, ctx, client, "Codex 3", channel.TypeCodex)
		require.NoError(t, client.Channel.UpdateOne(ch3).
			SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token3\"}"}).
			Exec(ctx))

		// A deterministic unknown result (not an error) is required to reach
		// the _previous_status carry in saveQuotaStatus; a checker error goes
		// to saveQuotaError instead.
		checker := &statusQuotaResetter{
			statusQuotaChecker: statusQuotaChecker{providerType: "codex", status: "unknown"},
			client:             client,
		}
		svc.checkers["codex"] = checker

		seedQuotaRow(t, ctx, client, ch3.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusExhausted, map[string]any{})
		require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch3.ID))

		require.Equal(t, int32(1), checker.calls.Load())

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch3.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.StatusUnknown, persisted.Status)
		require.False(t, persisted.Ready)
		// The suppressed refresh must still carry the last known non-unknown
		// status into the persisted quota_data so a later recovery resolves
		// exhausted → available.
		require.Equal(t, providerquotastatus.StatusExhausted.String(), persisted.QuotaData["_previous_status"])

		// No known→unknown transition fires an event, suppressed or not.
		expectNoQuotaWebhook(t, notifications)
	})

	t.Run("mismatched_provider_cache_is_ignored", func(t *testing.T) {
		// The cache holds an entry for the same channel from a DIFFERENT
		// provider: a channel that switched providers must not compare the
		// refresh against the old provider's status, so the suppressed
		// refresh must fall back to the loaded edge's previous status and
		// emit only the transition that edge actually implies.
		ch4 := createProviderQuotaCollectionChannel(t, ctx, client, "Codex 4", channel.TypeCodex)
		require.NoError(t, client.Channel.UpdateOne(ch4).
			SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token4\"}"}).
			Exec(ctx))

		checker := &statusQuotaResetter{
			statusQuotaChecker: statusQuotaChecker{providerType: "codex", status: "available"},
			client:             client,
		}
		svc.checkers["codex"] = checker

		// Loaded edge: exhausted. If the refresh (mistakenly) compared the
		// fresh available against the minimax cache's available, it would be
		// silent; against the edge's exhausted it must emit the recovery.
		seedQuotaRow(t, ctx, client, ch4.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusExhausted, map[string]any{})

		svc.quotaCache.Store(ch4.ID, &QuotaChannelStatus{
			ProviderType:    string(providerquotastatus.ProviderTypeMinimax),
			Status:          providerquotastatus.StatusAvailable,
			Ready:           true,
			LastKnownStatus: "available",
		})
		require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch4.ID))

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch4.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)

		// Edge-driven exhausted → available, emitted exactly once.
		expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "exhausted", "available")
		expectNoQuotaWebhook(t, notifications)
	})
}

// savingQuotaResetter models the duplicate-replay race: while the provider
// reset is in flight (i.e. after ResetChannelQuotaNow already loaded the
// pre-reset channel snapshot), a concurrent scheduled check saves a fresh
// status (concurrentStatus) unsuppressed — emitting its transition and
// writing the in-memory cache — and only then does the reset soft-delete the
// channel row, so the post-reset reload fails and the refresh reuses the
// stale snapshot. The embedded statusQuotaResetter's standing `status` is
// what ResetChannelQuotaNow's refresh itself checks (the reset CLI result of
// this provider).
type savingQuotaResetter struct {
	statusQuotaResetter

	svc              *ProviderQuotaService
	concurrentStatus string
}

func (c *savingQuotaResetter) Reset(ctx context.Context, ch *ent.Channel) error {
	// The row still exists here, so this loads the same edge the pre-reset
	// snapshot holds (stale: exhausted).
	chEdge, err := c.client.Channel.Query().
		Where(channel.IDEQ(ch.ID)).WithProviderQuotaStatus().Only(ctx)
	if err == nil {
		// The concurrent scheduled check, unsuppressed.
		c.svc.saveQuotaStatus(ctx, chEdge, "codex", "", provider_quota.QuotaData{
			ProviderType: string(providerquotastatus.ProviderTypeCodex),
			Status:       c.concurrentStatus,
			Ready:        c.concurrentStatus != "unknown",
		}, time.Now(), false)
	}
	return c.client.Channel.DeleteOne(ch).Exec(ctx)
}

// stagedSavingQuotaResetter runs a sequence of unsuppressed in-flight
// scheduled checks (stages) during the provider reset, each re-querying the
// loaded edge before saving exactly like the production poll loop, then
// deletes the row so the post-reset reload fails.
type stagedSavingQuotaResetter struct {
	statusQuotaResetter

	svc    *ProviderQuotaService
	stages []string
	// afterStages runs after the staged saves and before the row deletion,
	// to assert the row state the staged checks left behind.
	afterStages func(ctx context.Context)
}

func (c *stagedSavingQuotaResetter) Reset(ctx context.Context, ch *ent.Channel) error {
	for _, status := range c.stages {
		edge, err := c.client.Channel.Query().
			Where(channel.IDEQ(ch.ID)).WithProviderQuotaStatus().Only(ctx)
		if err != nil {
			return err
		}
		c.svc.saveQuotaStatus(ctx, edge, "codex", "", provider_quota.QuotaData{
			ProviderType: string(providerquotastatus.ProviderTypeCodex),
			Status:       status,
			Ready:        status != "unknown",
		}, time.Now(), false)
	}
	if c.afterStages != nil {
		c.afterStages(ctx)
	}
	return c.client.Channel.DeleteOne(ch).Exec(ctx)
}

// TestProviderQuotaService_ResetChannelQuotaNow_FailedReloadPrefersCacheOverStaleEdge
// is the regression test for the duplicate-replay path: a scheduled check
// saves and emits a transition (exhausted → available recovery) while a
// provider reset is in flight; the reset succeeds upstream but the post-reset
// reload fails, so the refresh reuses the pre-reset channel snapshot. With
// the stale loaded edge, the refresh would compare the fresh available
// against the edge's exhausted and re-emit the same recovery the in-flight
// check already sent. The in-memory cache (written by the in-flight check
// under the service mutex) is newer, so on the failure-reload path it wins
// over the stale edge: same status → silent, a genuine change against the
// newest known status → emitted exactly once.
func TestProviderQuotaService_ResetChannelQuotaNow_FailedReloadPrefersCacheOverStaleEdge(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	svc, notifications, _ := newTestQuotaWebhookHarness(t, client)

	t.Run("same_status_refresh_is_silent", func(t *testing.T) {
		ch := createProviderQuotaCollectionChannel(t, ctx, client, "Codex", channel.TypeCodex)
		require.NoError(t, client.Channel.UpdateOne(ch).
			SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token\"}"}).
			Exec(ctx))

		// Pre-reset state: exhausted row, so the snapshot carries an
		// exhausted loaded edge.
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusExhausted, map[string]any{})

		// The in-flight scheduled check saves available (emitting the
		// exhausted → available recovery and seeding the cache) during the
		// reset, then the row is deleted so the post-reset reload fails;
		// the refresh itself reports available as well.
		svc.checkers["codex"] = &savingQuotaResetter{
			statusQuotaResetter: statusQuotaResetter{
				statusQuotaChecker: statusQuotaChecker{providerType: "codex", status: "available"},
				client:             client,
			},
			svc:              svc,
			concurrentStatus: "available",
		}

		require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch.ID))

		// The in-flight check's emission landed once, with the cache freshly
		// seeded: without both, the assertion below would not exercise the
		// cache-over-stale-edge branch.
		expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "exhausted", "available")
		cached := svc.GetQuotaStatus(ctx, ch.ID)
		require.NotNil(t, cached)
		require.Equal(t, providerquotastatus.StatusAvailable, cached.Status)

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)
		require.True(t, persisted.Ready)

		// The refresh re-reported available: against the newest known status
		// (the cache's available) that is no genuine transition, so it must
		// be silent — re-emitting the recovery would be the duplicate the
		// fix removes (the stale edge's exhausted would have re-fired it).
		expectNoQuotaWebhook(t, notifications)
	})

	t.Run("genuine_change_against_cache_emitted_once", func(t *testing.T) {
		// Same setup, fresh channel: the in-flight check saves and emits the
		// recovery, then the refresh (failed reload) reports exhausted.
		// Against the newest known status (the cache's available) that is a
		// genuine transition and must be emitted exactly once.
		ch2 := createProviderQuotaCollectionChannel(t, ctx, client, "Codex 2", channel.TypeCodex)
		require.NoError(t, client.Channel.UpdateOne(ch2).
			SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token2\"}"}).
			Exec(ctx))
		seedQuotaRow(t, ctx, client, ch2.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusExhausted, map[string]any{})

		// In-flight check saves available; the refresh reports exhausted.
		svc.checkers["codex"] = &savingQuotaResetter{
			statusQuotaResetter: statusQuotaResetter{
				statusQuotaChecker: statusQuotaChecker{providerType: "codex", status: "exhausted"},
				client:             client,
			},
			svc:              svc,
			concurrentStatus: "available",
		}

		require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch2.ID))

		// The two alerts are dispatched in separate goroutines, so their
		// arrival order on the channel is timing-dependent: compare as a set.
		expectQuotaWebhooksSet(t, notifications,
			[3]string{EventProviderQuotaRecovered, "exhausted", "available"},
			[3]string{EventProviderQuotaExhausted, "available", "exhausted"},
		)
		expectNoQuotaWebhook(t, notifications)
	})
}

// TestProviderQuotaService_ResetChannelQuotaNow_FailedReloadUnknownCacheUsesFreshCarry
// is the regression test for the cached-`unknown` carry: the pre-reset edge
// is `unknown` carrying an old `_previous_status` (exhausted), an in-flight
// scheduled check saves and emits `available` during the reset (dropping the
// old carry from the persisted row and seeding the cache with the fresh
// last-known status), then a second save parks the channel at `unknown`
// again. A cached `unknown` carries no `_previous_status` metadata, so the
// failed-reload refresh must resolve its last-known status from the freshest
// source — the cache's recorded carry (#1's `available`) — not the stale
// pre-reset edge's (`exhausted`): a refresh reporting `available` is then
// silent (no replayed recovery), and one reporting `exhausted` fires the
// genuine `available → exhausted` transition against the fresh carry.
func TestProviderQuotaService_ResetChannelQuotaNow_FailedReloadUnknownCacheUsesFreshCarry(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	svc, notifications, _ := newTestQuotaWebhookHarness(t, client)

	t.Run("same_status_refresh_is_silent", func(t *testing.T) {
		ch := createProviderQuotaCollectionChannel(t, ctx, client, "Codex", channel.TypeCodex)
		require.NoError(t, client.Channel.UpdateOne(ch).
			SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token\"}"}).
			Exec(ctx))

		// Pre-reset state: unknown row carrying the old exhausted status.
		// The snapshot loads this edge, whose carry is STALE for the
		// refresh: the in-flight check below supersedes it.
		seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusUnknown, map[string]any{
			"_previous_status": providerquotastatus.StatusExhausted.String(),
		})

		// The in-flight scheduled check #1 saves available (emitting the
		// exhausted → available recovery from the edge's carry and seeding
		// the cache), #2 saves unknown (the carry must advance to the
		// fresh available), then the row is deleted so the post-reset
		// reload fails; the refresh itself reports available as well.
		svc.checkers["codex"] = &stagedSavingQuotaResetter{
			statusQuotaResetter: statusQuotaResetter{
				statusQuotaChecker: statusQuotaChecker{providerType: "codex", status: "available"},
				client:             client,
			},
			svc:    svc,
			stages: []string{"available", "unknown"},
			// In-flight check #2 saved unknown on top of #1's available:
			// its _previous_status must carry the FRESH available, not the
			// pre-reset edge's stale exhausted.
			afterStages: func(ctx context.Context) {
				row, err := client.ProviderQuotaStatus.Query().
					Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
				require.NoError(t, err)
				require.Equal(t, providerquotastatus.StatusUnknown, row.Status)
				require.Equal(t, providerquotastatus.StatusAvailable.String(), row.QuotaData["_previous_status"])
			},
		}

		require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch.ID))

		// The in-flight check's recovery: the transition resolves from the
		// edge's carried exhausted (unknown → available), while the payload's
		// prev carries the raw pre-save status, as in every
		// unknown-interlude event.
		expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "unknown", "available")
		// The suppressed refresh's save ran last, so the final cache entry is
		// the refresh's available (the in-flight stage #2's unknown is no
		// longer the newest known status).
		cached := svc.GetQuotaStatus(ctx, ch.ID)
		require.NotNil(t, cached)
		require.Equal(t, providerquotastatus.StatusAvailable, cached.Status)

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)
		require.True(t, persisted.Ready)
		// A fresh known status drops the carry key entirely.
		require.NotContains(t, persisted.QuotaData, "_previous_status")

		// Against the newest known status (the in-flight stage #2 save's
		// fresh available, recorded in the cache) the refresh's available is
		// no genuine transition: no duplicate recovery, no exhausted →
		// available re-emit from the stale edge's carry.
		expectNoQuotaWebhook(t, notifications)
	})

	t.Run("genuine_change_against_fresh_carry_emitted_once", func(t *testing.T) {
		ch2 := createProviderQuotaCollectionChannel(t, ctx, client, "Codex 2", channel.TypeCodex)
		require.NoError(t, client.Channel.UpdateOne(ch2).
			SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token2\"}"}).
			Exec(ctx))
		seedQuotaRow(t, ctx, client, ch2.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusUnknown, map[string]any{
			"_previous_status": providerquotastatus.StatusExhausted.String(),
		})

		// In-flight check saves available (fresh carry), then unknown; the
		// refresh reports exhausted.
		svc.checkers["codex"] = &stagedSavingQuotaResetter{
			statusQuotaResetter: statusQuotaResetter{
				statusQuotaChecker: statusQuotaChecker{providerType: "codex", status: "exhausted"},
				client:             client,
			},
			svc:    svc,
			stages: []string{"available", "unknown"},
			afterStages: func(ctx context.Context) {
				row, err := client.ProviderQuotaStatus.Query().
					Where(providerquotastatus.ChannelIDEQ(ch2.ID)).Only(ctx)
				require.NoError(t, err)
				require.Equal(t, providerquotastatus.StatusUnknown, row.Status)
				require.Equal(t, providerquotastatus.StatusAvailable.String(), row.QuotaData["_previous_status"])
			},
		}

		require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch2.ID))

		// Exactly one event against the FRESH carry (available → exhausted)
		// — not a replay of the same recovery the in-flight check emitted.
		// The two alerts are dispatched in separate goroutines, so their
		// arrival order on the channel is timing-dependent: compare as a set.
		expectQuotaWebhooksSet(t, notifications,
			[3]string{EventProviderQuotaRecovered, "unknown", "available"},
			[3]string{EventProviderQuotaExhausted, "unknown", "exhausted"},
		)
		expectNoQuotaWebhook(t, notifications)
	})
}

// TestProviderQuotaService_ResetChannelQuotaNow_EmitsRecoveryWhenReloadSucceeds
// is the positive control for quotaCheckGroup.suppressWebhooks: when the
// post-reset channel reload succeeds, suppression is off and the exhausted →
// available recovery must be delivered.
func TestProviderQuotaService_ResetChannelQuotaNow_EmitsRecoveryWhenReloadSucceeds(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	svc, notifications, _ := newTestQuotaWebhookHarness(t, client)
	svc.checkers["codex"] = &countingQuotaResetter{
		countingQuotaChecker: countingQuotaChecker{providerType: "codex"},
		resets:               provider_quota.ResetList{Supported: true},
	}

	ch := createProviderQuotaCollectionChannel(t, ctx, client, "Codex", channel.TypeCodex)
	require.NoError(t, client.Channel.UpdateOne(ch).
		SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token\"}"}).
		Exec(ctx))

	// A pre-primed exhausted row: with a passing resetter the post-reset
	// reload succeeds, suppression stays off, and the refresh emits the
	// exhausted → available recovery exactly once.
	seedQuotaRow(t, ctx, client, ch.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusExhausted, map[string]any{})

	require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch.ID))

	resetter := svc.checkers["codex"].(*countingQuotaResetter)
	require.Equal(t, int32(1), resetter.resetCalls.Load())
	require.Equal(t, int32(1), resetter.calls.Load())

	persisted, err := client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(ch.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)
	require.True(t, persisted.Ready)

	expectQuotaWebhook(t, notifications, EventProviderQuotaRecovered, "exhausted", "available")
	expectNoQuotaWebhook(t, notifications)
}
