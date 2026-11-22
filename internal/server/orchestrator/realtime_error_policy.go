// 职责: 在原生事件适配器之外显式执行 Realtime 下游错误信息安全策略。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//	llm/transformer/openai/realtime/native.go
//
// 注意事项:
//
//	正常 GA 消息不改写；仅错误负载在网关安全边界脱敏，不承诺错误帧端到端原文转发。

package orchestrator

import (
	"encoding/json"

	protocol "github.com/looplj/axonhub/llm/realtime"
)

func applyRealtimeErrorPolicy(data []byte) ([]byte, error) {
	var event map[string]json.RawMessage
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	var kind string
	_ = json.Unmarshal(event["type"], &kind)
	if kind == "error" {
		var upstream struct {
			EventID string `json:"event_id"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(event["error"], &upstream)
		safe := map[string]string{"type": "invalid_request_error", "code": "upstream_error", "message": "The upstream Realtime service rejected an event."}
		if protocol.SafeParameterDiagnostic(upstream.Code, upstream.Message) {
			safe["code"] = upstream.Code
			safe["message"] = upstream.Message
		}
		if upstream.EventID != "" && len(upstream.EventID) <= 256 {
			safe["event_id"] = upstream.EventID
		}
		event["error"], _ = json.Marshal(safe)
		envelope := map[string]json.RawMessage{"type": event["type"], "error": event["error"]}
		var eventID string
		if json.Unmarshal(event["event_id"], &eventID) == nil && eventID != "" && len(eventID) <= 256 {
			envelope["event_id"], _ = json.Marshal(eventID)
		}
		return json.Marshal(envelope)
	}
	if kind == "response.done" {
		var response map[string]json.RawMessage
		_ = json.Unmarshal(event["response"], &response)
		var details map[string]json.RawMessage
		_ = json.Unmarshal(response["status_details"], &details)
		if _, ok := details["error"]; ok {
			details["error"] = json.RawMessage(`{"type":"server_error","code":"upstream_error","message":"The upstream Realtime response failed."}`)
			response["status_details"], _ = json.Marshal(details)
			event["response"], _ = json.Marshal(response)
			return json.Marshal(event)
		}
	}
	return data, nil
}
