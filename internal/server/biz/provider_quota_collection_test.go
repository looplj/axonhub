package biz

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
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

	codexChecker := &countingQuotaChecker{providerType: "codex"}
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
}
