// 职责: 冻结实时会话已配置的模态费率，并在渠道失效后仍按快照精确记账。
// 关联文件:
//
//	internal/objects/price.go
//	internal/server/biz/cost_calc.go
//	internal/server/biz/channel_price.go
//	internal/server/biz/usage_log.go
//
// 注意事项:
//
//	快照深拷贝，渠道价格重载或会话中渠道被禁用都不得改变已开始的会话账单。
//	CreateRealtimeUsage 不得重查价格：渠道禁用后重查会丢失计量。
//	Realtime 模态价格码与历史 prompt_tokens/completion_tokens 语义独立，互不参与计算。
//	审计用 cost_items 保存 decimal 小计，total_cost float 仅用于历史兼容。

package biz

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// RealtimePriceSnapshot is an immutable price quote for one realtime session.
// Price and ReferenceID are detached from the channel price cache so a later
// price update or channel disable cannot change the billing of this session.
type RealtimePriceSnapshot struct {
	Price       objects.ModelPrice
	ReferenceID string
	QuotedAt    time.Time
	// RequireOpenAIImageCacheRates preserves OpenAI's complete image/cache
	// validation after the channel price cache is reloaded or disabled.
	RequireOpenAIImageCacheRates bool
}

// RealtimePrice resolves and freezes configured modality rates for a
// channel/model pair. It reuses the enabled-channel price cache first and the
// shared model library fallback second. Missing rates are left unpriced;
// duplicates, invalid schedules and non-positive configured rates are rejected.
// No effective Realtime rates yields a nil snapshot and usage without a charge.
func (s *UsageLogService) RealtimePrice(ctx context.Context, channelID int, model string) (*RealtimePriceSnapshot, error) {
	return s.realtimePrice(ctx, channelID, model, objects.ValidateRealtimePrice, func(code objects.PriceItemCode) bool {
		return objects.IsRealtimePriceItemCode(code) && code != objects.PriceItemCodeRealtimeTranslationAudioSamples
	})
}

func (s *UsageLogService) realtimePrice(ctx context.Context, channelID int, model string, validate func(objects.ModelPrice) error, relevant func(objects.PriceItemCode) bool) (*RealtimePriceSnapshot, error) {
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("realtime price requires a model")
	}

	ch := s.ChannelService.GetEnabledChannel(channelID)
	if ch == nil {
		log.Warn(ctx, "channel not enabled for realtime price",
			log.Int("channel_id", channelID),
			log.String("model_id", model),
		)
	}

	var (
		price       objects.ModelPrice
		referenceID string
		found       bool
	)

	if ch != nil {
		if entity, ok := ch.cachedModelPrices[model]; ok && entity != nil {
			price, referenceID, found = entity.Price, entity.ReferenceID, true
		}
	}

	if !found {
		if libraryPrice, ok := s.ChannelService.resolveModelLibraryPrice(ctx, model); ok && libraryPrice != nil {
			price, referenceID, found = *libraryPrice, modelLibraryPriceReferenceID(model), true
		}
	}

	hasRate := func(items []objects.ModelPriceItem) bool {
		return slices.ContainsFunc(items, func(item objects.ModelPriceItem) bool { return relevant(item.ItemCode) })
	}
	if !found {
		return nil, nil
	}
	hasScheduledRate := price.Schedule != nil && slices.ContainsFunc(price.Schedule.Overrides, func(override objects.PriceOverride) bool {
		return hasRate(override.Items)
	})
	if !hasRate(price.Items) && !hasScheduledRate {
		return nil, nil
	}

	if err := validate(price); err != nil {
		return nil, fmt.Errorf("invalid realtime price for channel %d model %q: %w", channelID, model, err)
	}

	quotedAt := time.Now()
	frozen := cloneRealtimePrice(price)
	if frozen.Schedule != nil {
		if override := findMatchingOverride(quotedAt, frozen.Schedule); override != nil {
			frozen.Items = cloneModelPriceItems(override.Items)
		}
		frozen.Schedule = nil
	}
	if !hasRate(frozen.Items) {
		return nil, nil
	}
	return &RealtimePriceSnapshot{
		Price:       frozen,
		ReferenceID: referenceID,
		QuotedAt:    quotedAt,
	}, nil
}

