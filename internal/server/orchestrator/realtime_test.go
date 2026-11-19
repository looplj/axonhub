// 职责: 验证实时请求必须选择显式实时端点，保留 HTTP 协议原有选择。
// 关联文件:
//
//	internal/server/orchestrator/select_endpoints.go
//	internal/server/orchestrator/realtime.go
package orchestrator

import (
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"testing"
)

func TestRealtimeNeverFallsBackToHTTP(t *testing.T) {
	httpEndpoints := []objects.ChannelEndpoint{{APIFormat: "openai/chat_completions"}, {APIFormat: "openai/responses-ws"}}
	require.Empty(t, SelectAPIFormat(httpEndpoints, &llm.Request{RequestType: llm.RequestTypeRealtime, APIFormat: llm.APIFormatOpenAIRealtime}))
	endpoints := append(httpEndpoints, objects.ChannelEndpoint{APIFormat: "qwen/realtime-ws"})
	require.Equal(t, "qwen/realtime-ws", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeRealtime}))
	require.Equal(t, "openai/chat_completions", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeChat}))
}

func TestRealtimeServiceDependencyInjection(t *testing.T) {
	var service *RealtimeService
	app := fx.New(fx.NopLogger, fx.Provide(NewRealtimeService, func() rt.Dialer { return rt.SecureDialer{} }), fx.Supply(&biz.AuthService{}, &DefaultSelector{}, &biz.ChannelService{}, &biz.QuotaService{}, &biz.UsageLogService{}, &biz.RequestService{}), fx.Populate(&service))
	require.NoError(t, app.Err())
	require.NotNil(t, service)
}

func TestRealtimeQwenMixedToolTranscriptPricingIsNotGuessed(t *testing.T) {
	target := &RealtimeTarget{APIFormat: llm.APIFormatQwenRealtimeWebSocket, Model: "qwen3.5-omni-flash-realtime"}
	usage := &llm.Usage{Realtime: &llm.RealtimeUsageDetails{OutputTextTokens: 10, OutputAudioTokens: 20}}
	require.Error(t, applyRealtimeBillingPolicy(target, usage, true))
	require.Zero(t, usage.Realtime.UnbilledOutputTextTokens)
	require.NoError(t, applyRealtimeBillingPolicy(target, usage, false))
	require.EqualValues(t, 10, usage.Realtime.UnbilledOutputTextTokens)
	target.Model = "qwen3.8-omni-flash-realtime"
	usage.Realtime.UnbilledOutputTextTokens = 0
	require.NoError(t, applyRealtimeBillingPolicy(target, usage, true))
	require.Zero(t, usage.Realtime.UnbilledOutputTextTokens)
}
