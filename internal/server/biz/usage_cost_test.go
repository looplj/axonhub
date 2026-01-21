package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
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
		SetReferenceID("ref-1").
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

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m1",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: (100/1e6)*0.01 + (200/1e6)*0.02 = 0.000005
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.000005, *ul.TotalCost, 1e-12)
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
								{UpTo: lo.ToPtr(int64(1000)), PricePerUnit: decimal.NewFromFloat(0.01)},
								{UpTo: nil, PricePerUnit: decimal.NewFromFloat(0.02)},
							},
						},
					},
				},
			},
		}).
		SetReferenceID("ref-2").
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

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m2",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: (1000/1e6)*0.01 + (500/1e6)*0.02 = 0.00002
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.00002, *ul.TotalCost, 1e-12)
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

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m3",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)
	require.Nil(t, ul.TotalCost)
	require.Len(t, ul.CostItems, 0)
}

func TestUsageCost_CacheVariant5Min(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c4").
		SetSupportedModels([]string{"m4"}).
		SetDefaultTestModel("m4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	// price: prompt_tokens $0.01, write_cached_tokens $0.04 (shared), 5m variant $0.03
	promptUnit := decimal.NewFromFloat(0.01)
	writeCacheShared := decimal.NewFromFloat(0.04)
	writeCache5Min := decimal.NewFromFloat(0.03)

	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("m4").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &promptUnit},
				},
				{
					ItemCode: objects.PriceItemCodeWriteCachedTokens,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &writeCacheShared},
					PromptWriteCacheVariants: []objects.PromptWriteCacheVariant{
						{
							VariantCode: objects.PromptWriteCacheVariantCode5Min,
							Pricing:     objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &writeCache5Min},
						},
					},
				},
			},
		}).
		SetReferenceID("ref-4").
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
		CompletionTokens: 0,
		TotalTokens:      100,
		PromptTokensDetails: &llm.PromptTokensDetails{
			WriteCachedTokens:      50,
			WriteCached5MinTokens:  50,
			WriteCached1HourTokens: 0,
		},
	}

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m4",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: ((100-50)/1e6)*0.01 + (50/1e6)*0.03 = 0.000002
	// Input tokens now exclude WriteCachedTokens: (50/1e6)*0.01 = 0.0000005
	// Write cached 5min: (50/1e6)*0.03 = 0.0000015
	// Total: 0.000002
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.000002, *ul.TotalCost, 1e-12)
	require.Len(t, ul.CostItems, 2)
	require.Equal(t, int64(50), ul.PromptWriteCachedTokens5m)
}

func TestUsageCost_CacheVariant1Hour(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c5").
		SetSupportedModels([]string{"m5"}).
		SetDefaultTestModel("m5").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	// price: prompt_tokens $0.01, write_cached_tokens $0.04 (shared), 1h variant $0.02
	promptUnit := decimal.NewFromFloat(0.01)
	writeCacheShared := decimal.NewFromFloat(0.04)
	writeCache1Hour := decimal.NewFromFloat(0.02)

	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("m5").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &promptUnit},
				},
				{
					ItemCode: objects.PriceItemCodeWriteCachedTokens,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &writeCacheShared},
					PromptWriteCacheVariants: []objects.PromptWriteCacheVariant{
						{
							VariantCode: objects.PromptWriteCacheVariantCode1Hour,
							Pricing:     objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &writeCache1Hour},
						},
					},
				},
			},
		}).
		SetReferenceID("ref-5").
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
		CompletionTokens: 0,
		TotalTokens:      100,
		PromptTokensDetails: &llm.PromptTokensDetails{
			WriteCachedTokens:      80,
			WriteCached5MinTokens:  0,
			WriteCached1HourTokens: 80,
		},
	}

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m5",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: ((100-80)/1e6)*0.01 + (80/1e6)*0.02 = 0.0000018
	// Input tokens now exclude WriteCachedTokens: (20/1e6)*0.01 = 0.0000002
	// Write cached 1hour: (80/1e6)*0.02 = 0.0000016
	// Total: 0.0000018
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.0000018, *ul.TotalCost, 1e-12)
	require.Len(t, ul.CostItems, 2)
	require.Equal(t, int64(80), ul.PromptWriteCachedTokens1h)
}

