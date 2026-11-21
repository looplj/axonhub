// 职责: 按实际发送的 PCM 样本数计算实时翻译费用并写入现有用量账。
// 关联文件:
//   internal/server/biz/realtime_billing.go
//   internal/objects/price.go
// 注意事项: 单价为每分钟，明细数量是 24 kHz PCM 样本数，上游不返回账单用量。

package biz

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

func translationRate(items []objects.ModelPriceItem) (decimal.Decimal, error) {
	var rate decimal.Decimal
	for _, item := range items {
		if item.ItemCode != objects.PriceItemCodeRealtimeTranslationAudioSamples {
			continue
		}
		if rate.IsPositive() || item.Pricing.Mode != objects.PricingModeUsagePerUnit ||
			item.Pricing.UsagePerUnit == nil || !item.Pricing.UsagePerUnit.IsPositive() {
			return decimal.Zero, errors.New("translation requires exactly one positive audio price per minute")
		}
		rate = *item.Pricing.UsagePerUnit
	}
	if !rate.IsPositive() {
		return decimal.Zero, errors.New("missing translation audio price per minute")
	}
	return rate, nil
}

func validateTranslationPrice(price objects.ModelPrice) error {
	if err := price.Validate(); err != nil {
		return err
	}
	if _, err := translationRate(price.Items); err != nil {
		return err
	}
	if price.Schedule != nil {
		for _, override := range price.Schedule.Overrides {
			if _, err := translationRate(override.Items); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *UsageLogService) RealtimeTranslationPrice(ctx context.Context, channelID int, model string) (*RealtimePriceSnapshot, error) {
	return s.realtimePrice(ctx, channelID, model, validateTranslationPrice, func(code objects.PriceItemCode) bool {
		return code == objects.PriceItemCodeRealtimeTranslationAudioSamples
	})
}

func RealtimeTranslationCost(snapshot *RealtimePriceSnapshot, samples int64) (decimal.Decimal, error) {
	if snapshot == nil || snapshot.Price.Schedule != nil || samples < 0 {
		return decimal.Zero, errors.New("invalid translation price snapshot or sample count")
	}
	rate, err := translationRate(snapshot.Price.Items)
	if err != nil {
		return decimal.Zero, err
	}
	return rate.Mul(decimal.NewFromInt(samples)).DivRound(decimal.NewFromInt(openairt.TranslationSamplesPerMinute), 16), nil
}

func (s *UsageLogService) CreateRealtimeTranslationUsage(ctx context.Context, params CreateUsageLogParams, snapshot *RealtimePriceSnapshot, samples int64) (*ent.UsageLog, error) {
	params.Usage = &llm.Usage{}
	if snapshot == nil && samples >= 0 {
		return s.saveRealtimeUsage(ctx, params, nil, nil, decimal.Zero)
	}
	total, err := RealtimeTranslationCost(snapshot, samples)
	if err != nil {
		return nil, err
	}
	items := []objects.CostItem{{ItemCode: objects.PriceItemCodeRealtimeTranslationAudioSamples, Quantity: samples, Subtotal: total}}
	return s.saveRealtimeUsage(ctx, params, snapshot, items, total)
}
