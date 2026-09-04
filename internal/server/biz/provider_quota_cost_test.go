package biz

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/intercept"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

func newPeriodQuotaTestChannel(t *testing.T, ctx context.Context, client *ent.Client) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetName("Claude Code").
		SetType(channel.TypeClaudecode).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	return ch
}

// createPeriodQuotaUsageLog records a usage log for the channel at createdAt.
// A nil cost mirrors a channel without configured model prices.
func createPeriodQuotaUsageLog(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	channelID int,
	createdAt time.Time,
	cost *float64,
) {
	t.Helper()

	_, err := client.UsageLog.Create().
		SetRequestID(1).
		SetChannelID(channelID).
		SetModelID("test-model").
		SetTotalTokens(100).
		SetNillableTotalCost(cost).
		SetCreatedAt(createdAt).
		SetUpdatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)
}

func TestProviderQuotaService_FillPeriodQuotas(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	ch := newPeriodQuotaTestChannel(t, ctx, client)

	now := time.Now()
	fiveHourStart := now.Add(-4 * time.Hour)
	sevenDayStart := now.Add(-6 * 24 * time.Hour)

	// Older than the 7d window: must not be counted by either limit.
	createPeriodQuotaUsageLog(t, ctx, client, ch.ID, now.Add(-8*24*time.Hour), lo.ToPtr(100.0))
	// Inside the 7d window only.
	createPeriodQuotaUsageLog(t, ctx, client, ch.ID, now.Add(-3*24*time.Hour), lo.ToPtr(30.0))
	// Inside both windows.
	createPeriodQuotaUsageLog(t, ctx, client, ch.ID, now.Add(-time.Hour), lo.ToPtr(12.0))
	// In the future relative to the check: excluded.
	createPeriodQuotaUsageLog(t, ctx, client, ch.ID, now.Add(time.Minute), lo.ToPtr(99.0))

	svc := &ProviderQuotaService{AbstractService: &AbstractService{db: client}}
	quotaData := provider_quota.QuotaData{
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, Window: "5h", UsageRatio: 0.5, PeriodStart: &fiveHourStart},
			{Type: provider_quota.QuotaLimitTypeToken, Window: "7d", UsageRatio: 0.25, PeriodStart: &sevenDayStart},
			// No period start: the checker could not determine the window.
			{Type: provider_quota.QuotaLimitTypeToken, Window: "monthly", UsageRatio: 0.5},
		},
	}

	svc.fillPeriodQuotas(ctx, ch.ID, &quotaData, now)

	require.NotNil(t, quotaData.Limits[0].PeriodCost)
	require.InDelta(t, 12.0, *quotaData.Limits[0].PeriodCost, 1e-9)
	require.NotNil(t, quotaData.Limits[0].PeriodQuota)
	require.InDelta(t, 24.0, *quotaData.Limits[0].PeriodQuota, 1e-9)

	require.NotNil(t, quotaData.Limits[1].PeriodCost)
	require.InDelta(t, 42.0, *quotaData.Limits[1].PeriodCost, 1e-9)
	require.NotNil(t, quotaData.Limits[1].PeriodQuota)
	require.InDelta(t, 168.0, *quotaData.Limits[1].PeriodQuota, 1e-9)

	require.Nil(t, quotaData.Limits[2].PeriodCost)
	require.Nil(t, quotaData.Limits[2].PeriodQuota)
}

func TestProviderQuotaService_FillPeriodQuotas_SkipsZenmux(t *testing.T) {
	now := time.Now()
	periodCost := 12.0
	periodQuota := 24.0
	quotaData := provider_quota.QuotaData{
		ProviderType: "zenmux",
		Limits: []provider_quota.QuotaLimitStatus{{
			PeriodStart: &now,
			PeriodCost:  &periodCost,
			PeriodQuota: &periodQuota,
		}},
	}

	svc := &ProviderQuotaService{}
	svc.fillPeriodQuotas(context.Background(), 0, &quotaData, now.Add(time.Hour))

	require.Nil(t, quotaData.Limits[0].PeriodCost)
	require.Nil(t, quotaData.Limits[0].PeriodQuota)
}

func TestProviderQuotaService_FillPeriodQuotas_OnlyCountsOwnChannel(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	ch := newPeriodQuotaTestChannel(t, ctx, client)

	other, err := client.Channel.Create().
		SetName("Other").
		SetType(channel.TypeCodex).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)

	now := time.Now()
	start := now.Add(-time.Hour)

	createPeriodQuotaUsageLog(t, ctx, client, ch.ID, now.Add(-time.Minute), lo.ToPtr(10.0))
	createPeriodQuotaUsageLog(t, ctx, client, other.ID, now.Add(-time.Minute), lo.ToPtr(500.0))

	svc := &ProviderQuotaService{AbstractService: &AbstractService{db: client}}
	quotaData := provider_quota.QuotaData{
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, UsageRatio: 0.5, PeriodStart: &start},
		},
	}

	svc.fillPeriodQuotas(ctx, ch.ID, &quotaData, now)

	require.NotNil(t, quotaData.Limits[0].PeriodCost)
	require.InDelta(t, 10.0, *quotaData.Limits[0].PeriodCost, 1e-9)
}

