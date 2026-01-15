package biz

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func createPriceItem(mode objects.PricingMode, unit float64) objects.ModelPriceItem {
	switch mode {
	case objects.PricingModeFlatFee:
		d := decimal.NewFromFloat(unit)

		return objects.ModelPriceItem{
			ItemCode: objects.PriceItemCodeUsage,
			Pricing: objects.Pricing{
				Mode:    mode,
				FlatFee: &d,
			},
		}
	case objects.PricingModeUsagePerUnit:
		d := decimal.NewFromFloat(unit)

		return objects.ModelPriceItem{
			ItemCode: objects.PriceItemCodeUsage,
			Pricing: objects.Pricing{
				Mode:         mode,
				UsagePerUnit: &d,
			},
		}
	default:
		return objects.ModelPriceItem{}
	}
}

func TestUsageCost_PerUnitPromptAndCompletion(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c1").
		SetSupportedModels([]string{"m1"}).
		SetDefaultTestModel("m1").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	// price: prompt_tokens $0.01 per token, completion_tokens $0.02 per token
	promptUnit := decimal.NewFromFloat(0.01)
	completionUnit := decimal.NewFromFloat(0.02)
	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("m1").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &promptUnit},
				},
				{
					ItemCode: objects.PriceItemCodeCompletion,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &completionUnit},
				},
			},
		}).
		SetRefreanceID("ref-1").
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, built)
	channelService.enabledChannels = []*Channel{built}

	usageLogService := NewUsageLogService(client, systemService, channelService)

	usage := &llm.Usage{
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
	}

	ul, err := usageLogService.CreateUsageLog(ctx, 1, 1, &ch.ID, "m1", usage, "api", "openai/chat_completions")
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: (100/1e6)*0.01 + (200/1e6)*0.02 = 0.000005
	require.InDelta(t, 0.000005, ul.TotalCost, 1e-12)
	require.Len(t, ul.CostItems, 2)
}

func TestUsageCost_TieredPrompt(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c2").
		SetSupportedModels([]string{"m2"}).
		SetDefaultTestModel("m2").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	upTo1000 := int64(1000)
	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("m2").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing: objects.Pricing{
						Mode: objects.PricingModeTiered,
						UsageTiered: &objects.TieredPricing{
							Tiers: []objects.PriceTier{
								{UpTo: &upTo1000, PricePerUnit: decimal.NewFromFloat(0.01)},
								{UpTo: nil, PricePerUnit: decimal.NewFromFloat(0.02)},
							},
						},
					},
				},
			},
		}).
		SetRefreanceID("ref-2").
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, built)
	channelService.enabledChannels = []*Channel{built}

	usageLogService := NewUsageLogService(client, systemService, channelService)

	usage := &llm.Usage{
		PromptTokens:     1500,
		CompletionTokens: 0,
		TotalTokens:      1500,
	}

	ul, err := usageLogService.CreateUsageLog(ctx, 1, 1, &ch.ID, "m2", usage, "api", "openai/chat_completions")
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: (1000/1e6)*0.01 + (500/1e6)*0.02 = 0.00002
	require.InDelta(t, 0.00002, ul.TotalCost, 1e-12)
	require.Len(t, ul.CostItems, 1)
	require.Len(t, ul.CostItems[0].TierBreakdown, 2)
}

func TestUsageCost_NoPriceConfigured(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c3").
		SetSupportedModels([]string{"m3"}).
		SetDefaultTestModel("m3").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	// preloadModelPrices not called -> no prices cached
	channelService.enabledChannels = []*Channel{built}

	usageLogService := NewUsageLogService(client, systemService, channelService)

	usage := &llm.Usage{
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
	}

	ul, err := usageLogService.CreateUsageLog(ctx, 1, 1, &ch.ID, "m3", usage, "api", "openai/chat_completions")
	require.NoError(t, err)
	require.NotNil(t, ul)
	require.InDelta(t, 0.0, ul.TotalCost, 1e-9)
	require.Len(t, ul.CostItems, 0)
}
