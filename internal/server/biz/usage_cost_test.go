package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
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
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c1").
		SetSupportedModels([]string{"m1"}).
		SetDefaultTestModel("m1").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
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
	channelService.SetEnabledChannelsForTest([]*Channel{built})

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
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c2").
		SetSupportedModels([]string{"m2"}).
		SetDefaultTestModel("m2").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
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
	channelService.SetEnabledChannelsForTest([]*Channel{built})

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

func TestUsageCost_VolumePrompt(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c8").
		SetSupportedModels([]string{"m8"}).
		SetDefaultTestModel("m8").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("m8").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing: objects.Pricing{
						Mode: objects.PricingModeVolume,
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
		SetReferenceID("ref-8").
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, built)
	channelService.SetEnabledChannelsForTest([]*Channel{built})

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
		ActualModelID: "m8",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: (1500/1e6)*0.02 = 0.00003 (all tokens billed at tier 2 price)
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.00003, *ul.TotalCost, 1e-12)
	require.Len(t, ul.CostItems, 1)
	require.Len(t, ul.CostItems[0].TierBreakdown, 1)
	require.Equal(t, int64(1500), ul.CostItems[0].TierBreakdown[0].Units)
}

func TestUsageCost_VolumePromptFirstTier(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c9").
		SetSupportedModels([]string{"m9"}).
		SetDefaultTestModel("m9").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("m9").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing: objects.Pricing{
						Mode: objects.PricingModeVolume,
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
		SetReferenceID("ref-9").
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, built)
	channelService.SetEnabledChannelsForTest([]*Channel{built})

	usageLogService := NewUsageLogService(client, systemService, channelService)

	usage := &llm.Usage{
		PromptTokens:     800,
		CompletionTokens: 0,
		TotalTokens:      800,
	}

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m9",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: (800/1e6)*0.01 = 0.000008 (all tokens billed at tier 1 price)
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.000008, *ul.TotalCost, 1e-12)
	require.Len(t, ul.CostItems, 1)
	require.Len(t, ul.CostItems[0].TierBreakdown, 1)
	require.Equal(t, int64(800), ul.CostItems[0].TierBreakdown[0].Units)
}

func TestUsageCost_NoPriceConfigured(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c3").
		SetSupportedModels([]string{"m3"}).
		SetDefaultTestModel("m3").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	// preloadModelPrices not called -> no prices cached
	channelService.SetEnabledChannelsForTest([]*Channel{built})

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
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c4").
		SetSupportedModels([]string{"m4"}).
		SetDefaultTestModel("m4").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
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
	channelService.SetEnabledChannelsForTest([]*Channel{built})

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
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c5").
		SetSupportedModels([]string{"m5"}).
		SetDefaultTestModel("m5").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
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
	channelService.SetEnabledChannelsForTest([]*Channel{built})

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
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c6").
		SetSupportedModels([]string{"m6"}).
		SetDefaultTestModel("m6").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
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
	channelService.SetEnabledChannelsForTest([]*Channel{built})

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
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c7").
		SetSupportedModels([]string{"m7"}).
		SetDefaultTestModel("m7").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
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
	channelService.SetEnabledChannelsForTest([]*Channel{built})

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

func createModelLibraryPriceEntry(t *testing.T, ctx context.Context, client *ent.Client, modelID string, status model.Status, cost objects.ModelCardCost) *ent.Model {
	t.Helper()

	entity, err := client.Model.Create().
		SetDeveloper("test").
		SetModelID(modelID).
		SetName("Test " + modelID).
		SetIcon("test").
		SetGroup("test").
		SetModelCard(&objects.ModelCard{Cost: cost}).
		SetSettings(&objects.ModelSettings{}).
		SetStatus(status).
		Save(ctx)
	require.NoError(t, err)

	return entity
}

// TestUsageCost_FallbackToModelLibraryPrice verifies that when a channel has no
// explicit price for a model, the cost is computed from the model library price
// configured under the Models menu.
func TestUsageCost_FallbackToModelLibraryPrice(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Model library has a price, but the channel below has no channel-level price.
	createModelLibraryPriceEntry(t, ctx, client, "m-fallback", model.StatusEnabled, objects.ModelCardCost{
		Input:  1,
		Output: 2,
	})

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("fallback-channel").
		SetSupportedModels([]string{"m-fallback"}).
		SetDefaultTestModel("m-fallback").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	// Intentionally do NOT preload channel prices: the channel has none.
	channelService.SetEnabledChannelsForTest([]*Channel{built})

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
		ActualModelID: "m-fallback",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: (100/1e6)*1 + (200/1e6)*2 = 0.0001 + 0.0004 = 0.0005
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.0005, *ul.TotalCost, 1e-12)
	require.Len(t, ul.CostItems, 2)
	require.Equal(t, modelLibraryPriceReferenceID("m-fallback"), ul.CostPriceReferenceID)
}

