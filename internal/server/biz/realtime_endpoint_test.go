// 职责: 验证渠道级实时配置与通用端点、旧 HTTP 地址严格隔离。
// 关联文件:
//
//	internal/server/biz/channel_endpoint.go
//	internal/server/biz/channel_realtime.go
//	internal/server/biz/channel_llm.go

package biz

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestRealtimeEndpointExplicitAndTrusted(t *testing.T) {
	ep := objects.ChannelEndpoint{APIFormat: "qwen/realtime-ws", BaseURL: "wss://workspace.cn-beijing.maas.aliyuncs.com", Path: "/api-ws/v1/realtime"}
	require.Error(t, validateEndpointsForChannelType(channel.TypeBailian, []objects.ChannelEndpoint{ep}, "https://dashscope.aliyuncs.com/compatible-mode/v1"), "Realtime must not be accepted by the endpoint form")
	require.Error(t, validateEndpointsForChannelType(channel.TypeOpenai, []objects.ChannelEndpoint{ep}))
	cfg := &objects.ChannelRealtimeConfig{Enabled: true, Protocol: ep.APIFormat, BaseURL: ep.BaseURL, Path: ep.Path}
	require.NoError(t, ValidateChannelRealtime(channel.TypeBailian, cfg))
	require.Error(t, ValidateChannelRealtime(channel.TypeOpenai, cfg), "provider isolation must remain enforced at the channel layer")
	cfg.BaseURL = "ws://workspace.cn-beijing.maas.aliyuncs.com"
	require.Error(t, ValidateChannelRealtime(channel.TypeBailian, cfg), "Realtime Base URLs must use wss")
	openai := &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://proxy.example.test", Path: "/v1/realtime"}
	require.NoError(t, ValidateChannelRealtime(channel.TypeOpenai, openai), "OpenAI Realtime accepts a configured WebSocket host")
	openai.BaseURL = "wss://127.0.0.1"
	require.Error(t, ValidateChannelRealtime(channel.TypeOpenai, openai), "IP literal targets remain disallowed")
	openai.BaseURL = "https://proxy.example.test"
	require.Error(t, ValidateChannelRealtime(channel.TypeOpenai, openai), "non-WebSocket schemes must be rejected")
	for _, bad := range []string{"wss://127.0.0.1", "wss://workspace.cn-beijing.maas.aliyuncs.com.evil.test", "https://dashscope.aliyuncs.com"} {
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

func TestRealtimeClientEventsPerSecondBounds(t *testing.T) {
	var cfg objects.ChannelRealtimeConfig
	require.NoError(t, json.Unmarshal([]byte(`{"enabled":true,"protocol":"openai/realtime-ws","baseURL":"wss://api.openai.com","path":"/v1/realtime"}`), &cfg))
	require.NoError(t, ValidateChannelRealtime(channel.TypeOpenai, &cfg), "an omitted override inherits the server default")

	for _, limit := range []int64{256, 1024, 4096} {
		payload, err := json.Marshal(map[string]any{"enabled": true, "protocol": "openai/realtime-ws", "baseURL": "wss://api.openai.com", "path": "/v1/realtime", "clientEventsPerSecond": limit})
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(payload, &cfg))
		require.NoError(t, ValidateChannelRealtime(channel.TypeOpenai, &cfg))
		encoded, err := json.Marshal(cfg)
		require.NoError(t, err)
		require.Equal(t, limit, gjson.GetBytes(encoded, "clientEventsPerSecond").Int(), "the configured event limit must survive channel settings persistence")
	}
	for _, limit := range []int64{100, 255, 4097, 5000} {
		payload, err := json.Marshal(map[string]any{"enabled": true, "protocol": "openai/realtime-ws", "baseURL": "wss://api.openai.com", "path": "/v1/realtime", "clientEventsPerSecond": limit})
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(payload, &cfg))
		require.ErrorContains(t, ValidateChannelRealtime(channel.TypeOpenai, &cfg), "256 and 4096")
	}
}
