// 职责: 从 OpenAI 兼容服务商的模型列表响应中提取模型价格。
// 关联文件:
//   internal/server/biz/model_fetcher.go
//   internal/server/biz/channel_model_sync.go
// 注意事项:
//   仅保留正价格，避免零成本字段覆盖已有价格。

package biz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

// supportsEndpointPricing reports whether a channel fetches models from an
// OpenAI-compatible endpoint whose response may include pricing.
func supportsEndpointPricing(t channel.Type) bool {
	if !t.IsOpenAI() {
		return false
	}

	switch t {
	case channel.TypeCodex,
		channel.TypeGithub,
		channel.TypeGithubCopilot,
		channel.TypeAntigravity,
		channel.TypeCline,
		channel.TypeVolcengine,
		channel.TypeQiniu,
		channel.TypeQiniuAnthropic:
		return false
	default:
		return true
	}
}

// ExtractModelPricing parses the provider model list and returns its positive
// endpoint prices indexed by model ID.
func ExtractModelPricing(channelType channel.Type, body []byte) (map[string]*objects.ModelPrice, error) {
	if !supportsEndpointPricing(channelType) {
		return make(map[string]*objects.ModelPrice), nil
	}

	items, _, err := extractRawModelResponse(body)
	if err != nil {
		return nil, err
	}

	models := make([]ModelIdentify, 0, len(items))
	for _, item := range items {
		var model struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(item, &model); err != nil {
			return nil, fmt.Errorf("failed to parse model ID for pricing: %w", err)
		}
		models = append(models, ModelIdentify{ID: model.ID})
	}

	return extractModelPricingFromRawItems(items, models), nil
}

// extractModelPricingFromRawItems is shared with parseModelsResponse so that
// attaching pricing does not decode the response body a second time.
func extractModelPricingFromRawItems(items []json.RawMessage, models []ModelIdentify) map[string]*objects.ModelPrice {
	prices := make(map[string]*objects.ModelPrice)
	for i, item := range items {
		modelID := models[i].ID
		if modelID == "" {
			continue
		}

		priceItems, ok := extractModelPriceItems(item)
		// Match modelCardToChannelModelPrice: models without any positive cost are not priceable.
		if !ok {
			continue
		}

		prices[modelID] = &objects.ModelPrice{Items: priceItems}
	}

	return prices
}

