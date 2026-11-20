// 职责: 验证实时协议登记保留控制字段且不改变既有插件钩子契约。
// 关联文件:
//
//	internal/server/plugin/protected_fields.go
//	llm/constants.go

package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRealtimeControlEnvelopesRemainHostOwned(t *testing.T) {
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIRealtime, llm.APIFormatOpenAIRealtimeWebSocket, llm.APIFormatQwenRealtimeWebSocket} {
		original := []byte(`{"type":"session.update","session":{"type":"realtime"}}`)
		request := &httpclient.Request{APIFormat: format.String(), Body: original}
		require.Empty(t, protectedAttemptFields(request, original, "voice"))
		require.Contains(t, protectedAttemptFields(request, []byte(`{"type":"session.update","session":{"type":"transcription"}}`), "voice"), "session")
		require.Contains(t, protectedAttemptFields(request, []byte(`{"type":"response.create","session":{"type":"realtime"}}`), "voice"), "type")
	}
}