func TestUsageCost_CacheVariantBoth5MinAnd1Hour(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c6").
		SetSupportedModels([]string{"m6"}).
		SetDefaultTestModel("m6").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	// price: prompt_tokens $0.01, write_cached_tokens $0.06 (shared), 5m variant $0.05, 1h variant $0.03
	promptUnit := decimal.NewFromFloat(0.01)
	writeCacheShared := decimal.NewFromFloat(0.06)
	writeCache5Min := decimal.NewFromFloat(0.05)
	writeCache1Hour := decimal.NewFromFloat(0.03)

	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("m6").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &promptUnit},
				},
				{
					ItemCode: objects.PriceItemCodeWriteCachedTokens,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &writeCacheShared},
					PromptWriteCacheVariants: []objects.PromptWriteCacheVariant{
						{
							VariantCode: objects.PromptWriteCacheVariantCode5Min,
							Pricing:     objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &writeCache5Min},
						},
						{
							VariantCode: objects.PromptWriteCacheVariantCode1Hour,
							Pricing:     objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &writeCache1Hour},
						},
					},
				},
			},
		}).
		SetReferenceID("ref-6").
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
		CompletionTokens: 0,
		TotalTokens:      100,
		PromptTokensDetails: &llm.PromptTokensDetails{
			WriteCachedTokens:      100,
			WriteCached5MinTokens:  40,
			WriteCached1HourTokens: 60,
		},
	}

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m6",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: ((100-100)/1e6)*0.01 + (40/1e6)*0.05 + (60/1e6)*0.03 = 0.0000038
	// Input tokens now exclude WriteCachedTokens: (0/1e6)*0.01 = 0
	// Write cached 5min: (40/1e6)*0.05 = 0.000002
	// Write cached 1hour: (60/1e6)*0.03 = 0.0000018
	// Total: 0.0000038
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.0000038, *ul.TotalCost, 1e-12)
	require.Len(t, ul.CostItems, 3)
	require.Equal(t, int64(40), ul.PromptWriteCachedTokens5m)
	require.Equal(t, int64(60), ul.PromptWriteCachedTokens1h)
}

func TestUsageCost_CacheVariantFallbackToShared(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c7").
		SetSupportedModels([]string{"m7"}).
		SetDefaultTestModel("m7").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	// price: prompt_tokens $0.01, write_cached_tokens $0.04 (shared, no variants)
	promptUnit := decimal.NewFromFloat(0.01)
	writeCacheShared := decimal.NewFromFloat(0.04)

	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("m7").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &promptUnit},
				},
				{
					ItemCode: objects.PriceItemCodeWriteCachedTokens,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &writeCacheShared},
					// No PromptWriteCacheVariants configured
				},
			},
		}).
		SetReferenceID("ref-7").
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
		CompletionTokens: 0,
		TotalTokens:      100,
		PromptTokensDetails: &llm.PromptTokensDetails{
			WriteCachedTokens:      70,
			WriteCached5MinTokens:  0,
			WriteCached1HourTokens: 0,
		},
	}

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m7",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: ((100-70)/1e6)*0.01 + (70/1e6)*0.04 = 0.0000031
	// Input tokens now exclude WriteCachedTokens: (30/1e6)*0.01 = 0.0000003
	// Write cached (shared): (70/1e6)*0.04 = 0.0000028
	// Total: 0.0000031
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.0000031, *ul.TotalCost, 1e-12)
	require.Len(t, ul.CostItems, 2)
	require.Equal(t, int64(70), ul.PromptWriteCachedTokens)
}
