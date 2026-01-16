package biz

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
)

func TestUsageLogService_CreateUsageLog_PromptWriteCachedTokens(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	p, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetModelID("test-model").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	channelService := NewChannelServiceForTest(client)
	svc := NewUsageLogService(client, systemService, channelService)

	usage := &llm.Usage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens:      2,
			WriteCachedTokens: 3,
		},
	}

	created, err := svc.CreateUsageLog(
		ctx,
		req.ID,
		p.ID,
		nil,
		"test-model",
		usage,
		usagelog.SourceAPI,
		"openai/chat_completions",
	)
	require.NoError(t, err)
	require.NotNil(t, created)

	require.Equal(t, int64(2), created.PromptCachedTokens)
	require.Equal(t, int64(3), created.PromptWriteCachedTokens)
}

func TestUsageLogService_CreateUsageLog_WithPriceReferenceID(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create project
	p, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Create channel
	ch, err := client.Channel.Create().
		SetName("test-channel").
		SetType("openai").
		SetBaseURL("https://api.openai.com/v1").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		Save(ctx)
	require.NoError(t, err)

	// Create model price with reference ID
	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetRefreanceID("test-ref-123").
		SetPrice(objects.ModelPrice{
			Items: []objects.ModelPriceItem{
				{
					ItemCode: objects.PriceItemCodeUsage,
					Pricing: objects.Pricing{
						Mode:         objects.PricingModeUsagePerUnit,
						UsagePerUnit: toDecimalPtr("0.03"),
					},
				},
				{
					ItemCode: objects.PriceItemCodeCompletion,
					Pricing: objects.Pricing{
						Mode:         objects.PricingModeUsagePerUnit,
						UsagePerUnit: toDecimalPtr("0.06"),
					},
				},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Create request
	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	channelService := NewChannelServiceForTest(client)

	// Preload the channel with model prices
	enabledCh, err := channelService.buildChannel(ch)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, enabledCh)

	// Add to enabled channels list so it can be found by GetEnabledChannel
	channelService.enabledChannels = []*Channel{enabledCh}

	// Verify cache contains the model price
	require.NotNil(t, enabledCh.cachedModelPrices["gpt-4"])
	require.Equal(t, "test-ref-123", enabledCh.cachedModelPrices["gpt-4"].RefreanceID)

	svc := NewUsageLogService(client, systemService, channelService)

	// Create usage log with price calculation
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}

	channelID := ch.ID
	created, err := svc.CreateUsageLog(
		ctx,
		req.ID,
		p.ID,
		&channelID,
		"gpt-4",
		usage,
		usagelog.SourceAPI,
		"openai/chat_completions",
	)
	require.NoError(t, err)
	require.NotNil(t, created)

	// Verify price_reference_id is set
	require.Equal(t, "test-ref-123", created.CostPriceReferenceID)
	require.NotNil(t, created.TotalCost)
	require.NotEmpty(t, created.CostItems)

	// Verify cost calculation is correct
	// (1000 / 1_000_000) * 0.03 + (500 / 1_000_000) * 0.06 = 0.00003 + 0.00003 = 0.00006
	require.InDelta(t, 0.00006, *created.TotalCost, 0.0000001)
}

func toDecimalPtr(s string) *decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return &d
}
