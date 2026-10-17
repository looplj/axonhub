package biz

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelpriceversion"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
)

func currencyTestPrice() objects.ModelPrice {
	return objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: loToDecimalPtr("0.01"),
				},
			},
		},
	}
}

func TestChannelService_SaveChannelModelPrices_Currency(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("CNY Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"m-cny", "m-usd"}).
		SetDefaultTestModel("m-cny").
		SetStatus(channel.StatusEnabled).
		SetCurrencyCode("CNY").
		Save(ctx)
	require.NoError(t, err)

	price := currencyTestPrice()

	results, err := svc.SaveChannelModelPrices(ctx, ch.ID, []SaveChannelModelPriceInput{
		{ModelID: "m-cny", Price: price},
		{ModelID: "m-usd", Price: price, CurrencyCode: "USD"},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	pricesByModel := lo.KeyBy(results, func(p *ent.ChannelModelPrice) string { return p.ModelID })
	require.Equal(t, "CNY", pricesByModel["m-cny"].CurrencyCode)
	require.Equal(t, "USD", pricesByModel["m-usd"].CurrencyCode)

	for _, modelID := range []string{"m-cny", "m-usd"} {
		activeVersion, err := client.ChannelModelPriceVersion.Query().
			Where(
				channelmodelpriceversion.ChannelModelPriceIDEQ(pricesByModel[modelID].ID),
				channelmodelpriceversion.StatusEQ(channelmodelpriceversion.StatusActive),
			).
			Only(ctx)
		require.NoError(t, err)
		require.Equal(t, pricesByModel[modelID].CurrencyCode, activeVersion.CurrencyCode)
	}

	// Changing only the currency must archive the old version and record the new one.
	results, err = svc.SaveChannelModelPrices(ctx, ch.ID, []SaveChannelModelPriceInput{
		{ModelID: "m-cny", Price: price, CurrencyCode: "CNY"},
		{ModelID: "m-usd", Price: price, CurrencyCode: "EUR"},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	pricesByModel = lo.KeyBy(results, func(p *ent.ChannelModelPrice) string { return p.ModelID })
	require.Equal(t, "EUR", pricesByModel["m-usd"].CurrencyCode)

	activeVersion, err := client.ChannelModelPriceVersion.Query().
		Where(
			channelmodelpriceversion.ChannelModelPriceIDEQ(pricesByModel["m-usd"].ID),
			channelmodelpriceversion.StatusEQ(channelmodelpriceversion.StatusActive),
		).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "EUR", activeVersion.CurrencyCode)

	_, err = svc.SaveChannelModelPrices(ctx, ch.ID, []SaveChannelModelPriceInput{
		{ModelID: "m-cny", Price: price, CurrencyCode: "NOT_A_CURRENCY"},
		{ModelID: "m-usd", Price: price, CurrencyCode: "EUR"},
	})
	require.ErrorContains(t, err, "invalid model price currency")
}

func TestUsageLogService_CreateUsageLog_UsesPriceCurrency(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	p, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	ch, err := client.Channel.Create().
		SetName("CNY Channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com/v1").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetCurrencyCode("CNY").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetCurrencyCode("CNY").
		SetReferenceID("cny-ref").
		SetPrice(currencyTestPrice()).
		Save(ctx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	channelService := NewChannelServiceForTest(client)
	enabledCh, err := channelService.buildChannelWithTransformer(ch)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, enabledCh)
	channelService.SetEnabledChannelsForTest([]*Channel{enabledCh})

	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	usageLogService := NewUsageLogService(client, systemService, channelService)

	created, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     req.ID,
		ProjectID:     p.ID,
		ChannelID:     ch.ID,
		ActualModelID: "gpt-4",
		Usage: &llm.Usage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
		},
		Source: usagelog.SourceAPI,
		Format: "openai/chat_completions",
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, "CNY", created.CostCurrencyCode)
}

func TestQuotaUsageAgg_FiltersCostByCurrency(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	p, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	const apiKeyID = 7

	now := time.Now()
	createLog := func(cost float64, currencyCode string) {
		req, err := client.Request.Create().
			SetProjectID(p.ID).
			SetAPIKeyID(apiKeyID).
			SetModelID("m").
			SetFormat("openai/chat_completions").
			SetStatus(request.StatusCompleted).
			SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
			Save(ctx)
		require.NoError(t, err)

		_, err = client.UsageLog.Create().
			SetRequestID(req.ID).
			SetAPIKeyID(apiKeyID).
			SetProjectID(p.ID).
			SetChannelID(1).
			SetModelID("m").
			SetTotalTokens(10).
			SetTotalCost(cost).
			SetCostCurrencyCode(currencyCode).
			SetCreatedAt(now).
			Save(ctx)
		require.NoError(t, err)
	}

	createLog(1, "USD")
	createLog(2, "CNY")

	svc := NewQuotaService(client, NewSystemService(SystemServiceParams{Ent: client}))
	window := QuotaWindow{
		Start:        lo.ToPtr(now.Add(-time.Minute)),
		End:          lo.ToPtr(now.Add(time.Minute)),
		EndInclusive: true,
	}

	usdUsage, err := svc.usageAgg(ctx, apiKeyID, window, true, true, "USD")
	require.NoError(t, err)
	require.Equal(t, int64(20), usdUsage.TotalTokens)
	require.True(t, usdUsage.TotalCost.Equal(decimal.NewFromFloat(1)))

	cnyUsage, err := svc.usageAgg(ctx, apiKeyID, window, true, true, "CNY")
	require.NoError(t, err)
	require.Equal(t, int64(20), cnyUsage.TotalTokens)
	require.True(t, cnyUsage.TotalCost.Equal(decimal.NewFromFloat(2)))
}

func TestQuotaCurrencyCode_DefaultsLegacyQuotasToUSD(t *testing.T) {
	require.Equal(t, objects.DefaultCurrencyCode, quotaCurrencyCode(nil))
	require.Equal(t, objects.DefaultCurrencyCode, quotaCurrencyCode(&objects.APIKeyQuota{}))
	require.Equal(t, "CNY", quotaCurrencyCode(&objects.APIKeyQuota{CurrencyCode: "cny"}))
}

func TestValidateProfileQuota_StampsCostCurrency(t *testing.T) {
	profiles := []objects.APIKeyProfile{
		{
			Name: "default",
			Quota: &objects.APIKeyQuota{
				Cost:   lo.ToPtr(decimal.NewFromInt(1)),
				Period: objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeAllTime},
			},
		},
		{
			Name: "explicit",
			Quota: &objects.APIKeyQuota{
				Cost:         lo.ToPtr(decimal.NewFromInt(1)),
				CurrencyCode: "cny",
				Period:       objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeAllTime},
			},
		},
	}

	require.NoError(t, validateProfileQuota(profiles))
	require.Equal(t, objects.DefaultCurrencyCode, profiles[0].Quota.CurrencyCode)
	require.Equal(t, "CNY", profiles[1].Quota.CurrencyCode)
}
