// 职责: 原子写入实时计量预约及其会话租约，不受媒体存储策略影响。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//	internal/server/biz/realtime_accounting.go
//	internal/server/biz/request.go
//
// 注意事项:
//
//	此处只接收网关生成的审计元数据，不得传入用户对话、音频或凭证。

package biz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func (s *RequestService) CreateRealtimeRequest(ctx context.Context, model string, channelID int, sessionID string, metadata objects.JSONRawMessage) (*ent.Request, error) {
	key, ok := contexts.GetAPIKey(ctx)
	if !ok || key == nil {
		return nil, errors.New("realtime audit requires an API key")
	}
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, errors.New("realtime audit requires a project")
	}
	return s.entFromContext(ctx).Request.Create().SetProjectID(projectID).SetAPIKeyID(key.ID).SetModelID(model).SetChannelID(channelID).SetFormat(llm.APIFormatOpenAIRealtime.String()).SetStatus(request.StatusProcessing).SetSource(request.SourceAPI).SetStream(true).SetExternalID(sessionID).SetRequestBody(metadata).Save(ctx)
}

// UpdateRealtimeAudit persists gateway-generated accounting metadata independently
// of the policy for storing response content. Callers must never pass media or conversation payloads.
func (s *RequestService) UpdateRealtimeAudit(ctx context.Context, req *ent.Request, status request.Status, externalID string, metadata any, latency *LatencyMetrics, _ http.Header) error {
	if req.Format != llm.APIFormatOpenAIRealtime.String() {
		return errors.New("realtime audit requires a realtime request")
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	update := s.entFromContext(ctx).Request.Update().SetStatus(status).SetExternalID(externalID).SetResponseBody(data)
	if latency != nil {
		if latency.LatencyMs != nil {
			update.SetMetricsLatencyMs(*latency.LatencyMs)
		}
		if latency.FirstTokenLatencyMs != nil {
			update.SetMetricsFirstTokenLatencyMs(*latency.FirstTokenLatencyMs)
		}
	}
	if err := s.saveRequestUpdate(ctx, req.ID, update); err != nil {
		return err
	}
	s.recordChatRequestMetrics(ctx, req, status, latency)
	return nil
}