func TestProviderQuotaService_FillPeriodQuotas_QueriesEachPeriodOnce(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	ch := newPeriodQuotaTestChannel(t, ctx, client)

	now := time.Now()
	start := now.Add(-time.Hour)
	createPeriodQuotaUsageLog(t, ctx, client, ch.ID, now.Add(-time.Minute), lo.ToPtr(10.0))

	var usageLogQueries atomic.Int64

	client.Intercept(intercept.Func(func(_ context.Context, query intercept.Query) error {
		if query.Type() == ent.TypeUsageLog {
			usageLogQueries.Add(1)
		}

		return nil
	}))

	svc := &ProviderQuotaService{AbstractService: &AbstractService{db: client}}
	quotaData := provider_quota.QuotaData{
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, Window: "5h", UsageRatio: 0.5, PeriodStart: &start},
			{Type: provider_quota.QuotaLimitTypeImage, Window: "5h", UsageRatio: 0.25, PeriodStart: &start},
		},
	}

	svc.fillPeriodQuotas(ctx, ch.ID, &quotaData, now)

	require.EqualValues(t, 1, usageLogQueries.Load(), "limits sharing a period must share one aggregation query")
	require.NotNil(t, quotaData.Limits[0].PeriodQuota)
	require.NotNil(t, quotaData.Limits[1].PeriodQuota)
}

func TestProviderQuotaService_FillPeriodQuotas_NoEstimateWithoutCost(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	ch := newPeriodQuotaTestChannel(t, ctx, client)

	now := time.Now()
	start := now.Add(-time.Hour)

	// Channel without model prices: usage is logged, cost is not.
	createPeriodQuotaUsageLog(t, ctx, client, ch.ID, now.Add(-time.Minute), nil)

	svc := &ProviderQuotaService{AbstractService: &AbstractService{db: client}}
	quotaData := provider_quota.QuotaData{
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, UsageRatio: 0.5, PeriodStart: &start},
		},
	}

	svc.fillPeriodQuotas(ctx, ch.ID, &quotaData, now)

	require.NotNil(t, quotaData.Limits[0].PeriodCost)
	require.Zero(t, *quotaData.Limits[0].PeriodCost)
	require.Nil(t, quotaData.Limits[0].PeriodQuota)
}

func TestProviderQuotaService_FillPeriodQuotas_ClearsStaleEstimate(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	ch := newPeriodQuotaTestChannel(t, ctx, client)

	now := time.Now()
	start := now.Add(-time.Hour)
	createPeriodQuotaUsageLog(t, ctx, client, ch.ID, now.Add(-time.Minute), lo.ToPtr(10.0))

	svc := &ProviderQuotaService{AbstractService: &AbstractService{db: client}}
	// A freshly reset window reports 0% usage; the values carried over from the
	// previous check must not survive.
	quotaData := provider_quota.QuotaData{
		Limits: []provider_quota.QuotaLimitStatus{
			{
				Type:        provider_quota.QuotaLimitTypeToken,
				UsageRatio:  0,
				PeriodStart: &start,
				PeriodCost:  lo.ToPtr(45.0),
				PeriodQuota: lo.ToPtr(90.0),
			},
		},
	}

	svc.fillPeriodQuotas(ctx, ch.ID, &quotaData, now)

	require.NotNil(t, quotaData.Limits[0].PeriodCost)
	require.InDelta(t, 10.0, *quotaData.Limits[0].PeriodCost, 1e-9)
	require.Nil(t, quotaData.Limits[0].PeriodQuota)
}

func TestProviderQuotaService_PeriodQuotaSurvivesPersistence(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	ch := newPeriodQuotaTestChannel(t, ctx, client)

	now := time.Now()
	periodStart := now.Add(-4 * time.Hour).Truncate(time.Second)
	nextReset := now.Add(time.Hour).Truncate(time.Second)

	svc := &ProviderQuotaService{
		AbstractService: &AbstractService{db: client},
		checkInterval:   5 * time.Minute,
	}
	svc.saveQuotaStatus(ctx, ch.ID, "claudecode", "", provider_quota.QuotaData{
		ProviderType: "claudecode",
		Status:       string(providerquotastatus.StatusAvailable),
		Ready:        true,
		RawData:      map[string]any{"plan_type": "max"},
		Limits: []provider_quota.QuotaLimitStatus{
			{
				Type:        provider_quota.QuotaLimitTypeToken,
				Window:      "5h",
				Status:      string(providerquotastatus.StatusAvailable),
				UsageRatio:  0.5,
				Ready:       true,
				NextResetAt: &nextReset,
				PeriodStart: &periodStart,
				PeriodCost:  lo.ToPtr(45.0),
				PeriodQuota: lo.ToPtr(90.0),
			},
		},
	}, now)

	persisted, err := client.ProviderQuotaStatus.Query().
		Where(providerquotastatus.ChannelIDEQ(ch.ID)).
		Only(ctx)
	require.NoError(t, err)

	limits := extractLimitsFromQuotaData(persisted.QuotaData)
	require.Len(t, limits, 1)
	require.Equal(t, "5h", limits[0].Window)
	require.NotNil(t, limits[0].PeriodStart)
	require.True(t, periodStart.Equal(*limits[0].PeriodStart))
	require.NotNil(t, limits[0].PeriodCost)
	require.InDelta(t, 45.0, *limits[0].PeriodCost, 1e-9)
	require.NotNil(t, limits[0].PeriodQuota)
	require.InDelta(t, 90.0, *limits[0].PeriodQuota, 1e-9)

	// A restart reloads the estimate from the database into the cache.
	reloaded := &ProviderQuotaService{AbstractService: &AbstractService{db: client}}
	reloaded.loadQuotaCache(ctx)

	cachedValue, ok := reloaded.quotaCache.Load(ch.ID)
	require.True(t, ok)
	cached, ok := cachedValue.(*QuotaChannelStatus)
	require.True(t, ok)
	require.Len(t, cached.Limits, 1)
	require.NotNil(t, cached.Limits[0].PeriodQuota)
	require.InDelta(t, 90.0, *cached.Limits[0].PeriodQuota, 1e-9)
}
