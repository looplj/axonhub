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

import "encoding/json"

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
		}
		_ = json.Unmarshal(event["error"], &upstream)
		safe := map[string]string{"type": "invalid_request_error", "code": "upstream_error", "message": "The upstream Realtime service rejected an event."}
		if upstream.EventID != "" {
			safe["event_id"] = upstream.EventID
		}
		event["error"], _ = json.Marshal(safe)
		return json.Marshal(event)
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
