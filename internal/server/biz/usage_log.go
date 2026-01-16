package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/privacy"
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

func (s *UsageLogService) computeUsageCost(ctx context.Context, channelID int, modelID string, usage *llm.Usage) ([]objects.CostItem, float64) {
	if s.ChannelService == nil || usage == nil {
		return nil, 0
	}

	ch := s.ChannelService.GetEnabledChannel(channelID)
	if ch == nil {
		log.Warn(ctx, "channel not enabled for cost calculation",
			log.Int("channel_id", channelID),
			log.String("model_id", modelID),
		)

		return nil, 0
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "checking cached model price",
			log.Int("channel_id", channelID),
			log.String("model_id", modelID),
			log.Int("cached_price_count", len(ch.cachedModelPrices)),
		)
	}

	if price, ok := ch.cachedModelPrices[modelID]; ok {
		items, total := ComputeUsageCost(usage, price)

		totalCost := total.InexactFloat64()
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "computed usage cost from cache",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Float64("total_cost", totalCost),
				log.Int64("total_tokens", usage.TotalTokens),
			)
		}

		return items, totalCost
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "cached price not found, querying database",
			log.Int("channel_id", channelID),
			log.String("model_id", modelID),
		)
	}

	client := s.entFromContext(ctx)
	dbCtx := privacy.DecisionContext(ctx, privacy.Allow)

	p, err := client.ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(channelID),
			channelmodelprice.ModelIDEQ(modelID),
			channelmodelprice.DeletedAtEQ(0),
		).
		First(dbCtx)
	if err != nil {
		log.Warn(ctx, "model price not found in database",
			log.Int("channel_id", channelID),
			log.String("model_id", modelID),
			log.Cause(err),
		)

		return nil, 0
	}

	items, total := ComputeUsageCost(usage, p.Price)
	totalCost := total.InexactFloat64()

	if ch.cachedModelPrices == nil {
		ch.cachedModelPrices = make(map[string]objects.ModelPrice)
	}

	ch.cachedModelPrices[modelID] = p.Price

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "computed usage cost from db and refreshed cache",
			log.Int("channel_id", channelID),
			log.String("model_id", modelID),
			log.Float64("total_cost", totalCost),
			log.Int("cached_price_count", len(ch.cachedModelPrices)),
		)
	}

	return items, totalCost
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
			SetPromptWriteCachedTokens(usage.PromptTokensDetails.WriteCachedTokens)
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
		totalCost float64
		costItems []objects.CostItem
	)

	if channelID != nil && s.ChannelService != nil {
		costItems, totalCost = s.computeUsageCost(ctx, *channelID, modelID, usage)
	}

	mut = mut.SetTotalCost(totalCost).SetCostItems(costItems)

	usageLog, err := mut.Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to create usage log", log.Cause(err))
		return nil, err
	}

	log.Debug(ctx, "Created usage log",
		log.Int("usage_log_id", usageLog.ID),
		log.Int("request_id", requestID),
		log.String("model_id", modelID),
		log.Int64("total_tokens", usage.TotalTokens),
	)

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
