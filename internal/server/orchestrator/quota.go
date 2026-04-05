package orchestrator

import (
	"context"
	"net/http"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

func enforceQuota(inbound *PersistentInboundTransformer, quotaService *biz.QuotaService) pipeline.Middleware {
	return pipeline.OnLlmRequest("enforce-quota", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		if quotaService == nil {
			return llmRequest, nil
		}

		apiKey := inbound.state.APIKey
		if apiKey == nil {
			return llmRequest, nil
		}

		profile := apiKey.GetActiveProfile()
		if profile == nil || profile.Quota == nil {
			return llmRequest, nil
		}

		result, err := quotaService.CheckAPIKeyQuota(ctx, apiKey.ID, profile.Quota)
		if err != nil {
			return nil, err
		}

		if result.Allowed {
			return llmRequest, nil
		}

		requestID, _ := contexts.GetRequestID(ctx)
		fields := []log.Field{
			log.Int("api_key_id", apiKey.ID),
			log.String("profile_name", profile.Name),
			log.String("message", result.Message),
		}
		if result.Window.Start != nil {
			fields = append(fields, log.Time("window_start", *result.Window.Start))
		}
		if result.Window.End != nil {
			fields = append(fields, log.Time("window_end", *result.Window.End))
		}
		if requestID != "" {
			fields = append(fields, log.String("request_id", requestID))
		}
		log.Info(ctx, "api key quota exceeded", fields...)

		return nil, &llm.ResponseError{
			StatusCode: http.StatusForbidden,
			Detail: llm.ErrorDetail{
				Code:      "quota_exceeded",
				Message:   result.Message,
				Type:      "quota_exceeded_error",
				RequestID: requestID,
			},
		}
	})
}
