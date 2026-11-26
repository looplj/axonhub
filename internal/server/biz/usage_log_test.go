package biz

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate"
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
	ctx = authz.WithTestBypass(ctx)

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

	created, err := svc.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     req.ID,
		ProjectID:     p.ID,
		ChannelID:     0,
		ActualModelID: "test-model",
		Usage:         usage,
		Source:        usagelog.SourceAPI,
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, created)

	require.Equal(t, int64(2), created.PromptCachedTokens)
	require.Equal(t, int64(3), created.PromptWriteCachedTokens)
}

func TestUsageLogService_CreateUsageLog_CacheReadReporting(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	p, err := client.Project.Create().SetName("cache-read-project").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)
	req, err := client.Request.Create().SetProjectID(p.ID).SetModelID("test-model").
		SetStatus(request.StatusCompleted).SetRequestBody(objects.JSONRawMessage(`{}`)).Save(ctx)
	require.NoError(t, err)
	svc := NewUsageLogService(client,
		NewSystemService(SystemServiceParams{CacheConfig: xcache.Config{}, Ent: client}),
		NewChannelServiceForTest(client))

	for _, tt := range []struct {
		name   string
		status llm.CacheReadStatus
		cached int64
		want   llm.CacheReadStatus
	}{
		{"reported zero", llm.CacheReadReported, 0, llm.CacheReadReported},
		{"reported positive", llm.CacheReadReported, 2, llm.CacheReadReported},
		{"missing", "", 0, llm.CacheReadUnknown},
		{"unknown with historical count", llm.CacheReadUnknown, 2, llm.CacheReadUnknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			created, err := svc.CreateUsageLog(ctx, CreateUsageLogParams{
				RequestID:     req.ID,
				ProjectID:     p.ID,
				ActualModelID: "test-model",
				Usage: &llm.Usage{
					PromptTokens: 10, CacheReadStatus: tt.status,
					PromptTokensDetails: &llm.PromptTokensDetails{CachedTokens: tt.cached},
				},
				Source: usagelog.SourceAPI,
				Format: "openai/chat_completions",
			})
			require.NoError(t, err)
			persisted, err := client.UsageLog.Get(ctx, created.ID)
			require.NoError(t, err)
			require.Equal(t, string(tt.want), persisted.CacheReadStatus)
			require.Equal(t, tt.cached, persisted.PromptCachedTokens)
		})
	}

	defaulted, err := client.UsageLog.Create().SetProjectID(p.ID).SetRequestID(req.ID).
		SetModelID("test-model").SetPromptCachedTokens(2).Save(ctx)
	require.NoError(t, err)
	require.Equal(t, string(llm.CacheReadUnknown), defaulted.CacheReadStatus)
}

