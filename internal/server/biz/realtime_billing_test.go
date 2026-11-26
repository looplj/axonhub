// 职责: 覆盖 Realtime 价格快照、分项 token 账单与渠道失效后的记账。
// 关联文件:
//
//	internal/server/biz/realtime_billing.go
//	internal/server/biz/cost_calc.go

package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
)

type realtimeBillingFixture struct {
	client         *ent.Client
	ctx            context.Context
	svc            *UsageLogService
	channelService *ChannelService
	projectID      int
	requestID      int
	channelID      int
	model          string
}

func newRealtimeBillingFixture(t *testing.T, price objects.ModelPrice) *realtimeBillingFixture {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	p, err := client.Project.Create().
		SetName("rt-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	ch, err := client.Channel.Create().
		SetName("rt-channel").
		SetType("openai").
		SetBaseURL("https://api.openai.com/v1").
		SetSupportedModels([]string{"rt-model"}).
		SetDefaultTestModel("rt-model").
		SetStatus("enabled").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("rt-model").
		SetReferenceID("rt-ref").
		SetPrice(price).
		Save(ctx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetChannelID(ch.ID).
		SetModelID("rt-model").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	channelService := NewChannelServiceForTest(client)

	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, built)
	channelService.SetEnabledChannelsForTest([]*Channel{built})

	svc := NewUsageLogService(client, systemService, channelService)

	return &realtimeBillingFixture{
		client:         client,
		ctx:            ctx,
		svc:            svc,
		channelService: channelService,
		projectID:      p.ID,
		requestID:      req.ID,
		channelID:      ch.ID,
		model:          "rt-model",
	}
}

func (f *realtimeBillingFixture) createParams(usage *llm.Usage) CreateUsageLogParams {
	return CreateUsageLogParams{
		RequestID:     f.requestID,
		ProjectID:     f.projectID,
		ChannelID:     f.channelID,
		ActualModelID: f.model,
		Usage:         usage,
		Source:        usagelog.SourceAPI,
		Format:        "openai/realtime",
	}
}

func realtimeUsageFixture() *llm.Usage {
	return &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
		PromptTokensDetails: &llm.PromptTokensDetails{
			AudioTokens: 300,
		},
		CompletionTokensDetails: &llm.CompletionTokensDetails{
			AudioTokens: 100,
		},
	}
}

func costItemByCode(t *testing.T, items []objects.CostItem, code objects.PriceItemCode) objects.CostItem {
	t.Helper()

	for _, item := range items {
		if item.ItemCode == code {
			return item
		}
	}

	require.FailNowf(t, "missing cost item", "code %s", code)

	return objects.CostItem{}
}

func priceItemByCode(t *testing.T, items []objects.ModelPriceItem, code objects.PriceItemCode) objects.ModelPriceItem {
	t.Helper()

	idx := priceItemIndex(t, items, code)

	return items[idx]
}

func priceItemIndex(t *testing.T, items []objects.ModelPriceItem, code objects.PriceItemCode) int {
	t.Helper()

	for i := range items {
		if items[i].ItemCode == code {
			return i
		}
	}

	require.FailNowf(t, "missing price item", "code %s", code)

	return -1
}

func TestUsageLogService_RealtimePrice(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1.10", "2.20", "3.30", "4.40"))

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, "rt-ref", snapshot.ReferenceID)
	require.NoError(t, objects.ValidateRealtimePrice(snapshot.Price))

	for _, code := range objects.RealtimePriceItemCodes() {
		item := priceItemByCode(t, snapshot.Price.Items, code)
		require.True(t, item.Pricing.UsagePerUnit.IsPositive(), "code %s", code)
	}
}

func TestUsageLogService_RealtimePrice_AcceptsInputAudioOnlyPrice(t *testing.T) {
	partial := objects.ModelPrice{Items: []objects.ModelPriceItem{
		{
			ItemCode: objects.PriceItemCodeRealtimeInputAudio,
			Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("2")},
		},
	}}

	f := newRealtimeBillingFixture(t, partial)

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Len(t, snapshot.Price.Items, 1)

	created, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(realtimeUsageFixture()), snapshot)
	require.NoError(t, err)
	require.EqualValues(t, 1500, created.TotalTokens)
	require.Len(t, created.CostItems, 1, "unpriced modalities are omitted from the bill")
	require.Equal(t, objects.PriceItemCodeRealtimeInputAudio, created.CostItems[0].ItemCode)
	require.Equal(t, int64(300), created.CostItems[0].Quantity)
	require.InDelta(t, 0.0006, *created.TotalCost, 1e-12)
}

