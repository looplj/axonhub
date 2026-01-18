package biz

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/contexts"
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
				log.String("price_reference_id", modelPrice.ReferenceID),
			)
		}

		return items, lo.ToPtr(totalCost), modelPrice.ReferenceID
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

// CreateUsageLogParams represents the parameters for creating a usage log.
type CreateUsageLogParams struct {
	RequestID     int
	ProjectID     int
	ChannelID     *int
	ActualModelID string // The channel actual model ID, not the request model ID.
	Usage         *llm.Usage
	Source        usagelog.Source
	Format        string
	APIKeyID      *int
}

// CreateUsageLog creates a new usage log record from LLM response usage data.
func (s *UsageLogService) CreateUsageLog(ctx context.Context, params CreateUsageLogParams) (*ent.UsageLog, error) {
	if params.Usage == nil {
		return nil, nil // No usage data to log
	}

	client := s.entFromContext(ctx)

	mut := client.UsageLog.Create().
		SetRequestID(params.RequestID).
		SetProjectID(params.ProjectID).
		SetModelID(params.ActualModelID).
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

	// Set channel ID if provided
	if params.ChannelID != nil {
		mut = mut.SetChannelID(*params.ChannelID)
	}

	// Set prompt tokens details if available
	if params.Usage.PromptTokensDetails != nil {
		mut = mut.
			SetPromptAudioTokens(params.Usage.PromptTokensDetails.AudioTokens).
			SetPromptCachedTokens(params.Usage.PromptTokensDetails.CachedTokens).
			SetPromptWriteCachedTokens(params.Usage.PromptTokensDetails.WriteCachedTokens).
			SetPromptWriteCachedTokens5m(params.Usage.PromptTokensDetails.WriteCached5MinTokens).
			SetPromptWriteCachedTokens1h(params.Usage.PromptTokensDetails.WriteCached1HourTokens)
	}

	// Set completion tokens details if available
	if params.Usage.CompletionTokensDetails != nil {
		mut = mut.
			SetCompletionAudioTokens(params.Usage.CompletionTokensDetails.AudioTokens).
			SetCompletionReasoningTokens(params.Usage.CompletionTokensDetails.ReasoningTokens).
			SetCompletionAcceptedPredictionTokens(params.Usage.CompletionTokensDetails.AcceptedPredictionTokens).
			SetCompletionRejectedPredictionTokens(params.Usage.CompletionTokensDetails.RejectedPredictionTokens)
	}

	// Calculate cost if price is configured
	var (
		totalCost        *float64
		costItems        []objects.CostItem
		priceReferenceID string
	)

	if params.ChannelID != nil {
		costItems, totalCost, priceReferenceID = s.computeUsageCost(ctx, *params.ChannelID, params.ActualModelID, params.Usage)
	}

	mut = mut.
		SetNillableTotalCost(totalCost).
		SetCostItems(costItems)

	if priceReferenceID != "" {
		mut = mut.SetCostPriceReferenceID(priceReferenceID)
	}

	usageLog, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create usage log: %w", err)
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "Created usage log",
			log.Int("usage_log_id", usageLog.ID),
			log.Int("request_id", params.RequestID),
			log.String("model_id", params.ActualModelID),
			log.Int64("total_tokens", params.Usage.TotalTokens),
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

	return s.CreateUsageLog(ctx, CreateUsageLogParams{
		RequestID:     request.ID,
		ProjectID:     request.ProjectID,
		ChannelID:     channelID,
		ActualModelID: requestExec.ModelID,
		Usage:         usage,
		Source:        usagelog.Source(request.Source),
		Format:        request.Format,
		APIKeyID:      lo.ToPtr(request.APIKeyID),
	})
}
