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
	SystemService *SystemService
}

// NewUsageLogService creates a new UsageLogService.
func NewUsageLogService(systemService *SystemService) *UsageLogService {
	return &UsageLogService{
		SystemService: systemService,
	}
}

// CreateUsageLog creates a new usage log record from LLM response usage data.
func (s *UsageLogService) CreateUsageLog(
	ctx context.Context,
	userID int,
	requestID int,
	channelID *int,
	modelID string,
	usage *llm.Usage,
	source usagelog.Source,
	format string,
) (*ent.UsageLog, error) {
	if usage == nil {
		return nil, nil // No usage data to log
	}

	client := ent.FromContext(ctx)

	mut := client.UsageLog.Create().
		SetUserID(userID).
		SetRequestID(requestID).
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
		log.Int("user_id", userID),
		log.Int("request_id", requestID),
		log.String("model_id", modelID),
		log.Int("total_tokens", usage.TotalTokens),
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
		request.UserID,
		request.ID,
		channelID,
		request.ModelID,
		usage,
		usagelog.Source(request.Source),
		request.Format,
	)
}
