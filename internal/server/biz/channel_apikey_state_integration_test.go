package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

// newReloadableChannelService returns the test service with a real HTTP client
// so the actual build path (buildChannelWithTransformer) can run, while keeping
// the real reload func so Load() rebuilds snapshots the way the server does.
func newReloadableChannelService(t *testing.T, client *ent.Client) *ChannelService {
	t.Helper()

	svc := newTestChannelService(client)
	svc.httpClient = httpclient.NewHttpClient()

	return svc
}

// setChannelStrategy writes the multi-key strategy onto a stored channel.
func setChannelStrategy(t *testing.T, ctx context.Context, client *ent.Client, id int, strategy string, per ...int) {
	t.Helper()

	settings := &objects.ChannelSettings{APIKeyStrategy: &strategy}
	if len(per) > 0 {
		count := per[0]
		settings.APIKeyRoundRobinSwitchAfter = &count
	}

	_, err := client.Channel.UpdateOneID(id).SetSettings(settings).Save(ctx)
	require.NoError(t, err)
}

// The real build path must attach the shared state for exactly the strategies
// that remember something between requests. If it ever stops doing that, every
// one of them silently falls back to provider-local state and loses its place on
// each channel reload.
func TestKeySelectionState_AttachedByRealChannelBuild(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	svc := newReloadableChannelService(t, client)

	cases := []struct {
		strategy  string
		wantState bool
	}{
		{"", true}, // empty means sticky
		{objects.APIKeyStrategySticky, true},
		{objects.APIKeyStrategyRandom, false},
		{objects.APIKeyStrategyRoundRobin, true},
		{objects.APIKeyStrategyRoundRobinSuccess, true},
		{objects.APIKeyStrategyPriority, false},
		{objects.APIKeyStrategyFixed, true},
	}

	for i, tc := range cases {
		strategy := tc.strategy
		entity := &ent.Channel{
			ID:          i + 1,
			Type:        channel.TypeOpenai,
			BaseURL:     "https://api.openai.com",
			Credentials: objects.ChannelCredentials{APIKeys: []string{"k1", "k2"}},
			Settings:    &objects.ChannelSettings{APIKeyStrategy: &strategy},
		}

		built, err := svc.buildChannelWithTransformer(entity)
		require.NoErrorf(t, err, "strategy %q", tc.strategy)

		if tc.wantState {
			require.NotNilf(t, built.apiKeyState, "strategy %q must attach the shared state", tc.strategy)
		} else {
			require.Nilf(t, built.apiKeyState, "strategy %q must not attach shared state", tc.strategy)
		}
	}
}

// A real cache reload rebuilds every snapshot. The round-robin rotation must
// carry on from where it was instead of restarting at the first key.
func TestRoundRobinState_SurvivesRealCacheReload(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	svc := newReloadableChannelService(t, client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "rr-state-reload", []string{"k1", "k2", "k3"})
	setChannelStrategy(t, ctx, client, ch.ID, objects.APIKeyStrategyRoundRobin, 1)

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))

	first := svc.GetEnabledChannel(ch.ID)
	require.NotNil(t, first)
	require.NotNil(t, first.apiKeyState, "the real reload must attach the shared state")

	provider := NewRoundRobinKeyProvider(first, 1)
	require.Equal(t, "k1", provider.Get(ctx))
	require.Equal(t, "k2", provider.Get(ctx))

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))

	second := svc.GetEnabledChannel(ch.ID)
	require.NotNil(t, second)
	require.NotSame(t, first, second, "the reload must produce a new snapshot")

	require.Equal(t, "k3", NewRoundRobinKeyProvider(second, 1).Get(ctx))
	require.Equal(t, "k1", NewRoundRobinKeyProvider(second, 1).Get(ctx))
}

// The same reload must not move an ongoing sticky session onto another key.
func TestStickyState_SurvivesRealCacheReload(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	svc := newReloadableChannelService(t, client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "sticky-state-reload", []string{"k1", "k2", "k3"})
	setChannelStrategy(t, ctx, client, ch.ID, objects.APIKeyStrategySticky)

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))

	first := svc.GetEnabledChannel(ch.ID)
	require.NotNil(t, first)
	require.NotNil(t, first.apiKeyState)

	traceCtx := contexts.WithTrace(ctx, &ent.Trace{TraceID: "trace-reload"})
	selected := newSharedStickyKeyProvider(first).Get(traceCtx)

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))

	second := svc.GetEnabledChannel(ch.ID)
	require.NotNil(t, second)
	require.NotSame(t, first, second)

	require.Equal(t, selected, newSharedStickyKeyProvider(second).Get(traceCtx))
}

// The fixed strategy remembers one key: disabling a different key must not move
// it, disabling the remembered one must walk forward.
func TestFixedState_SurvivesKeyDisableAndReload(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	svc := newReloadableChannelService(t, client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "fixed-state-disable", []string{"k1", "k2", "k3"})
	setChannelStrategy(t, ctx, client, ch.ID, objects.APIKeyStrategyFixed)

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))

	first := svc.GetEnabledChannel(ch.ID)
	require.NotNil(t, first)
	require.NotNil(t, first.apiKeyState)

	setFixedCursor(first.apiKeyState, "k2", 1)

	// Disabling a different key keeps the remembered one.
	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, "k1", 401, "test"))
	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))

	second := svc.GetEnabledChannel(ch.ID)
	require.NotNil(t, second)
	require.Equal(t, "k2", NewFixedKeyProvider(second).Get(ctx))

	// Disabling the remembered key moves on to the next selectable one.
	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, "k2", 401, "test"))
	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))

	third := svc.GetEnabledChannel(ch.ID)
	require.NotNil(t, third)
	require.Equal(t, "k3", NewFixedKeyProvider(third).Get(ctx))
}

// The round_robin_success cursor must survive a reload too, and keep pointing at
// the key the success report had advanced it to.
func TestRoundRobinSuccessState_SurvivesRealCacheReload(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	svc := newReloadableChannelService(t, client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "rr-success-state-reload", []string{"k1", "k2"})
	setChannelStrategy(t, ctx, client, ch.ID, objects.APIKeyStrategyRoundRobinSuccess, 1)

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))

	first := svc.GetEnabledChannel(ch.ID)
	require.NotNil(t, first)
	require.NotNil(t, first.apiKeyState)

	require.Equal(t, "k1", NewRoundRobinSuccessKeyProvider(first).Get(ctx))

	// One success advances the cursor to k2.
	svc.onAPIKeySuccess(ch.ID, "k1")

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))

	second := svc.GetEnabledChannel(ch.ID)
	require.NotNil(t, second)
	require.NotSame(t, first, second)

	require.Equal(t, "k2", NewRoundRobinSuccessKeyProvider(second).Get(ctx))
}
