// 职责: 验证网关错误脱敏策略的明确例外及正常 GA 消息的保真性。
// 关联文件:
//
//	internal/server/orchestrator/realtime_error_policy.go
//	internal/server/orchestrator/realtime.go
package orchestrator

import (
	"github.com/stretchr/testify/require"
	"testing"
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
