// 职责: 验证模型列表定价提取及不同服务商价格格式的兼容性。
// 关联文件:
//   internal/server/biz/model_pricing_extractor.go

package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestExtractModelPricingProviderShapes(t *testing.T) {
	tests := []struct {
		name       string
		channel    channel.Type
		body       string
		modelID    string
		wantPrices map[objects.PriceItemCode]string
	}{
		{
			name:    "Synthetic per-token strings and dollar prefix in direct array",
			channel: channel.TypeOpenai,
			body: `[{
				"id":"synthetic-model",
				"pricing":{
					"prompt":"$0.0000006",
					"completion":"0.0000012",
					"input_cache_reads":"0.0000001",
					"input_cache_writes":"$0.0000008"
				}
			}]`,
			modelID: "synthetic-model",
			wantPrices: map[objects.PriceItemCode]string{
				objects.PriceItemCodeUsage:             "0.6",
				objects.PriceItemCodeCompletion:        "1.2",
				objects.PriceItemCodePromptCachedToken: "0.1",
				objects.PriceItemCodeWriteCachedTokens: "0.8",
			},
		},
		{
			name:    "Neural Watt per-million metadata in data wrapper",
			channel: channel.TypeOpenai,
			body: `{"data":[{
				"id":"neural-watt-model",
				"metadata":{"pricing":{
					"input_per_million":0.75,
					"output_per_million":1.5,
					"cached_input_per_million":0.12,
					"cached_output_per_million":0.9
				}}
			}]}`,
			modelID: "neural-watt-model",
			wantPrices: map[objects.PriceItemCode]string{
				objects.PriceItemCodeUsage:             "0.75",
				objects.PriceItemCodeCompletion:        "1.5",
				objects.PriceItemCodePromptCachedToken: "0.12",
				objects.PriceItemCodeWriteCachedTokens: "0.9",
			},
		},
		{
			name:    "OpenCode Go cost text TTL dictionary",
			channel: channel.TypeOpencodeGo,
			body: `{"data":[{
				"id":"opencode-model",
				"cost":{"text":{
					"input":{"price":0.4,"cache":{"read":0.2,"write":{"5m":2.5,"1h":3.5}}},
					"output":{"price":1.1}
				}}
			}]}`,
			modelID: "opencode-model",
			wantPrices: map[objects.PriceItemCode]string{
				objects.PriceItemCodeUsage:             "0.4",
				objects.PriceItemCodeCompletion:        "1.1",
				objects.PriceItemCodePromptCachedToken: "0.2",
				objects.PriceItemCodeWriteCachedTokens: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prices, err := ExtractModelPricing(tt.channel, []byte(tt.body))
			require.NoError(t, err)
			price := prices[tt.modelID]
			require.NotNil(t, price)
			require.Len(t, price.Items, len(tt.wantPrices))
			for code, want := range tt.wantPrices {
				item := findModelPriceItem(t, price, code)
				if want == "" {
					require.Equal(t, objects.Pricing{}, item.Pricing)
					continue
				}
				require.Equal(t, want, item.Pricing.UsagePerUnit.String())
				require.Equal(t, objects.PricingModeUsagePerUnit, item.Pricing.Mode)
			}

			if tt.modelID == "opencode-model" {
				writeItem := findModelPriceItem(t, price, objects.PriceItemCodeWriteCachedTokens)
				require.Equal(t, []objects.PromptWriteCacheVariantCode{
					objects.PromptWriteCacheVariantCode5Min,
					objects.PromptWriteCacheVariantCode1Hour,
				}, []objects.PromptWriteCacheVariantCode{
					writeItem.PromptWriteCacheVariants[0].VariantCode,
					writeItem.PromptWriteCacheVariants[1].VariantCode,
				})
				require.Equal(t, "2.5", writeItem.PromptWriteCacheVariants[0].Pricing.UsagePerUnit.String())
				require.Equal(t, "3.5", writeItem.PromptWriteCacheVariants[1].Pricing.UsagePerUnit.String())
			}
		})
	}
}

func TestExtractModelPricingIgnoresNonPositiveAndGarbage(t *testing.T) {
	body := []byte(`{"data":[
		{"id":"zero","pricing":{"prompt":"0","completion":"$0.00"}},
		{"id":"garbage","pricing":{"prompt":"not-a-price","completion":"$bad","input_cache_reads":"-0.5"}},
		{"id":"absent","pricing":{}}
	]}`)

	prices, err := ExtractModelPricing(channel.TypeOpenai, body)
	require.NoError(t, err)
	require.Empty(t, prices)
}