func TestUsageLogService_CreateRealtimeUsage_OpenAIKeepsImageCacheRatesComplete(t *testing.T) {
	f := newRealtimeBillingFixture(t, objects.ModelPrice{})
	price := objects.ModelPrice{Items: []objects.ModelPriceItem{
		{ItemCode: objects.PriceItemCodeRealtimeInputAudio, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("2")}},
		{ItemCode: objects.PriceItemCodeRealtimeInputImage, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("3")}},
		{ItemCode: objects.PriceItemCodeRealtimeCachedInputText, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("0.1")}},
		{ItemCode: objects.PriceItemCodeRealtimeCachedInputAudio, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("0.2")}},
		{ItemCode: objects.PriceItemCodeRealtimeCachedInputImage, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("0.3")}},
	}}
	snapshot := &RealtimePriceSnapshot{Price: price, RequireOpenAIImageCacheRates: true}

	created, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(realtimeUsageFixture()), snapshot)
	require.NoError(t, err)
	require.NotNil(t, created.TotalCost)
	require.Contains(t, lo.Map(created.CostItems, func(item objects.CostItem, _ int) objects.PriceItemCode { return item.ItemCode }), objects.PriceItemCodeRealtimeInputAudio)
	require.NotContains(t, lo.Map(created.CostItems, func(item objects.CostItem, _ int) objects.PriceItemCode { return item.ItemCode }), objects.PriceItemCodeRealtimeOutputText)

	deleteImageCacheRate := &RealtimePriceSnapshot{Price: objects.ModelPrice{Items: price.Items[:len(price.Items)-1]}, RequireOpenAIImageCacheRates: true}
	_, err = f.svc.CreateRealtimeUsage(f.ctx, f.createParams(realtimeUsageFixture()), deleteImageCacheRate)
	require.Error(t, err)
	require.Contains(t, err.Error(), "realtime_cached_input_image_tokens")
}

func TestUsageLogService_RealtimePrice_IgnoresLegacyLibraryPrices(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))

	// Remove the channel price from the cache so resolution falls back to the
	// model library, which can only express legacy prompt/completion costs.
	enabled := f.channelService.GetEnabledChannel(f.channelID)
	require.NotNil(t, enabled)
	enabled.cachedModelPrices = map[string]*ent.ChannelModelPrice{}

	createModelLibraryPriceEntry(t, f.ctx, f.client, f.model, model.StatusEnabled, objects.ModelCardCost{Input: 1, Output: 2})

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)
	require.Nil(t, snapshot)
}

func TestUsageLogService_RealtimePrice_NoPrice(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))

	f.channelService.SetEnabledChannelsForTest([]*Channel{})

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)
	require.Nil(t, snapshot)
}

func TestUsageLogService_RealtimePrice_SnapshotIsStable(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1.10", "2.20", "3.30", "4.40"))

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)

	// Mutating the live channel cache must not change the frozen snapshot.
	enabled := f.channelService.GetEnabledChannel(f.channelID)
	require.NotNil(t, enabled)

	liveItems := enabled.cachedModelPrices[f.model].Price.Items
	liveIdx := priceItemIndex(t, liveItems, objects.PriceItemCodeRealtimeInputText)
	liveItems[liveIdx].Pricing.UsagePerUnit = mustDecimalPtr("99")

	snapshotInputText := priceItemByCode(t, snapshot.Price.Items, objects.PriceItemCodeRealtimeInputText)
	require.True(t, snapshotInputText.Pricing.UsagePerUnit.Equal(decimal.RequireFromString("1.10")))
	require.True(t, liveItems[liveIdx].Pricing.UsagePerUnit.Equal(decimal.RequireFromString("99")))

	// The original snapshot still bills after the current price becomes unavailable.
	f.channelService.SetEnabledChannelsForTest([]*Channel{})

	current, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)
	require.Nil(t, current)

	created, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(realtimeUsageFixture()), snapshot)
	require.NoError(t, err)
	require.NotNil(t, created.TotalCost)
	require.InDelta(t, 0.00319, *created.TotalCost, 1e-12)
}

