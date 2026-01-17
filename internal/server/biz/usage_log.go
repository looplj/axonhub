package biz

import (
	"context"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// UsageLogService handles usage log operations.
type UsageLogService struct {
	*AbstractService

	SystemService  *SystemService
	ChannelService *ChannelService
}

func (s *UsageLogService) computeUsageCost(ctx context.Context, channelID int, modelID string, usage *llm.Usage) ([]objects.CostItem, *float64, string) {
	if usage == nil {
		return nil, nil, ""
	}

	ch := s.ChannelService.GetEnabledChannel(channelID)
	if ch == nil {
		log.Warn(ctx, "channel not enabled for cost calculation",
			log.Int("channel_id", channelID),
			log.String("model_id", modelID),
		)

		return nil, nil, ""
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "checking cached model price",
			log.Int("channel_id", channelID),
			log.String("model_id", modelID),
			log.Int("cached_price_count", len(ch.cachedModelPrices)),
		)
	}

	if modelPrice, ok := ch.cachedModelPrices[modelID]; ok {
		items, total := ComputeUsageCost(usage, modelPrice.Price)

		totalCost := total.InexactFloat64()
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "computed usage cost from cache",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Float64("total_cost", totalCost),
				log.Int64("total_tokens", usage.TotalTokens),
				log.String("price_reference_id", modelPrice.RefreanceID),
			)
		}

		return items, lo.ToPtr(totalCost), modelPrice.RefreanceID
	}

	return nil, nil, ""
}

// NewUsageLogService creates a new UsageLogService.
func NewUsageLogService(ent *ent.Client, systemService *SystemService, channelService *ChannelService) *UsageLogService {
	return &UsageLogService{
		AbstractService: &AbstractService{
			db: ent,
		},
		SystemService:  systemService,
		ChannelService: channelService,
	}
}

// CreateUsageLog creates a new usage log record from LLM response usage data.
func (s *UsageLogService) CreateUsageLog(
	ctx context.Context,
	requestID int,
	projectID int,
	channelID *int,
	modelID string,
	usage *llm.Usage,
	source usagelog.Source,
	format string,
) (*ent.UsageLog, error) {
	if usage == nil {
		return nil, nil // No usage data to log
	}

	client := s.entFromContext(ctx)

	mut := client.UsageLog.Create().
		SetRequestID(requestID).
		SetProjectID(projectID).
		SetModelID(modelID).
		SetPromptTokens(usage.PromptTokens).
		SetCompletionTokens(usage.CompletionTokens).
		SetTotalTokens(usage.TotalTokens).
		SetSource(source).
		SetFormat(format)

	// Set channel ID if provided
	if channelID != nil {
		mut = mut.SetChannelID(*channelID)
	}

	// Set prompt tokens details if available
	if usage.PromptTokensDetails != nil {
		mut = mut.
			SetPromptAudioTokens(usage.PromptTokensDetails.AudioTokens).
			SetPromptCachedTokens(usage.PromptTokensDetails.CachedTokens).
			SetPromptWriteCachedTokens(usage.PromptTokensDetails.WriteCachedTokens).
			SetPromptWriteCachedTokens5m(usage.PromptTokensDetails.WriteCached5MinTokens).
			SetPromptWriteCachedTokens1h(usage.PromptTokensDetails.WriteCached1HourTokens)
	}

	// Set completion tokens details if available
	if usage.CompletionTokensDetails != nil {
		mut = mut.
			SetCompletionAudioTokens(usage.CompletionTokensDetails.AudioTokens).
			SetCompletionReasoningTokens(usage.CompletionTokensDetails.ReasoningTokens).
			SetCompletionAcceptedPredictionTokens(usage.CompletionTokensDetails.AcceptedPredictionTokens).
			SetCompletionRejectedPredictionTokens(usage.CompletionTokensDetails.RejectedPredictionTokens)
	}

	// Calculate cost if price is configured
	var (
		totalCost        *float64
		costItems        []objects.CostItem
		priceReferenceID string
	)

	if channelID != nil {
		costItems, totalCost, priceReferenceID = s.computeUsageCost(ctx, *channelID, modelID, usage)
	}

	mut = mut.
		SetNillableTotalCost(totalCost).
		SetCostItems(costItems)

	if priceReferenceID != "" {
		mut = mut.SetCostPriceReferenceID(priceReferenceID)
	}

	usageLog, err := mut.Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to create usage log", log.Cause(err))
		return nil, err
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "Created usage log",
			log.Int("usage_log_id", usageLog.ID),
			log.Int("request_id", requestID),
			log.String("model_id", modelID),
			log.Int64("total_tokens", usage.TotalTokens),
		)
	}

	return usageLog, nil
}

// CreateUsageLogFromRequest creates a usage log from request and response data.
func (s *UsageLogService) CreateUsageLogFromRequest(
	ctx context.Context,
	request *ent.Request,
	requestExec *ent.RequestExecution,
	usage *llm.Usage,
) (*ent.UsageLog, error) {
	if request == nil || usage == nil {
		return nil, nil
	}

	// Get channel ID from request if available
	var channelID *int
	if request.ChannelID != 0 {
		channelID = &request.ChannelID
	}

	if channelID == nil {
		channelID = &requestExec.ChannelID
	}

	return s.CreateUsageLog(
		ctx,
		request.ID,
		request.ProjectID,
		channelID,
		request.ModelID,
		usage,
		usagelog.Source(request.Source),
		request.Format,
	)
}
