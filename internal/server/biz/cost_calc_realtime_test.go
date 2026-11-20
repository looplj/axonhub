// 职责: 覆盖 Realtime 四类 token 的精确计费与历史价格码隔离。
// 关联文件:
//
//	internal/server/biz/cost_calc.go
//	internal/objects/price.go

package biz

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func realtimePrice(inputText, inputAudio, outputText, outputAudio string) objects.ModelPrice {
	return objects.ModelPrice{Items: []objects.ModelPriceItem{
		{
			ItemCode: objects.PriceItemCodeRealtimeInputText,
			Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr(inputText)},
		},
		{
			ItemCode: objects.PriceItemCodeRealtimeInputAudio,
			Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr(inputAudio)},
		},
		{
			ItemCode: objects.PriceItemCodeRealtimeOutputText,
			Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr(outputText)},
		},
		{
			ItemCode: objects.PriceItemCodeRealtimeOutputAudio,
			Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr(outputAudio)},
		},
	}}
}

func requireCostItem(t *testing.T, items []objects.CostItem, code objects.PriceItemCode, quantity int64, subtotal string) {
	t.Helper()

	var found *objects.CostItem

	for i := range items {
		if items[i].ItemCode == code {
			found = &items[i]
			break
		}
	}

	require.NotNil(t, found, "missing cost item %s", code)
	require.Equal(t, quantity, found.Quantity, "quantity of %s", code)
	require.True(t, found.Subtotal.Equal(decimal.RequireFromString(subtotal)),
		"subtotal of %s: want %s, got %s", code, subtotal, found.Subtotal)
}

func TestComputeUsageCost_RealtimeTokens(t *testing.T) {
	usage := &llm.Usage{
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

	items, total := ComputeRealtimeUsageCost(usage, realtimePrice("1.10", "2.20", "3.30", "4.40"), time.Now())

	// (700*1.10 + 300*2.20 + 400*3.30 + 100*4.40) / 1e6
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputText, 700, "0.00077")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputAudio, 300, "0.00066")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputText, 400, "0.00132")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputAudio, 100, "0.00044")
	require.Len(t, items, 4)

	require.True(t, total.Equal(decimal.RequireFromString("0.00319")), "total: %s", total)
}

func TestComputeUsageCost_RealtimeAudioOnly(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens:            400,
		CompletionTokens:        200,
		TotalTokens:             600,
		PromptTokensDetails:     &llm.PromptTokensDetails{AudioTokens: 400},
		CompletionTokensDetails: &llm.CompletionTokensDetails{AudioTokens: 200},
	}

	items, total := ComputeRealtimeUsageCost(usage, realtimePrice("1", "2", "3", "4"), time.Now())

	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputText, 0, "0")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputAudio, 400, "0.0008")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputText, 0, "0")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputAudio, 200, "0.0008")
	require.True(t, total.Equal(decimal.RequireFromString("0.0016")), "total: %s", total)
}

func TestComputeUsageCost_RealtimeClampsAudioAboveTotal(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens:            100,
		CompletionTokens:        50,
		TotalTokens:             150,
		PromptTokensDetails:     &llm.PromptTokensDetails{AudioTokens: 150},
		CompletionTokensDetails: &llm.CompletionTokensDetails{AudioTokens: 80},
	}

	items, _ := ComputeRealtimeUsageCost(usage, realtimePrice("1", "1", "1", "1"), time.Now())

	// Audio is clamped to the reported total so text + audio never exceed it.
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputText, 0, "0")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputAudio, 100, "0.0001")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputText, 0, "0")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputAudio, 50, "0.00005")
}

func TestComputeUsageCost_RealtimeMissingDetails(t *testing.T) {
	usage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}

	items, total := ComputeRealtimeUsageCost(usage, realtimePrice("1", "2", "3", "4"), time.Now())

	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputText, 1000, "0.001")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputAudio, 0, "0")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputText, 500, "0.0015")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputAudio, 0, "0")
	require.True(t, total.Equal(decimal.RequireFromString("0.0025")), "total: %s", total)
}

func TestComputeUsageCost_RealtimeScheduleOverride(t *testing.T) {
	price := realtimePrice("1", "2", "3", "4")
	price.Schedule = &objects.PriceSchedule{
		Timezone: "UTC",
		Overrides: []objects.PriceOverride{{
			Name:     "night",
			Priority: 1,
			When:     objects.OverrideWhen{DailyTime: &objects.DailyTimeRange{Start: "00:00", End: "23:59"}},
			Items:    realtimePrice("10", "20", "30", "40").Items,
		}},
	}

	usage := &llm.Usage{
		PromptTokens:            1000,
		CompletionTokens:        500,
		TotalTokens:             1500,
		PromptTokensDetails:     &llm.PromptTokensDetails{AudioTokens: 300},
		CompletionTokensDetails: &llm.CompletionTokensDetails{AudioTokens: 100},
	}

	items, total := ComputeRealtimeUsageCost(usage, price, time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC))

	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputText, 700, "0.007")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputAudio, 300, "0.006")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputText, 400, "0.012")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputAudio, 100, "0.004")
	require.True(t, total.Equal(decimal.RequireFromString("0.029")), "total: %s", total)
}

func TestComputeUsageCost_LegacyCodesIgnoreRealtimeAudio(t *testing.T) {
	// The historical prompt_tokens/completion_tokens semantics must not change:
	// their quantities include audio tokens even when the breakdown is present.
	usage := &llm.Usage{
		PromptTokens:            1000,
		CompletionTokens:        500,
		TotalTokens:             1500,
		PromptTokensDetails:     &llm.PromptTokensDetails{AudioTokens: 300},
		CompletionTokensDetails: &llm.CompletionTokensDetails{AudioTokens: 100},
	}

	price := objects.ModelPrice{Items: []objects.ModelPriceItem{
		{
			ItemCode: objects.PriceItemCodeUsage,
			Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("0.03")},
		},
		{
			ItemCode: objects.PriceItemCodeCompletion,
			Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("0.06")},
		},
	}}

	items, total := ComputeUsageCost(usage, price, time.Now())

	requireCostItem(t, items, objects.PriceItemCodeUsage, 1000, "0.00003")
	requireCostItem(t, items, objects.PriceItemCodeCompletion, 500, "0.00003")
	require.True(t, total.Equal(decimal.RequireFromString("0.00006")), "total: %s", total)
}

func TestComputeUsageCostKeepsHTTPBillingWhenPriceContainsRealtimeItems(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens:     100,
		CompletionTokens: 20,
		TotalTokens:      120,
		PromptTokensDetails: &llm.PromptTokensDetails{
			ImageTokens: 10,
		},
	}
	price := objects.ModelPrice{Items: []objects.ModelPriceItem{
		{ItemCode: objects.PriceItemCodeUsage, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("1")}},
		{ItemCode: objects.PriceItemCodeCompletion, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("2")}},
		{ItemCode: objects.PriceItemCodeRealtimeInputImage, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: mustDecimalPtr("3")}},
	}}

	items, total := ComputeUsageCost(usage, price, time.Now())

	require.True(t, total.Equal(decimal.RequireFromString("0.00014")), total.String())
	require.Len(t, items, 2, "ordinary usage logs must not contain Realtime cost items")
	requireCostItem(t, items, objects.PriceItemCodeUsage, 100, "0.0001")
	requireCostItem(t, items, objects.PriceItemCodeCompletion, 20, "0.00004")
}