// TestUsageCost_ChannelPriceWinsOverModelLibrary verifies that an explicit
// channel-level price still takes precedence over the model library fallback.
func TestUsageCost_ChannelPriceWinsOverModelLibrary(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	createModelLibraryPriceEntry(t, ctx, client, "m-precedence", model.StatusEnabled, objects.ModelCardCost{
		Input: 100,
	})

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("precedence-channel").
		SetSupportedModels([]string{"m-precedence"}).
		SetDefaultTestModel("m-precedence").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	usageUnit := decimal.NewFromFloat(0.01)
	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("m-precedence").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &usageUnit},
				},
			},
		}).
		SetReferenceID("channel-ref").
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, built)
	channelService.SetEnabledChannelsForTest([]*Channel{built})

	usageLogService := NewUsageLogService(client, systemService, channelService)

	usage := &llm.Usage{
		PromptTokens:     100,
		CompletionTokens: 0,
		TotalTokens:      100,
	}

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m-precedence",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// Channel price (0.01/1M) must win over the model library price (100/1M).
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.000001, *ul.TotalCost, 1e-12)
	require.Equal(t, "channel-ref", ul.CostPriceReferenceID)
}

// TestUsageCost_FallbackSkipsUnpriceableModelLibraryEntries verifies that the
// fallback stays inert when the model library entry has no usable cost.
func TestUsageCost_FallbackSkipsUnpriceableModelLibraryEntries(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Enabled model without cost, archived model with cost: neither is priceable.
	createModelLibraryPriceEntry(t, ctx, client, "m-no-cost", model.StatusEnabled, objects.ModelCardCost{})
	createModelLibraryPriceEntry(t, ctx, client, "m-archived", model.StatusArchived, objects.ModelCardCost{Input: 5})

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("unpriceable-channel").
		SetSupportedModels([]string{"m-no-cost", "m-archived", "m-missing"}).
		SetDefaultTestModel("m-no-cost").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	channelService.SetEnabledChannelsForTest([]*Channel{built})

	usageLogService := NewUsageLogService(client, systemService, channelService)

	usage := &llm.Usage{PromptTokens: 100, CompletionTokens: 0, TotalTokens: 100}

	for _, modelID := range []string{"m-no-cost", "m-archived", "m-missing"} {
		ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
			RequestID:     1,
			ProjectID:     1,
			ChannelID:     ch.ID,
			ActualModelID: modelID,
			Usage:         usage,
			Source:        "api",
			Format:        "openai/chat_completions",
			APIKeyID:      nil,
		})
		require.NoError(t, err)
		require.NotNil(t, ul)
		require.Nil(t, ul.TotalCost, "model %s should have no cost", modelID)
		require.Empty(t, ul.CostItems)
		require.Empty(t, ul.CostPriceReferenceID)
	}
}

// TestUsageCost_FallbackStripsEffortSuffix verifies that when the model library
// has no exact entry for a suffixed model id, the fallback strips the known
// effort/tier suffix and bills with the base model price.
func TestUsageCost_FallbackStripsEffortSuffix(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Only the base model "m-suffix" exists in the library; the channel serves
	// "m-suffix-high".
	createModelLibraryPriceEntry(t, ctx, client, "m-suffix", model.StatusEnabled, objects.ModelCardCost{
		Input:  1,
		Output: 2,
	})

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("suffix-channel").
		SetSupportedModels([]string{"m-suffix-high"}).
		SetDefaultTestModel("m-suffix-high").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	channelService.SetEnabledChannelsForTest([]*Channel{built})

	usageLogService := NewUsageLogService(client, systemService, channelService)

	usage := &llm.Usage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300}

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m-suffix-high",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// expected total: (100/1e6)*1 + (200/1e6)*2 = 0.0005
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.0005, *ul.TotalCost, 1e-12)
	// Reference id keeps the requested (suffixed) model id for traceability.
	require.Equal(t, modelLibraryPriceReferenceID("m-suffix-high"), ul.CostPriceReferenceID)
}

// TestUsageCost_FallbackPrefersExactModelBeforeSuffix verifies that an exact
// model library entry for the suffixed id wins over the stripped base model.
func TestUsageCost_FallbackPrefersExactModelBeforeSuffix(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	createModelLibraryPriceEntry(t, ctx, client, "m-exact", model.StatusEnabled, objects.ModelCardCost{Input: 10})
	createModelLibraryPriceEntry(t, ctx, client, "m-exact-high", model.StatusEnabled, objects.ModelCardCost{Input: 1})
	{
		n, qerr := client.Model.Query().Count(ctx)
		require.NoError(t, qerr)
		require.Equal(t, 2, n, "both library entries must exist")
		m, merr := client.Model.Query().Where(model.ModelID("m-exact-high")).Only(ctx)
		require.NoError(t, merr, "exact entry must be queryable")
		require.Equal(t, float64(1), m.ModelCard.Cost.Input)
	}

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("exact-channel").
		SetSupportedModels([]string{"m-exact-high"}).
		SetDefaultTestModel("m-exact-high").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	built, err := channelService.GetChannel(ctx, ch.ID)
	require.NoError(t, err)
	channelService.SetEnabledChannelsForTest([]*Channel{built})

	usageLogService := NewUsageLogService(client, systemService, channelService)

	usage := &llm.Usage{PromptTokens: 100, CompletionTokens: 0, TotalTokens: 100}

	ul, err := usageLogService.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     1,
		ProjectID:     1,
		ChannelID:     ch.ID,
		ActualModelID: "m-exact-high",
		Usage:         usage,
		Source:        "api",
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, ul)

	// Exact "m-exact-high" price (1/1M) must win over base "m-exact" (10/1M).
	// (100 / 1e6) * 1 = 0.0001; the base price would yield 0.001.
	require.NotNil(t, ul.TotalCost)
	require.InDelta(t, 0.0001, *ul.TotalCost, 1e-12)
}

