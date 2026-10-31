package biz

import (
	"context"
	"errors"
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
type statusQuotaResetter struct {
	statusQuotaChecker
}

func (c *statusQuotaResetter) ListResets(context.Context, *ent.Channel) (provider_quota.ResetList, error) {
	return provider_quota.ResetList{Supported: true}, nil
}

func (c *statusQuotaResetter) Reset(context.Context, *ent.Channel) error {
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
	if err := c.client.Channel.DeleteOne(ch).Exec(ctx); err != nil {
		return err
	}
	return nil
}

// TestProviderQuotaService_ResetChannelQuotaNow_SucceedsWhenReloadFails locks
// the log-and-proceed behavior when the post-reset channel reload fails: the
// reset is reported as successful, the refresh still runs, and the fresh
// status is persisted with the transition webhook suppressed (see
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

	// A pre-primed exhausted row: without suppression the post-reset refresh
	// (exhausted → available) would emit provider.quota.recovered.
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

	// The stale pre-reset edge made the refresh recompute exhausted →
	// available; the duplicate recovery must not be delivered.
	expectNoQuotaWebhook(t, notifications)

	// Suppression must skip only the notification: the fresh status was
	// persisted above and must also be in the in-memory cache, proving
	// updateQuotaCache ran.
	cached := svc.GetQuotaStatus(ctx, ch.ID)
	require.NotNil(t, cached)
	require.Equal(t, providerquotastatus.StatusAvailable, cached.Status)
	require.True(t, cached.Ready)

	t.Run("unknown_refresh_carries_previous_status", func(t *testing.T) {
		ch2 := createProviderQuotaCollectionChannel(t, ctx, client, "Codex 2", channel.TypeCodex)
		require.NoError(t, client.Channel.UpdateOne(ch2).
			SetCredentials(objects.ChannelCredentials{APIKey: "{\"access_token\":\"test-token2\"}"}).
			Exec(ctx))

		// A deterministic unknown result (not an error) is required to reach
		// the _previous_status carry in saveQuotaStatus; a checker error goes
		// to saveQuotaError instead.
		checker := &statusQuotaResetter{statusQuotaChecker: statusQuotaChecker{providerType: "codex", status: "unknown"}}
		svc.checkers["codex"] = checker

		seedQuotaRow(t, ctx, client, ch2.ID, providerquotastatus.ProviderTypeCodex, providerquotastatus.StatusExhausted, map[string]any{})
		require.NoError(t, svc.ResetChannelQuotaNow(ctx, ch2.ID))

		require.Equal(t, int32(1), checker.calls.Load())

		persisted, err := client.ProviderQuotaStatus.Query().
			Where(providerquotastatus.ChannelIDEQ(ch2.ID)).Only(ctx)
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