// CreateRealtimeUsage persists confirmed realtime usage with the frozen
// snapshot. It never re-resolves the price, because a realtime channel can be
// disabled while its session is still running and re-resolution would lose the
// metering. Configured modality CostItems keep exact decimal subtotals; missing
// rates do not produce charges. The legacy total_cost float is set from the
// same exact decimal sum.
func (s *UsageLogService) CreateRealtimeUsage(
	ctx context.Context,
	params CreateUsageLogParams,
	snapshot *RealtimePriceSnapshot,
) (*ent.UsageLog, error) {
	if params.Usage == nil {
		return nil, fmt.Errorf("realtime usage is required")
	}

	if snapshot != nil && snapshot.Price.Schedule != nil {
		return nil, fmt.Errorf("realtime price snapshot must contain resolved prices")
	}
	if err := validateRealtimeUsage(params.Usage); err != nil {
		return nil, err
	}

	if snapshot == nil {
		return s.saveRealtimeUsage(ctx, params, nil, nil, decimal.Zero)
	}
	validatePrice := objects.ValidateRealtimePrice
	if snapshot.RequireOpenAIImageCacheRates {
		validatePrice = objects.ValidateOpenAIRealtimePrice
	}
	if err := validatePrice(snapshot.Price); err != nil {
		return nil, fmt.Errorf("invalid realtime price snapshot: %w", err)
	}

	costItems, total, err := computeRealtimeCostItems(params.Usage, snapshot.Price)
	if err != nil {
		return nil, err
	}

	return s.saveRealtimeUsage(ctx, params, snapshot, costItems, total)
}

func (s *UsageLogService) saveRealtimeUsage(ctx context.Context, params CreateUsageLogParams, snapshot *RealtimePriceSnapshot, costItems []objects.CostItem, total decimal.Decimal) (*ent.UsageLog, error) {
	client := s.entFromContext(ctx)

	mut := client.UsageLog.Create().
		SetRequestID(params.RequestID).
		SetProjectID(params.ProjectID).
		SetModelID(params.ActualModelID).
		SetChannelID(params.ChannelID).
		SetPromptTokens(params.Usage.PromptTokens).
		SetCompletionTokens(params.Usage.CompletionTokens).
		SetTotalTokens(params.Usage.TotalTokens).
		SetSource(params.Source).
		SetFormat(params.Format)

	if params.APIKeyID != nil {
		mut = mut.SetAPIKeyID(*params.APIKeyID)
	} else if ctxAPIKey, ok := contexts.GetAPIKey(ctx); ok && ctxAPIKey != nil {
		mut = mut.SetAPIKeyID(ctxAPIKey.ID)
	}

	if params.Usage.PromptTokensDetails != nil {
		mut = mut.
			SetPromptAudioTokens(params.Usage.PromptTokensDetails.AudioTokens).
			SetPromptCachedTokens(params.Usage.PromptTokensDetails.CachedTokens).
			SetPromptWriteCachedTokens(params.Usage.PromptTokensDetails.WriteCachedTokens).
			SetPromptWriteCachedTokens5m(params.Usage.PromptTokensDetails.WriteCached5MinTokens).
			SetPromptWriteCachedTokens1h(params.Usage.PromptTokensDetails.WriteCached1HourTokens)
	}

	if params.Usage.CompletionTokensDetails != nil {
		mut = mut.
			SetCompletionAudioTokens(params.Usage.CompletionTokensDetails.AudioTokens).
			SetCompletionReasoningTokens(params.Usage.CompletionTokensDetails.ReasoningTokens).
			SetCompletionAcceptedPredictionTokens(params.Usage.CompletionTokensDetails.AcceptedPredictionTokens).
			SetCompletionRejectedPredictionTokens(params.Usage.CompletionTokensDetails.RejectedPredictionTokens)
	}

	if snapshot != nil {
		mut.SetCostItems(costItems).SetTotalCost(total.InexactFloat64())
		if snapshot.ReferenceID != "" {
			mut.SetCostPriceReferenceID(snapshot.ReferenceID)
		}
	}

	usageLog, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create realtime usage log: %w", err)
	}

	if s.OnUsageLogCreated != nil {
		s.OnUsageLogCreated()
	}

	return usageLog, nil
}

