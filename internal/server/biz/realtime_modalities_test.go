// 职责: 验证缓存交集扣除及 Qwen3.5 语音转录免费规则不会影响实际统计。
// 关联文件:
//
//	internal/server/biz/cost_calc.go
//	internal/objects/realtime_price.go
//	llm/model.go

package biz

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestRealtimeCachedModalitiesUseIndependentRates(t *testing.T) {
	usage := &llm.Usage{PromptTokens: 100, CompletionTokens: 30, TotalTokens: 130, Realtime: &llm.RealtimeUsageDetails{InputTextTokens: 40, InputAudioTokens: 50, InputImageTokens: 10, CachedInputTextTokens: 10, CachedInputAudioTokens: 15, CachedInputImageTokens: 5, OutputTextTokens: 10, OutputAudioTokens: 20}}
	rates := map[objects.PriceItemCode]string{objects.PriceItemCodeRealtimeInputText: "1", objects.PriceItemCodeRealtimeInputAudio: "2", objects.PriceItemCodeRealtimeInputImage: "3", objects.PriceItemCodeRealtimeCachedInputText: "0.1", objects.PriceItemCodeRealtimeCachedInputAudio: "0.2", objects.PriceItemCodeRealtimeCachedInputImage: "0.3", objects.PriceItemCodeRealtimeOutputText: "4", objects.PriceItemCodeRealtimeOutputAudio: "5"}
	price := objects.ModelPrice{}
	for code, rate := range rates {
		value := decimal.RequireFromString(rate)
		price.Items = append(price.Items, objects.ModelPriceItem{ItemCode: code, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &value}})
	}
	require.NoError(t, objects.ValidateOpenAIRealtimePrice(price))
	items, total := ComputeRealtimeUsageCost(usage, price, time.Now())
	require.True(t, total.Equal(decimal.RequireFromString("0.0002605")), total.String())
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputText, 30, "0.00003")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputAudio, 35, "0.00007")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeCachedInputImage, 5, "0.0000015")
}

func TestRealtimeFreeSpeechTranscriptRetainsActualTokens(t *testing.T) {
	usage := &llm.Usage{PromptTokens: 100, CompletionTokens: 30, TotalTokens: 130, Realtime: &llm.RealtimeUsageDetails{InputTextTokens: 100, OutputTextTokens: 10, OutputAudioTokens: 20, UnbilledOutputTextTokens: 10}}
	items, total := ComputeRealtimeUsageCost(usage, realtimePrice("1", "2", "3", "4"), time.Now())
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputText, 0, "0")
	require.True(t, total.Equal(decimal.RequireFromString("0.00018")))
	require.EqualValues(t, 30, usage.CompletionTokens)
	require.EqualValues(t, 10, usage.Realtime.OutputTextTokens)
	usage.Realtime.UnbilledOutputTextTokens = 0
	items, total = ComputeRealtimeUsageCost(usage, realtimePrice("1", "2", "3", "4"), time.Now())
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputText, 10, "0.00003")
	require.True(t, total.Equal(decimal.RequireFromString("0.00021")))
}

func TestRealtimeUnusedOrFreeModalitiesDoNotIncurFlatFees(t *testing.T) {
	price := realtimePrice("1", "2", "3", "4")
	for index := range price.Items {
		fee := decimal.NewFromInt(1)
		price.Items[index].Pricing = objects.Pricing{Mode: objects.PricingModeFlatFee, FlatFee: &fee}
	}
	usage := &llm.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3, Realtime: &llm.RealtimeUsageDetails{InputTextTokens: 1, OutputTextTokens: 1, OutputAudioTokens: 1, UnbilledOutputTextTokens: 1}}
	items, total := ComputeRealtimeUsageCost(usage, price, time.Now())
	require.Equal(t, "2", total.String())
	requireCostItem(t, items, objects.PriceItemCodeRealtimeInputAudio, 0, "0")
	requireCostItem(t, items, objects.PriceItemCodeRealtimeOutputText, 0, "0")
}