func TestUsageLogService_CreateRealtimeUsage_ExactFourCostItems(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1.10", "2.20", "3.30", "4.40"))

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)

	created, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(realtimeUsageFixture()), snapshot)
	require.NoError(t, err)
	require.NotNil(t, created)

	require.Equal(t, "rt-ref", created.CostPriceReferenceID)
	require.Equal(t, f.requestID, created.RequestID)
	require.Equal(t, f.projectID, created.ProjectID)
	require.Equal(t, f.channelID, created.ChannelID)
	require.Equal(t, f.model, created.ModelID)
	require.Equal(t, usagelog.SourceAPI, created.Source)
	require.Equal(t, "openai/realtime", created.Format)
	require.Equal(t, int64(1000), created.PromptTokens)
	require.Equal(t, int64(500), created.CompletionTokens)
	require.Equal(t, int64(1500), created.TotalTokens)
	require.Equal(t, int64(300), created.PromptAudioTokens)
	require.Equal(t, int64(100), created.CompletionAudioTokens)

	require.Len(t, created.CostItems, 4)

	expected := map[objects.PriceItemCode]struct {
		quantity int64
		subtotal string
	}{
		objects.PriceItemCodeRealtimeInputText:   {700, "0.00077"},
		objects.PriceItemCodeRealtimeInputAudio:  {300, "0.00066"},
		objects.PriceItemCodeRealtimeOutputText:  {400, "0.00132"},
		objects.PriceItemCodeRealtimeOutputAudio: {100, "0.00044"},
	}

	sum := decimal.Zero

	for code, want := range expected {
		item := costItemByCode(t, created.CostItems, code)
		require.Equal(t, want.quantity, item.Quantity, "quantity of %s", code)
		require.True(t, item.Subtotal.Equal(decimal.RequireFromString(want.subtotal)),
			"subtotal of %s: want %s, got %s", code, want.subtotal, item.Subtotal)
		sum = sum.Add(item.Subtotal)
	}

	require.True(t, sum.Equal(decimal.RequireFromString("0.00319")), "sum: %s", sum)
	require.NotNil(t, created.TotalCost)
	require.InDelta(t, 0.00319, *created.TotalCost, 1e-12)
}

func TestUsageLogService_CreateRealtimeUsage_UsesSnapshotAfterChannelDisabled(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1.10", "2.20", "3.30", "4.40"))

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)

	// Simulate the channel being disabled mid-session. Re-resolving the price
	// would lose the price; the frozen snapshot must still bill.
	f.channelService.SetEnabledChannelsForTest([]*Channel{})

	created, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(realtimeUsageFixture()), snapshot)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotNil(t, created.TotalCost)
	require.InDelta(t, 0.00319, *created.TotalCost, 1e-12)
	require.Equal(t, "rt-ref", created.CostPriceReferenceID)
}