// TestModelLibraryPriceModelIDCandidates covers candidate ordering and de-dup.
func TestModelLibraryPriceModelIDCandidates(t *testing.T) {
	require.Equal(t, []string{"m"}, modelLibraryPriceModelIDCandidates("m"))
	require.Equal(t, []string{"m-high", "m"}, modelLibraryPriceModelIDCandidates("m-high"))
	require.Equal(t, []string{"m-low", "m"}, modelLibraryPriceModelIDCandidates("m-low"))
	require.Equal(t, []string{"m-medium", "m"}, modelLibraryPriceModelIDCandidates("m-medium"))
	// A trailing dash is not a known suffix; nothing is stripped.
	require.Equal(t, []string{"m-high-"}, modelLibraryPriceModelIDCandidates("m-high-"))
}

// TestModelLibraryPriceMemo_ReusesLookupWithinRequest verifies that repeated
// resolutions inside one request reuse a single catalog lookup, including a
// miss, while a fresh request re-queries (so later price changes are observed).
func TestModelLibraryPriceMemo_ReusesLookupWithinRequest(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	createModelLibraryPriceEntry(t, ctx, client, "m-memo", model.StatusEnabled, objects.ModelCardCost{Input: 1})

	channelService := NewChannelServiceForTest(client)

	// --- inside a memoized request: the miss is cached ---
	memoCtx := WithModelLibraryPriceMemo(ctx)

	price, ok := channelService.resolveModelLibraryPrice(memoCtx, "m-memo")
	require.True(t, ok)
	require.Equal(t, "1", price.Items[0].Pricing.UsagePerUnit.String())

	// A model absent from the library is cached as a miss within the request.
	_, missing := channelService.resolveModelLibraryPrice(memoCtx, "m-absent")
	require.False(t, missing)

	// Add the previously missing model; the memoized request must not observe it.
	createModelLibraryPriceEntry(t, ctx, client, "m-absent", model.StatusEnabled, objects.ModelCardCost{Input: 9})

	_, stillMissing := channelService.resolveModelLibraryPrice(memoCtx, "m-absent")
	require.False(t, stillMissing, "memoized miss must be reused within the request")

	// A fresh request (new memo) sees the newly added price.
	freshCtx := WithModelLibraryPriceMemo(ctx)
	added, addedOK := channelService.resolveModelLibraryPrice(freshCtx, "m-absent")
	require.True(t, addedOK)
	require.Equal(t, "9", added.Items[0].Pricing.UsagePerUnit.String())
}

// TestWithModelLibraryPriceMemo_Idempotent verifies the memo is installed once
// per context chain.
func TestWithModelLibraryPriceMemo_Idempotent(t *testing.T) {
	ctx := WithModelLibraryPriceMemo(context.Background())
	require.NotNil(t, modelLibraryPriceMemoFromContext(ctx))

	second := WithModelLibraryPriceMemo(ctx)
	require.Same(t, modelLibraryPriceMemoFromContext(ctx), modelLibraryPriceMemoFromContext(second))
}

// TestModelLibraryPriceMemo_DoesNotCacheLookupErrors verifies that a failed
// lookup (e.g. a canceled context) is not cached, so a later calculation with a
// healthy context still resolves the price. This guards the usage-log write,
// which runs with cancellation removed but shares the request memo.
func TestModelLibraryPriceMemo_DoesNotCacheLookupErrors(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	createModelLibraryPriceEntry(t, ctx, client, "m-err", model.StatusEnabled, objects.ModelCardCost{Input: 2})

	channelService := NewChannelServiceForTest(client)

	memoCtx := WithModelLibraryPriceMemo(ctx)

	// A canceled context makes the lookup fail; the result must not be memoized.
	canceledCtx, cancel := context.WithCancel(memoCtx)
	cancel()

	_, ok := channelService.resolveModelLibraryPrice(canceledCtx, "m-err")
	require.False(t, ok)

	// The same request, now with a healthy context, must still resolve the price.
	price, ok := channelService.resolveModelLibraryPrice(memoCtx, "m-err")
	require.True(t, ok)
	require.Equal(t, "2", price.Items[0].Pricing.UsagePerUnit.String())
}