func extractRawModelResponse(body []byte) ([]json.RawMessage, []GeminiModelResponse, error) {
	var direct []json.RawMessage
	if err := json.Unmarshal(body, &direct); err == nil && direct != nil {
		return direct, nil, nil
	}

	var response commonModelsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		if err := ExtractJSONArray(body, &response.Data); err != nil {
			return nil, nil, fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return response.Data, response.Models, nil
}

func extractModelPriceItems(item json.RawMessage) ([]objects.ModelPriceItem, bool) {
	var model struct {
		Pricing struct {
			Prompt           json.RawMessage `json:"prompt"`
			Completion       json.RawMessage `json:"completion"`
			InputCacheReads  json.RawMessage `json:"input_cache_reads"`
			InputCacheWrites json.RawMessage `json:"input_cache_writes"`
		} `json:"pricing"`
		Metadata struct {
			Pricing struct {
				InputPerMillion        json.RawMessage `json:"input_per_million"`
				OutputPerMillion       json.RawMessage `json:"output_per_million"`
				CachedInputPerMillion  json.RawMessage `json:"cached_input_per_million"`
				CachedOutputPerMillion json.RawMessage `json:"cached_output_per_million"`
			} `json:"pricing"`
		} `json:"metadata"`
		Cost struct {
			Text struct {
				Input struct {
					Price json.RawMessage `json:"price"`
					Cache struct {
						Read  json.RawMessage `json:"read"`
						Write json.RawMessage `json:"write"`
					} `json:"cache"`
				} `json:"input"`
				Output struct {
					Price json.RawMessage `json:"price"`
				} `json:"output"`
			} `json:"text"`
		} `json:"cost"`
	}
	if err := json.Unmarshal(item, &model); err != nil {
		return nil, false
	}

	if items, ok := extractSyntheticPricing(model.Pricing); ok {
		return items, true
	}
	if items, ok := extractMetadataPricing(model.Metadata.Pricing); ok {
		return items, true
	}

	return extractCostPricing(model.Cost.Text)
}

func extractSyntheticPricing(raw struct {
	Prompt           json.RawMessage `json:"prompt"`
	Completion       json.RawMessage `json:"completion"`
	InputCacheReads  json.RawMessage `json:"input_cache_reads"`
	InputCacheWrites json.RawMessage `json:"input_cache_writes"`
},
) ([]objects.ModelPriceItem, bool) {
	items := make([]objects.ModelPriceItem, 0, 4)
	add := func(code objects.PriceItemCode, raw json.RawMessage) {
		value, ok := parseEndpointPrice(raw, true)
		if !ok {
			return
		}
		items = append(items, modelPriceItem(code, value.Mul(decimal.NewFromInt(1_000_000))))
	}

	add(objects.PriceItemCodeUsage, raw.Prompt)
	add(objects.PriceItemCodeCompletion, raw.Completion)
	add(objects.PriceItemCodePromptCachedToken, raw.InputCacheReads)
	add(objects.PriceItemCodeWriteCachedTokens, raw.InputCacheWrites)

	return items, len(items) > 0
}

func extractMetadataPricing(raw struct {
	InputPerMillion        json.RawMessage `json:"input_per_million"`
	OutputPerMillion       json.RawMessage `json:"output_per_million"`
	CachedInputPerMillion  json.RawMessage `json:"cached_input_per_million"`
	CachedOutputPerMillion json.RawMessage `json:"cached_output_per_million"`
},
) ([]objects.ModelPriceItem, bool) {
	items := make([]objects.ModelPriceItem, 0, 4)
	add := func(code objects.PriceItemCode, raw json.RawMessage) {
		value, ok := parseEndpointPrice(raw, false)
		if !ok {
			return
		}
		items = append(items, modelPriceItem(code, value))
	}

	add(objects.PriceItemCodeUsage, raw.InputPerMillion)
	add(objects.PriceItemCodeCompletion, raw.OutputPerMillion)
	add(objects.PriceItemCodePromptCachedToken, raw.CachedInputPerMillion)
	add(objects.PriceItemCodeWriteCachedTokens, raw.CachedOutputPerMillion)

	return items, len(items) > 0
}

func extractCostPricing(raw struct {
	Input struct {
		Price json.RawMessage `json:"price"`
		Cache struct {
			Read  json.RawMessage `json:"read"`
			Write json.RawMessage `json:"write"`
		} `json:"cache"`
	} `json:"input"`
	Output struct {
		Price json.RawMessage `json:"price"`
	} `json:"output"`
},
) ([]objects.ModelPriceItem, bool) {
	items := make([]objects.ModelPriceItem, 0, 4)
	add := func(code objects.PriceItemCode, raw json.RawMessage) {
		value, ok := parseEndpointPrice(raw, false)
		if !ok {
			return
		}
		items = append(items, modelPriceItem(code, value))
	}

	add(objects.PriceItemCodeUsage, raw.Input.Price)
	add(objects.PriceItemCodeCompletion, raw.Output.Price)
	add(objects.PriceItemCodePromptCachedToken, raw.Input.Cache.Read)

	writeItem, ok := extractCacheWritePrice(raw.Input.Cache.Write)
	if ok {
		items = append(items, writeItem)
	}

	return items, len(items) > 0
}

func extractCacheWritePrice(raw json.RawMessage) (objects.ModelPriceItem, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return objects.ModelPriceItem{}, false
	}

	if trimmed[0] != '{' {
		value, ok := parseEndpointPrice(trimmed, false)
		if !ok {
			return objects.ModelPriceItem{}, false
		}
		return modelPriceItem(objects.PriceItemCodeWriteCachedTokens, value), true
	}

	var variants map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &variants); err != nil {
		return objects.ModelPriceItem{}, false
	}

	fiveMin, hasFiveMin := firstPositivePrice(variants, "5m", "30m")
	oneHour, hasOneHour := firstPositivePrice(variants, "1h", "12h")
	if !hasFiveMin && !hasOneHour {
		return objects.ModelPriceItem{}, false
	}

	item := objects.ModelPriceItem{ItemCode: objects.PriceItemCodeWriteCachedTokens}
	if hasFiveMin && hasOneHour {
		// Aggregate cached-write usage has no TTL, so neither variant is a safe fallback.
		item.Pricing = objects.Pricing{}
	} else if hasFiveMin {
		item.Pricing = usagePricing(fiveMin)
	} else {
		item.Pricing = usagePricing(oneHour)
	}
	if hasFiveMin {
		item.PromptWriteCacheVariants = append(item.PromptWriteCacheVariants, objects.PromptWriteCacheVariant{
			VariantCode: objects.PromptWriteCacheVariantCode5Min,
			Pricing:     usagePricing(fiveMin),
		})
	}
	if hasOneHour {
		item.PromptWriteCacheVariants = append(item.PromptWriteCacheVariants, objects.PromptWriteCacheVariant{
			VariantCode: objects.PromptWriteCacheVariantCode1Hour,
			Pricing:     usagePricing(oneHour),
		})
	}

	return item, true
}

func firstPositivePrice(values map[string]json.RawMessage, keys ...string) (decimal.Decimal, bool) {
	for _, key := range keys {
		value, ok := parseEndpointPrice(values[key], false)
		if ok {
			return value, true
		}
	}

	return decimal.Zero, false
}

func parseEndpointPrice(raw json.RawMessage, allowDollarPrefix bool) (decimal.Decimal, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return decimal.Zero, false
	}

	if trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return decimal.Zero, false
		}
		trimmed = strings.TrimSpace(value)
		if allowDollarPrefix {
			trimmed = strings.TrimPrefix(trimmed, "$")
			trimmed = strings.TrimSpace(trimmed)
		}
	} else if allowDollarPrefix {
		return decimal.Zero, false
	}

	value, err := decimal.NewFromString(trimmed)
	if err != nil || !value.IsPositive() {
		return decimal.Zero, false
	}

	return value, true
}

func modelPriceItem(code objects.PriceItemCode, value decimal.Decimal) objects.ModelPriceItem {
	return objects.ModelPriceItem{
		ItemCode: code,
		Pricing:  usagePricing(value),
	}
}

func usagePricing(value decimal.Decimal) objects.Pricing {
	return objects.Pricing{
		Mode:         objects.PricingModeUsagePerUnit,
		UsagePerUnit: &value,
	}
}
