package gql

import (
	"context"
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
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestPromptResolverProjectID(t *testing.T) {
	projectID, err := (&promptResolver{}).ProjectID(context.Background(), &ent.Prompt{ProjectID: 42})

	require.NoError(t, err)
	require.Equal(t, ent.TypeProject, projectID.Type)
	require.Equal(t, 42, projectID.ID)
}

func TestChannelResolver_ProviderQuotaStatus_HidesDisabledCollectionProvider(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent:         client,
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
	})
	channelEntity, err := client.Channel.Create().
		SetName("MiniMax").
		SetType(channel.TypeMinimax).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ProviderQuotaStatus.Create().
		SetChannelID(channelEntity.ID).
		SetProviderType(providerquotastatus.ProviderTypeMinimax).
		SetStatus(providerquotastatus.StatusUnknown).
		SetQuotaData(map[string]any{"error": "plan required"}).
		SetReady(false).
		SetNextCheckAt(time.Now().Add(time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, nil, []biz.ProviderQuotaCollectionProvider{
		{Provider: "minimax", Enabled: false},
	}))
	resolver := &channelResolver{&Resolver{systemService: systemService}}

	status, err := resolver.ProviderQuotaStatus(ctx, channelEntity)
	require.NoError(t, err)
	require.Nil(t, status)

	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, nil, []biz.ProviderQuotaCollectionProvider{
		{Provider: "minimax", Enabled: true},
	}))
	status, err = resolver.ProviderQuotaStatus(ctx, channelEntity)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Equal(t, providerquotastatus.ProviderTypeMinimax, status.ProviderType)
}

func TestChannelResolver_ProviderQuotaStatus_ReturnsNilWhenStatusDoesNotExist(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent:         client,
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
	})
	channelEntity, err := client.Channel.Create().
		SetName("Without quota status").
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	resolver := &channelResolver{&Resolver{systemService: systemService}}
	status, err := resolver.ProviderQuotaStatus(ctx, channelEntity)

	require.NoError(t, err)
	require.Nil(t, status)
}
