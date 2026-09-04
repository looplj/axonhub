package biz

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/intercept"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestNewProviderQuotaService_WaitsForInitialCacheLoad(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	channelEntity, err := client.Channel.Create().
		SetName("Codex").
		SetType(channel.TypeCodex).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ProviderQuotaStatus.Create().
		SetChannelID(channelEntity.ID).
		SetProviderType(providerquotastatus.ProviderTypeCodex).
		SetStatus(providerquotastatus.StatusAvailable).
		SetReady(true).
		SetQuotaData(map[string]any{}).
		SetNextCheckAt(time.Now().Add(time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	queryStarted := make(chan struct{})
	releaseQuery := make(chan struct{})
	var blockOnce sync.Once
	client.Intercept(intercept.Func(func(_ context.Context, query intercept.Query) error {
		if query.Type() == ent.TypeProviderQuotaStatus {
			blockOnce.Do(func() {
				close(queryStarted)
				<-releaseQuery
			})
		}
		return nil
	}))

	systemService := NewSystemService(SystemServiceParams{
		Ent:         client,
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
	})
	serviceResult := make(chan *ProviderQuotaService, 1)
	go func() {
		serviceResult <- NewProviderQuotaService(ProviderQuotaServiceParams{
			Ent:           client,
			SystemService: systemService,
			HttpClient:    httpclient.NewHttpClient(),
		})
	}()

	select {
	case <-queryStarted:
	case <-time.After(time.Second):
		t.Fatal("initial quota cache query did not start")
	}

	var service *ProviderQuotaService
	returnedBeforeLoad := false
	select {
	case service = <-serviceResult:
		returnedBeforeLoad = true
	default:
	}

	close(releaseQuery)
	if service == nil {
		service = <-serviceResult
	}

	var value any
	var ok bool
	require.Eventually(t, func() bool {
		value, ok = service.quotaCache.Load(channelEntity.ID)
		return ok
	}, time.Second, 10*time.Millisecond)
	require.False(t, returnedBeforeLoad, "service must not be published before its initial cache load completes")
	require.True(t, ok)
	status, ok := value.(*QuotaChannelStatus)
	require.True(t, ok)
	require.Equal(t, "codex", status.ProviderType)
	require.Equal(t, providerquotastatus.StatusAvailable, status.Status)
}

func TestProviderQuotaService_SaveQuotaError_ResetsChangedProviderIdentity(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	channelEntity, err := client.Channel.Create().
		SetName("OpenAI compatible").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.synthetic.new/v1").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	oldResetAt := time.Now().Add(time.Hour)
	_, err = client.ProviderQuotaStatus.Create().
		SetChannelID(channelEntity.ID).
		SetProviderType(providerquotastatus.ProviderTypeWafer).
		SetStatus(providerquotastatus.StatusExhausted).
		SetReady(false).
		SetNextResetAt(oldResetAt).
		SetQuotaData(map[string]any{
			"error_count": 3,
			"_limits": []provider_quota.QuotaLimitStatus{{
				Type:       provider_quota.QuotaLimitTypeToken,
				Status:     string(providerquotastatus.StatusExhausted),
				UsageRatio: 1,
				Ready:      false,
			}},
		}).
		SetNextCheckAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	channelWithStatus, err := client.Channel.Query().
		Where(channel.IDEQ(channelEntity.ID)).
		WithProviderQuotaStatus().
		Only(ctx)
	require.NoError(t, err)

	service := &ProviderQuotaService{
		AbstractService: &AbstractService{db: client},
		checkInterval:   5 * time.Minute,
	}
	now := time.Now()
	service.saveQuotaError(ctx, channelWithStatus, "synthetic", "", errors.New("synthetic quota unavailable"), 1, now)

	persisted, err := client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(channelEntity.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, providerquotastatus.ProviderTypeSynthetic, persisted.ProviderType)
	require.Equal(t, providerquotastatus.StatusUnknown, persisted.Status)
	require.False(t, persisted.Ready)
	require.Nil(t, persisted.NextResetAt)
	require.EqualValues(t, 1, persisted.QuotaData["error_count"])
	require.Equal(t, "check_failed", persisted.QuotaData["error_code"])
	require.NotContains(t, persisted.QuotaData, "error")
	require.NotContains(t, persisted.QuotaData, "_limits")

	cachedValue, ok := service.quotaCache.Load(channelEntity.ID)
	require.True(t, ok)
	cached, ok := cachedValue.(*QuotaChannelStatus)
	require.True(t, ok)
	require.Equal(t, "synthetic", cached.ProviderType)
	require.Equal(t, providerquotastatus.StatusUnknown, cached.Status)
	require.False(t, cached.Ready)
	require.Empty(t, cached.Limits)

	reloadedService := &ProviderQuotaService{AbstractService: &AbstractService{db: client}}
	reloadedService.loadQuotaCache(ctx)
	reloadedValue, ok := reloadedService.quotaCache.Load(channelEntity.ID)
	require.True(t, ok)
	reloaded, ok := reloadedValue.(*QuotaChannelStatus)
	require.True(t, ok)
	require.Equal(t, "synthetic", reloaded.ProviderType)
	require.Equal(t, providerquotastatus.StatusUnknown, reloaded.Status)
	require.False(t, reloaded.Ready)
	require.Empty(t, reloaded.Limits)
}

func TestProviderQuotaService_SaveQuotaStatus_ReplacesStaleClinePassDataWhenUnavailable(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	channelEntity, err := client.Channel.Create().
		SetName("Cline").
		SetType(channel.TypeCline).
		SetStatus(channel.StatusEnabled).
		SetBaseURL("https://api.cline.bot/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"cline-pass/deepseek-v4-flash"}).
		SetDefaultTestModel("cline-pass/deepseek-v4-flash").
		Save(ctx)
	require.NoError(t, err)

	oldResetAt := time.Now().Add(time.Hour)
	_, err = client.ProviderQuotaStatus.Create().
		SetChannelID(channelEntity.ID).
		SetProviderType(providerquotastatus.ProviderTypeCline).
		SetStatus(providerquotastatus.StatusWarning).
		SetReady(true).
		SetNextResetAt(oldResetAt).
		SetQuotaData(map[string]any{
			"error":       "old quota failure",
			"error_count": 3,
			"windows":     map[string]any{"last5h": map[string]any{"usage_ratio": 0.9}},
			"_limits": []provider_quota.QuotaLimitStatus{{
				Type:        provider_quota.QuotaLimitTypeToken,
				Status:      string(providerquotastatus.StatusWarning),
				UsageRatio:  0.9,
				Ready:       true,
				NextResetAt: &oldResetAt,
			}},
		}).
		SetNextCheckAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	service := &ProviderQuotaService{
		AbstractService: &AbstractService{db: client},
		checkInterval:   5 * time.Minute,
	}
	service.saveQuotaStatus(ctx, channelEntity.ID, "cline", "", provider_quota.QuotaData{
		ProviderType: "cline",
		Status:       string(providerquotastatus.StatusExhausted),
		Ready:        false,
		RawData: map[string]any{
			"model_scope":  "cline_pass_only",
			"status_basis": "cline_pass_unavailable",
			"pool":         "cline_pass",
			"pass_state":   "unavailable",
			"balance":      map[string]any{"raw_balance": int64(497582)},
		},
	}, time.Now())

	persisted, err := client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(channelEntity.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, providerquotastatus.StatusExhausted, persisted.Status)
	require.False(t, persisted.Ready)
	require.Nil(t, persisted.NextResetAt)
	require.Equal(t, "unavailable", persisted.QuotaData["pass_state"])
	require.NotContains(t, persisted.QuotaData, "error")
	require.NotContains(t, persisted.QuotaData, "error_count")
	require.NotContains(t, persisted.QuotaData, "windows")
	require.NotContains(t, persisted.QuotaData, "_limits")

	cachedValue, ok := service.quotaCache.Load(channelEntity.ID)
	require.True(t, ok)
	cached, ok := cachedValue.(*QuotaChannelStatus)
	require.True(t, ok)
	require.Equal(t, providerquotastatus.StatusExhausted, cached.Status)
	require.False(t, cached.Ready)
	require.Empty(t, cached.Limits)
}

func TestProviderQuotaService_SaveQuotaStatus_UpdatesChangedProviderIdentity(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	channelEntity, err := client.Channel.Create().
		SetName("OpenAI compatible").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.synthetic.new/v1").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ProviderQuotaStatus.Create().
		SetChannelID(channelEntity.ID).
		SetProviderType(providerquotastatus.ProviderTypeWafer).
		SetStatus(providerquotastatus.StatusExhausted).
		SetReady(false).
		SetQuotaData(map[string]any{"provider": "wafer"}).
		SetNextCheckAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	service := &ProviderQuotaService{
		AbstractService: &AbstractService{db: client},
		checkInterval:   5 * time.Minute,
	}
	service.saveQuotaStatus(ctx, channelEntity.ID, "synthetic", "", provider_quota.QuotaData{
		ProviderType: "synthetic",
		Status:       string(providerquotastatus.StatusAvailable),
		Ready:        true,
		RawData:      map[string]any{"provider": "synthetic"},
	}, time.Now())

	persisted, err := client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(channelEntity.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, providerquotastatus.ProviderTypeSynthetic, persisted.ProviderType)
	require.Equal(t, providerquotastatus.StatusAvailable, persisted.Status)
	require.True(t, persisted.Ready)
	require.Equal(t, "synthetic", persisted.QuotaData["provider"])

	cachedValue, ok := service.quotaCache.Load(channelEntity.ID)
	require.True(t, ok)
	cached, ok := cachedValue.(*QuotaChannelStatus)
	require.True(t, ok)
	require.Equal(t, "synthetic", cached.ProviderType)
	require.Equal(t, providerquotastatus.StatusAvailable, cached.Status)
	require.True(t, cached.Ready)
}