func TestUsageLogService_CreateRealtimeUsage_RejectsInvalidUsage(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)

	tests := []struct {
		name    string
		mutate  func(*llm.Usage)
		wantErr string
	}{
		{
			name: "negative prompt tokens",
			mutate: func(usage *llm.Usage) {
				usage.PromptTokens = -1
			},
			wantErr: "negative",
		},
		{
			name: "negative prompt audio",
			mutate: func(usage *llm.Usage) {
				usage.PromptTokensDetails.AudioTokens = -5
			},
			wantErr: "negative",
		},
		{
			name: "prompt audio above total",
			mutate: func(usage *llm.Usage) {
				usage.PromptTokensDetails.AudioTokens = 1001
			},
			wantErr: "audio",
		},
		{
			name: "completion audio above total",
			mutate: func(usage *llm.Usage) {
				usage.CompletionTokensDetails.AudioTokens = 501
			},
			wantErr: "audio",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := realtimeUsageFixture()
			tt.mutate(usage)

			_, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(usage), snapshot)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("nil usage", func(t *testing.T) {
		_, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(nil), snapshot)
		require.Error(t, err)
		require.Contains(t, err.Error(), "realtime usage is required")
	})

	t.Run("unpriced usage retains tokens without a charge", func(t *testing.T) {
		usage := realtimeUsageFixture()
		row, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(usage), nil)
		require.NoError(t, err)
		require.Equal(t, usage.TotalTokens, row.TotalTokens)
		require.Nil(t, row.TotalCost)
		require.Empty(t, row.CostItems)
		require.Empty(t, row.CostPriceReferenceID)
	})

	t.Run("partial snapshot bills only configured modalities", func(t *testing.T) {
		partial := &RealtimePriceSnapshot{
			Price: objects.ModelPrice{Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeRealtimeInputText,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("1")},
				},
			}},
			ReferenceID: "partial",
		}

		created, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(realtimeUsageFixture()), partial)
		require.NoError(t, err)
		require.Len(t, created.CostItems, 1)
		require.Equal(t, objects.PriceItemCodeRealtimeInputText, created.CostItems[0].ItemCode)
		require.Equal(t, int64(700), created.CostItems[0].Quantity)
		require.InDelta(t, 0.0007, *created.TotalCost, 1e-12)
	})
}

func TestUsageLogService_CreateRealtimeUsage_MissingDetailsBillsText(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1.10", "2.20", "3.30", "4.40"))

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)

	usage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}

	created, err := f.svc.CreateRealtimeUsage(f.ctx, f.createParams(usage), snapshot)
	require.NoError(t, err)
	require.NotNil(t, created.TotalCost)
	require.InDelta(t, 0.00275, *created.TotalCost, 1e-12)
	require.Equal(t, int64(1000), costItemByCode(t, created.CostItems, objects.PriceItemCodeRealtimeInputText).Quantity)
	require.Equal(t, int64(0), costItemByCode(t, created.CostItems, objects.PriceItemCodeRealtimeInputAudio).Quantity)
	require.Equal(t, int64(500), costItemByCode(t, created.CostItems, objects.PriceItemCodeRealtimeOutputText).Quantity)
	require.Equal(t, int64(0), costItemByCode(t, created.CostItems, objects.PriceItemCodeRealtimeOutputAudio).Quantity)
}

func TestUsageLogService_CreateRealtimeUsage_APIKeyBinding(t *testing.T) {
	f := newRealtimeBillingFixture(t, realtimePrice("1", "2", "3", "4"))

	snapshot, err := f.svc.RealtimePrice(f.ctx, f.channelID, f.model)
	require.NoError(t, err)

	ctxKey, err := f.client.APIKey.Create().
		SetName("ctx-key").
		SetKey("sk-ctx").
		SetProjectID(f.projectID).
		SetType("user").
		SetScopes([]string{"read_channels", "write_requests"}).
		Save(f.ctx)
	require.NoError(t, err)

	t.Run("context API key", func(t *testing.T) {
		ctx := contexts.WithAPIKey(f.ctx, ctxKey)

		created, err := f.svc.CreateRealtimeUsage(ctx, f.createParams(realtimeUsageFixture()), snapshot)
		require.NoError(t, err)
		require.Equal(t, ctxKey.ID, created.APIKeyID)
	})

	t.Run("explicit API key wins", func(t *testing.T) {
		explicitKey, err := f.client.APIKey.Create().
			SetName("explicit-key").
			SetKey("sk-explicit").
			SetProjectID(f.projectID).
			SetType("user").
			SetScopes([]string{"read_channels", "write_requests"}).
			Save(f.ctx)
		require.NoError(t, err)

		ctx := contexts.WithAPIKey(f.ctx, ctxKey)
		params := f.createParams(realtimeUsageFixture())
		params.APIKeyID = lo.ToPtr(explicitKey.ID)

		created, err := f.svc.CreateRealtimeUsage(ctx, params, snapshot)
		require.NoError(t, err)
		require.Equal(t, explicitKey.ID, created.APIKeyID)
	})
}