// validateRealtimeUsage rejects usage that cannot be billed exactly: negative
// counters, an audio breakdown larger than its reported total, or missing usage.
// A missing breakdown is treated as zero audio and billed as text, which matches
// the provider semantics for text-only realtime turns.
func validateRealtimeUsage(usage *llm.Usage) error {
	if usage == nil {
		return fmt.Errorf("realtime usage is required")
	}

	if usage.PromptTokens < 0 || usage.CompletionTokens < 0 || usage.TotalTokens < 0 {
		return fmt.Errorf("realtime usage has negative token counts")
	}

	if usage.PromptTokens > usage.TotalTokens || usage.CompletionTokens != usage.TotalTokens-usage.PromptTokens {
		return fmt.Errorf("realtime total tokens do not reconcile")
	}
	if d := usage.Realtime; d != nil {
		if d.UnbilledOutputTextTokens < 0 || d.UnbilledOutputTextTokens > d.OutputTextTokens || (d.UnbilledOutputTextTokens > 0 && d.OutputAudioTokens == 0) {
			return fmt.Errorf("invalid non-billable transcript quantity")
		}
		if d.InputTextTokens < 0 || d.InputAudioTokens < 0 || d.InputImageTokens < 0 || d.CachedInputTextTokens < 0 || d.CachedInputAudioTokens < 0 || d.CachedInputImageTokens < 0 || d.OutputTextTokens < 0 || d.OutputAudioTokens < 0 {
			return fmt.Errorf("negative realtime modality usage")
		}
		if d.InputTextTokens > usage.PromptTokens || d.InputAudioTokens > usage.PromptTokens-d.InputTextTokens || d.InputImageTokens != usage.PromptTokens-d.InputTextTokens-d.InputAudioTokens || d.OutputTextTokens > usage.CompletionTokens || d.OutputAudioTokens != usage.CompletionTokens-d.OutputTextTokens {
			return fmt.Errorf("realtime modality usage does not reconcile")
		}
		if d.CachedInputTextTokens > d.InputTextTokens || d.CachedInputAudioTokens > d.InputAudioTokens || d.CachedInputImageTokens > d.InputImageTokens {
			return fmt.Errorf("realtime cache usage exceeds modality totals")
		}
	}
	if usage.PromptTokensDetails != nil {
		if usage.PromptTokensDetails.AudioTokens < 0 {
			return fmt.Errorf("realtime prompt audio tokens must not be negative")
		}

		if usage.PromptTokensDetails.AudioTokens > usage.PromptTokens {
			return fmt.Errorf("realtime prompt audio tokens %d exceed prompt tokens %d",
				usage.PromptTokensDetails.AudioTokens, usage.PromptTokens)
		}
	}

	if usage.CompletionTokensDetails != nil {
		if usage.CompletionTokensDetails.AudioTokens < 0 {
			return fmt.Errorf("realtime completion audio tokens must not be negative")
		}

		if usage.CompletionTokensDetails.AudioTokens > usage.CompletionTokens {
			return fmt.Errorf("realtime completion audio tokens %d exceed completion tokens %d",
				usage.CompletionTokensDetails.AudioTokens, usage.CompletionTokens)
		}
	}

	return nil
}