func TestUsageLogCacheReadReportingMigration(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))

	legacy, err := schema.CopyTables(migrate.Tables)
	require.NoError(t, err)
	for _, table := range legacy {
		if table.Name == usagelog.Table {
			table.Columns = slices.DeleteFunc(table.Columns, func(column *schema.Column) bool {
				return column.Name == usagelog.FieldCacheReadStatus
			})
		}
	}
	require.NoError(t, migrate.Create(ctx, client.Schema, legacy))
	p, err := client.Project.Create().SetName("legacy-cache-read-project").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)
	req, err := client.Request.Create().SetProjectID(p.ID).SetModelID("test-model").
		SetStatus(request.StatusCompleted).SetRequestBody(objects.JSONRawMessage(`{}`)).Save(ctx)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO usage_logs
		(created_at, updated_at, project_id, request_id, model_id, prompt_cached_tokens)
		VALUES (CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?, ?, 'test-model', 0),
		       (CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?, ?, 'test-model', 5)`, p.ID, req.ID, p.ID, req.ID)
	require.NoError(t, err)

	for range 2 {
		require.NoError(t, client.Schema.Create(ctx))
		rows, err := client.UsageLog.Query().Order(ent.Asc(usagelog.FieldID)).All(ctx)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		require.Equal(t, int64(0), rows[0].PromptCachedTokens)
		require.Equal(t, int64(5), rows[1].PromptCachedTokens)
		for _, row := range rows {
			require.Equal(t, string(llm.CacheReadUnknown), row.CacheReadStatus)
		}
	}
}

func TestUsageLogService_CreateUsageLog_WithPriceReferenceID(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

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
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		Save(ctx)
	require.NoError(t, err)

	// Create model price with reference ID
	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetReferenceID("test-ref-123").
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
	enabledCh, err := channelService.buildChannelWithTransformer(ch)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, enabledCh)

	// Add to enabled channels list so it can be found by GetEnabledChannel
	channelService.SetEnabledChannelsForTest([]*Channel{enabledCh})

	// Verify cache contains the model price
	require.NotNil(t, enabledCh.cachedModelPrices["gpt-4"])
	require.Equal(t, "test-ref-123", enabledCh.cachedModelPrices["gpt-4"].ReferenceID)

	svc := NewUsageLogService(client, systemService, channelService)

	// Create usage log with price calculation
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}

	channelID := ch.ID
	created, err := svc.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     req.ID,
		ProjectID:     p.ID,
		ChannelID:     channelID,
		ActualModelID: "gpt-4",
		Usage:         usage,
		Source:        usagelog.SourceAPI,
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
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

func TestUsageLogService_CreateUsageLog_WithCachedTokens(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

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
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		Save(ctx)
	require.NoError(t, err)

	// Create model price with reference ID
	// Input tokens: $0.03 per 1M tokens
	// Completion tokens: $0.06 per 1M tokens
	// Cached tokens: $0.015 per 1M tokens (50% discount)
	_, err = client.ChannelModelPrice.Create().
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetReferenceID("test-ref-cached").
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
				{
					ItemCode: objects.PriceItemCodePromptCachedToken,
					Pricing: objects.Pricing{
						Mode:         objects.PricingModeUsagePerUnit,
						UsagePerUnit: toDecimalPtr("0.015"),
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
	enabledCh, err := channelService.buildChannelWithTransformer(ch)
	require.NoError(t, err)
	channelService.preloadModelPrices(ctx, enabledCh)

	// Add to enabled channels list so it can be found by GetEnabledChannel
	channelService.SetEnabledChannelsForTest([]*Channel{enabledCh})

	// Verify cache contains the model price
	require.NotNil(t, enabledCh.cachedModelPrices["gpt-4"])
	require.Equal(t, "test-ref-cached", enabledCh.cachedModelPrices["gpt-4"].ReferenceID)

	svc := NewUsageLogService(client, systemService, channelService)

	// Create usage log with cached tokens
	// Total prompt tokens: 1000 (includes 300 cached tokens)
	// Billable prompt tokens: 700 (1000 - 300)
	// Cached tokens: 300 (read from cache, charged at discounted rate)
	// Completion tokens: 500
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens: 300,
		},
	}

	channelID := ch.ID
	created, err := svc.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     req.ID,
		ProjectID:     p.ID,
		ChannelID:     channelID,
		ActualModelID: "gpt-4",
		Usage:         usage,
		Source:        usagelog.SourceAPI,
		Format:        "openai/chat_completions",
		APIKeyID:      nil,
	})
	require.NoError(t, err)
	require.NotNil(t, created)

	// Verify price_reference_id is set
	require.Equal(t, "test-ref-cached", created.CostPriceReferenceID)
	require.NotNil(t, created.TotalCost)
	require.NotEmpty(t, created.CostItems)

	// Verify cost calculation excludes cached tokens from input cost
	// Expected cost:
	// - Input tokens (billable): (700 / 1_000_000) * 0.03 = 0.000021
	// - Cached tokens: (300 / 1_000_000) * 0.015 = 0.0000045
	// - Completion tokens: (500 / 1_000_000) * 0.06 = 0.00003
	// Total: 0.000021 + 0.0000045 + 0.00003 = 0.0000555
	expectedCost := 0.0000555
	require.InDelta(t, expectedCost, *created.TotalCost, 0.0000001)

	// Verify cost items breakdown
	require.Len(t, created.CostItems, 3)

	// Find each cost item and verify
	var inputItem, cachedItem, completionItem *objects.CostItem

	for i := range created.CostItems {
		switch created.CostItems[i].ItemCode {
		case objects.PriceItemCodeUsage:
			inputItem = &created.CostItems[i]
		case objects.PriceItemCodePromptCachedToken:
			cachedItem = &created.CostItems[i]
		case objects.PriceItemCodeCompletion:
			completionItem = &created.CostItems[i]
		}
	}

	require.NotNil(t, inputItem, "input cost item should exist")
	require.NotNil(t, cachedItem, "cached cost item should exist")
	require.NotNil(t, completionItem, "completion cost item should exist")

	// Verify input tokens quantity excludes cached tokens
	require.Equal(t, int64(700), inputItem.Quantity, "input quantity should be 700 (1000 - 300 cached)")
	require.InDelta(t, 0.000021, inputItem.Subtotal.InexactFloat64(), 0.0000001)

	// Verify cached tokens quantity
	require.Equal(t, int64(300), cachedItem.Quantity, "cached quantity should be 300")
	require.InDelta(t, 0.0000045, cachedItem.Subtotal.InexactFloat64(), 0.0000001)

	// Verify completion tokens quantity
	require.Equal(t, int64(500), completionItem.Quantity, "completion quantity should be 500")
	require.InDelta(t, 0.00003, completionItem.Subtotal.InexactFloat64(), 0.0000001)
}

func toDecimalPtr(s string) *decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return &d
}
