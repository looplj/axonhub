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

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func (s *RequestService) CreateRealtimeRequest(ctx context.Context, model string, channelID int, sessionID string, metadata objects.JSONRawMessage, userAgent ...string) (*ent.Request, error) {
	key, ok := contexts.GetAPIKey(ctx)
	if !ok || key == nil {
		return nil, errors.New("realtime audit requires an API key")
	}
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, errors.New("realtime audit requires a project")
	}
	create := s.entFromContext(ctx).Request.Create().SetProjectID(projectID).SetAPIKeyID(key.ID).SetModelID(model).SetChannelID(channelID).SetFormat(llm.APIFormatOpenAIRealtime.String()).SetStatus(request.StatusProcessing).SetSource(request.SourceAPI).SetStream(true).SetExternalID(sessionID).SetRequestBody(metadata)
	if len(userAgent) > 0 {
		create.SetUserAgent(userAgent[0])
	}
	return create.Save(ctx)
}

// RecordRealtimeConnectionFailure stores an admission or upstream handshake
// failure as an audit row without creating usage or changing quota accounting.
func (s *RequestService) RecordRealtimeConnectionFailure(
	ctx context.Context,
	model string,
	channelID int,
	sessionID string,
	requestMetadata map[string]any,
	errorCode string,
	message string,
	latencyMs int64,
	userAgent string,
) error {
	key, ok := contexts.GetAPIKey(ctx)
	if !ok || key == nil {
		return errors.New("realtime audit requires an API key")
	}
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return errors.New("realtime audit requires a project")
	}
	requestBody, err := json.Marshal(requestMetadata)
	if err != nil {
		return err
	}
	responseBody, err := json.Marshal(map[string]any{
		"error": map[string]string{"error_code": errorCode, "message": message},
	})
	if err != nil {
		return err
	}
	req, err := s.entFromContext(ctx).Request.Create().
		SetProjectID(projectID).
		SetAPIKeyID(key.ID).
		SetModelID(model).
		SetChannelID(channelID).
		SetFormat(llm.APIFormatOpenAIRealtime.String()).
		SetStatus(request.StatusFailed).
		SetSource(request.SourceAPI).
		SetStream(true).
		SetExternalID(sessionID).
		SetRequestBody(requestBody).
		SetResponseBody(responseBody).
		SetMetricsLatencyMs(latencyMs).
		SetUserAgent(userAgent).
		Save(ctx)
	if err != nil {
		return err
	}
	metricRequest := *req
	metricRequest.Status = request.StatusProcessing
	s.recordChatRequestMetrics(ctx, &metricRequest, request.StatusFailed, &LatencyMetrics{LatencyMs: &latencyMs})
	return nil
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

// UpdateRealtimeSessionMetadata adds bounded session diagnostics after accounting.
func (s *RequestService) UpdateRealtimeSessionMetadata(ctx context.Context, ids []int, metadata any, durationMs int64) error {
	realtime, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	client := s.entFromContext(ctx)
	rows, err := authz.RunAsHost(ctx, client, "realtime-session-audit-metadata", func(hostCtx context.Context) ([]*ent.Request, error) {
		return client.Request.Query().
			Where(request.IDIn(ids...), request.FormatEQ(llm.APIFormatOpenAIRealtime.String()), request.RequestBodyChunkedEQ(false)).
			Select(request.FieldID, request.FieldStatus, request.FieldRequestBody, request.FieldMetricsLatencyMs).All(hostCtx)
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		body := make(map[string]json.RawMessage)
		if len(row.RequestBody) != 0 {
			if err := json.Unmarshal(row.RequestBody, &body); err != nil {
				return err
			}
		}
		if body == nil {
			body = make(map[string]json.RawMessage)
		}
		body["realtime"] = realtime
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		// Request bodies are immutable to ordinary updates. Realtime rows
		// contain gateway metadata only and can be enriched after the session.
		update := client.Request.Update().Where(request.FormatEQ(llm.APIFormatOpenAIRealtime.String()), request.RequestBodyChunkedEQ(false)).
			Modify(func(builder *entsql.UpdateBuilder) {
				builder.Set(request.FieldRequestBody, json.RawMessage(data))
			})
		if row.Status == request.StatusFailed && row.MetricsLatencyMs == nil {
			update.SetMetricsLatencyMs(durationMs)
		}
		if err := s.saveRequestUpdate(ctx, row.ID, update); err != nil {
			return err
		}
	}
	return nil
}