// computeRealtimeCostItems bills only configured Realtime codes, in audit
// order, and sums their exact decimal subtotals. Legacy codes never contribute,
// so a price carrying both sets cannot double-count the same tokens.
func computeRealtimeCostItems(usage *llm.Usage, price objects.ModelPrice) ([]objects.CostItem, decimal.Decimal, error) {
	computed, _ := ComputeRealtimeUsageCost(usage, price, time.Now())

	byCode := make(map[objects.PriceItemCode]objects.CostItem, len(objects.RealtimePriceItemCodes()))

	for _, item := range computed {
		if !objects.IsRealtimePriceItemCode(item.ItemCode) {
			continue
		}

		if _, duplicate := byCode[item.ItemCode]; duplicate {
			return nil, decimal.Zero, fmt.Errorf("duplicate realtime cost item %q", item.ItemCode)
		}

		byCode[item.ItemCode] = item
	}

	items := make([]objects.CostItem, 0, len(objects.RealtimePriceItemCodes()))
	total := decimal.Zero

	for _, code := range objects.AllRealtimePriceItemCodes() {
		item, ok := byCode[code]
		if !ok {
			continue
		}

		items = append(items, item)
		total = total.Add(item.Subtotal)
	}

	return items, total, nil
}

// cloneRealtimePrice deep-copies every slice and pointer a session might later
// mutate through the shared channel price cache.
func cloneRealtimePrice(price objects.ModelPrice) objects.ModelPrice {
	cloned := price
	cloned.Items = cloneModelPriceItems(price.Items)

	if price.Schedule != nil {
		schedule := *price.Schedule
		schedule.Overrides = make([]objects.PriceOverride, len(price.Schedule.Overrides))

		for i := range price.Schedule.Overrides {
			override := price.Schedule.Overrides[i]
			override.Items = cloneModelPriceItems(price.Schedule.Overrides[i].Items)
			override.When.Weekdays = slices.Clone(price.Schedule.Overrides[i].When.Weekdays)

			if price.Schedule.Overrides[i].When.DailyTime != nil {
				daily := *price.Schedule.Overrides[i].When.DailyTime
				override.When.DailyTime = &daily
			}

			if price.Schedule.Overrides[i].When.DateRange != nil {
				dateRange := *price.Schedule.Overrides[i].When.DateRange
				override.When.DateRange = &dateRange
			}

			schedule.Overrides[i] = override
		}

		cloned.Schedule = &schedule
	}

	return cloned
}

func cloneModelPriceItems(items []objects.ModelPriceItem) []objects.ModelPriceItem {
	cloned := slices.Clone(items)

	for i := range cloned {
		cloned[i].Pricing = clonePricing(items[i].Pricing)

		if items[i].PromptWriteCacheVariants != nil {
			variants := slices.Clone(items[i].PromptWriteCacheVariants)

			for j := range variants {
				variants[j].Pricing = clonePricing(items[i].PromptWriteCacheVariants[j].Pricing)
			}

			cloned[i].PromptWriteCacheVariants = variants
		}
	}

	return cloned
}

func clonePricing(pricing objects.Pricing) objects.Pricing {
	cloned := pricing

	if pricing.FlatFee != nil {
		value := *pricing.FlatFee
		cloned.FlatFee = &value
	}

	if pricing.UsagePerUnit != nil {
		value := *pricing.UsagePerUnit
		cloned.UsagePerUnit = &value
	}

	if pricing.UsageTiered != nil {
		tiered := *pricing.UsageTiered
		tiered.Tiers = slices.Clone(pricing.UsageTiered.Tiers)

		for i := range tiered.Tiers {
			if tiered.Tiers[i].UpTo != nil {
				upTo := *tiered.Tiers[i].UpTo
				tiered.Tiers[i].UpTo = &upTo
			}
		}

		cloned.UsageTiered = &tiered
	}

	return cloned
}
