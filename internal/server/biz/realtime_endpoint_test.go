// 职责: 验证渠道级实时配置与通用端点、旧 HTTP 地址严格隔离。
// 关联文件:
//
//	internal/server/biz/channel_endpoint.go
//	internal/server/biz/channel_realtime.go
//	internal/server/biz/channel_llm.go
package biz

import (
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRealtimeEndpointExplicitAndTrusted(t *testing.T) {
	ep := objects.ChannelEndpoint{APIFormat: "qwen/realtime-ws", BaseURL: "wss://workspace.cn-beijing.maas.aliyuncs.com", Path: "/api-ws/v1/realtime"}
	require.Error(t, validateEndpointsForChannelType(channel.TypeBailian, []objects.ChannelEndpoint{ep}, "https://dashscope.aliyuncs.com/compatible-mode/v1"), "Realtime must not be accepted by the endpoint form")
	require.Error(t, validateEndpointsForChannelType(channel.TypeOpenai, []objects.ChannelEndpoint{ep}))
	cfg := &objects.ChannelRealtimeConfig{Enabled: true, Protocol: ep.APIFormat, BaseURL: ep.BaseURL, Path: ep.Path}
	require.NoError(t, ValidateChannelRealtime(channel.TypeBailian, cfg))
	require.Error(t, ValidateChannelRealtime(channel.TypeOpenai, cfg), "provider isolation must remain enforced at the channel layer")
	for _, bad := range []string{"ws://workspace.cn-beijing.maas.aliyuncs.com", "wss://127.0.0.1", "wss://workspace.cn-beijing.maas.aliyuncs.com.evil.test", "https://dashscope.aliyuncs.com"} {
		ep.BaseURL = bad
		require.Error(t, ValidateEndpoints([]objects.ChannelEndpoint{ep}))
		cfg.BaseURL = bad
		require.Error(t, ValidateChannelRealtime(channel.TypeBailian, cfg), "trusted host validation must survive moving configuration off endpoints")
	}
	for _, ep := range DefaultEndpointsForChannelType(channel.TypeBailian) {
		require.NotEqual(t, llm.APIFormatQwenRealtimeWebSocket.String(), ep.APIFormat)
	}
}

func TestRealtimeEndpointHandshakeTimeoutBounds(t *testing.T) {
	ep := objects.ChannelEndpoint{APIFormat: "qwen/realtime-ws", BaseURL: "wss://workspace.cn-beijing.maas.aliyuncs.com", Path: "/api-ws/v1/realtime", HandshakeTimeoutMs: lo.ToPtr(int64(60000))}
	require.Error(t, ValidateEndpoints([]objects.ChannelEndpoint{ep}))
	cfg := &objects.ChannelRealtimeConfig{Enabled: true, Protocol: ep.APIFormat, BaseURL: ep.BaseURL, Path: ep.Path, HandshakeTimeoutMs: ep.HandshakeTimeoutMs}
	require.NoError(t, ValidateChannelRealtime(channel.TypeBailian, cfg))
	for _, timeout := range []int64{-1, 0, 999, 60001} {
		ep.HandshakeTimeoutMs = lo.ToPtr(timeout)
		require.Error(t, ValidateEndpoints([]objects.ChannelEndpoint{ep}))
		cfg.HandshakeTimeoutMs = lo.ToPtr(timeout)
		require.Error(t, ValidateChannelRealtime(channel.TypeBailian, cfg))
	}
}
