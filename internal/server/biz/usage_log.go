package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
)

// UsageLogService handles usage log operations.
type UsageLogService struct {
	*AbstractService

	SystemService *SystemService
}

// NewUsageLogService creates a new UsageLogService.
func NewUsageLogService(ent *ent.Client, systemService *SystemService) *UsageLogService {
	return &UsageLogService{
		AbstractService: &AbstractService{
			db: ent,
		},
		SystemService: systemService,
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
		if usage.PromptTokensDetails.AudioTokens > 0 {
			mut = mut.SetPromptAudioTokens(usage.PromptTokensDetails.AudioTokens)
		}

		if usage.PromptTokensDetails.CachedTokens > 0 {
			mut = mut.SetPromptCachedTokens(usage.PromptTokensDetails.CachedTokens)
		}

		if usage.PromptTokensDetails.WriteCachedTokens > 0 {
			mut = mut.SetPromptWriteCachedTokens(usage.PromptTokensDetails.WriteCachedTokens)
		}
	}

	// Set completion tokens details if available
	if usage.CompletionTokensDetails != nil {
		if usage.CompletionTokensDetails.AudioTokens > 0 {
			mut = mut.SetCompletionAudioTokens(usage.CompletionTokensDetails.AudioTokens)
		}

		if usage.CompletionTokensDetails.ReasoningTokens > 0 {
			mut = mut.SetCompletionReasoningTokens(usage.CompletionTokensDetails.ReasoningTokens)
		}

		if usage.CompletionTokensDetails.AcceptedPredictionTokens > 0 {
			mut = mut.SetCompletionAcceptedPredictionTokens(usage.CompletionTokensDetails.AcceptedPredictionTokens)
		}

		if usage.CompletionTokensDetails.RejectedPredictionTokens > 0 {
			mut = mut.SetCompletionRejectedPredictionTokens(usage.CompletionTokensDetails.RejectedPredictionTokens)
		}
	}

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