func TestExtractModelPricingCacheWriteSingleEntryFallback(t *testing.T) {
	prices, err := ExtractModelPricing(channel.TypeOpencodeGo, []byte(`{"data":[{
		"id":"single-ttl",
		"cost":{"text":{"input":{"cache":{"write":{"12h":4}}}}}
	}]}`))
	require.NoError(t, err)
	price := prices["single-ttl"]
	require.NotNil(t, price)
	writeItem := findModelPriceItem(t, price, objects.PriceItemCodeWriteCachedTokens)
	require.Equal(t, "4", writeItem.Pricing.UsagePerUnit.String())
	require.Equal(t, []objects.PromptWriteCacheVariantCode{objects.PromptWriteCacheVariantCode1Hour}, []objects.PromptWriteCacheVariantCode{
		writeItem.PromptWriteCacheVariants[0].VariantCode,
	})
	require.Equal(t, "4", writeItem.PromptWriteCacheVariants[0].Pricing.UsagePerUnit.String())
}

func TestExtractModelPricingCacheWriteMixedTTLLeavesGenericEmpty(t *testing.T) {
	prices, err := ExtractModelPricing(channel.TypeOpencodeGo, []byte(`{"data":[{
		"id":"mixed-ttl",
		"cost":{"text":{"input":{"cache":{"write":{"5m":2.5,"1h":3.5}}}}}
	}]}`))
	require.NoError(t, err)
	price := prices["mixed-ttl"]
	require.NotNil(t, price)
	// The variants-only price (empty aggregate) must be persistable through the
	// ent validator, which is what the sync path relies on.
	require.NoError(t, price.Validate())
	writeItem := findModelPriceItem(t, price, objects.PriceItemCodeWriteCachedTokens)
	require.Equal(t, objects.Pricing{}, writeItem.Pricing)
	require.Equal(t, []objects.PromptWriteCacheVariantCode{
		objects.PromptWriteCacheVariantCode5Min,
		objects.PromptWriteCacheVariantCode1Hour,
	}, []objects.PromptWriteCacheVariantCode{
		writeItem.PromptWriteCacheVariants[0].VariantCode,
		writeItem.PromptWriteCacheVariants[1].VariantCode,
	})
	require.Equal(t, "2.5", writeItem.PromptWriteCacheVariants[0].Pricing.UsagePerUnit.String())
	require.Equal(t, "3.5", writeItem.PromptWriteCacheVariants[1].Pricing.UsagePerUnit.String())
}

func TestExtractModelPricingGeminiWrapperAndUnsupportedType(t *testing.T) {
	body := []byte(`{"models":[{"name":"models/gemini-pro","pricing":{"prompt":"0.000001"}}]}`)

	prices, err := ExtractModelPricing(channel.TypeOpenai, body)
	require.NoError(t, err)
	require.Empty(t, prices)

	prices, err = ExtractModelPricing(channel.TypeGemini, body)
	require.NoError(t, err)
	require.Empty(t, prices)

	prices, err = ExtractModelPricing(channel.TypeCodex, []byte(`[{"id":"codex-model","pricing":{"prompt":"0.000001"}}]`))
	require.NoError(t, err)
	require.Empty(t, prices)
}

func TestExtractModelPricingSupportsEndpointTypes(t *testing.T) {
	for _, typ := range []channel.Type{channel.TypeOpenrouter, channel.TypeNanogpt, channel.TypeNanogptResponses, channel.TypeOpencodeGo} {
		require.True(t, supportsEndpointPricing(typ), "%s should use endpoint pricing", typ)
	}
	for _, typ := range []channel.Type{
		channel.TypeCodex,
		channel.TypeGithub,
		channel.TypeGithubCopilot,
		channel.TypeAntigravity,
		channel.TypeCline,
		channel.TypeVolcengine,
		channel.TypeQiniu,
		channel.TypeQiniuAnthropic,
		channel.TypeGemini,
	} {
		require.False(t, supportsEndpointPricing(typ), "%s should not use endpoint pricing", typ)
	}
}

func TestExtractModelPricingDeduplicatesModelPointer(t *testing.T) {
	prices, err := ExtractModelPricing(channel.TypeOpenai, []byte(`[{"id":"same","pricing":{"prompt":"0.000001"}},{"id":"same","pricing":{"prompt":"0.000001"}}]`))
	require.NoError(t, err)
	require.Len(t, prices, 1)
	require.NotNil(t, prices["same"])
}

func findModelPriceItem(t *testing.T, price *objects.ModelPrice, code objects.PriceItemCode) *objects.ModelPriceItem {
	t.Helper()
	for i := range price.Items {
		if price.Items[i].ItemCode == code {
			return &price.Items[i]
		}
	}
	t.Fatalf("price item %s not found", code)
	return nil
}
