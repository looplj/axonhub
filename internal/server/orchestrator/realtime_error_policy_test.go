// 职责: 验证网关错误脱敏策略的明确例外及正常 GA 消息的保真性。
// 关联文件:
//
//	internal/server/orchestrator/realtime_error_policy.go
//	internal/server/orchestrator/realtime.go

package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRealtimeExplicitErrorBoundaryPreservesNormalBytes(t *testing.T) {
	raw := []byte(`{ "type":"future.event", "extension":{"order":1} }`)
	actual, err := applyRealtimeErrorPolicy(raw)
	require.NoError(t, err)
	require.Equal(t, raw, actual)
	for _, raw := range []string{`{"type":"error","event_id":"e1","error":{"message":"synthetic-sensitive-provider-detail"}}`, `{"type":"response.done","response":{"id":"r1","status_details":{"error":{"message":"synthetic-sensitive-provider-detail"}}}}`} {
		actual, err := applyRealtimeErrorPolicy([]byte(raw))
		require.NoError(t, err)
		require.NotContains(t, string(actual), "synthetic-sensitive-provider-detail")
		require.Contains(t, string(actual), "upstream_error")
	}
}

func TestRealtimeErrorRedactionRetainsRejectedEventID(t *testing.T) {
	raw := []byte(`{"type":"error","event_id":"server-event","error":{"event_id":"client-event","message":"private","param":"private"}}`)
	actual, err := applyRealtimeErrorPolicy(raw)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"error","event_id":"server-event","error":{"event_id":"client-event","type":"invalid_request_error","code":"upstream_error","message":"The upstream Realtime service rejected an event."}}`, string(actual))
}

func TestRealtimeErrorPolicyForwardsSafeParameterDiagnostic(t *testing.T) {
	actual, err := applyRealtimeErrorPolicy([]byte(`{"type":"error","request":"sensitive","error":{"code":"unknown_parameter","message":"Unknown parameter: 'session.audio.input.unknown_field'.","audio":"sensitive"}}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"error","error":{"type":"invalid_request_error","code":"unknown_parameter","message":"Unknown parameter: 'session.audio.input.unknown_field'."}}`, string(actual))
}
