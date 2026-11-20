// 职责: 通过真实 WebSocket 建连、配置确认和轻量会话事件测试渠道端点。
// 关联文件:
//
//	internal/server/orchestrator/tester.go
//	internal/server/orchestrator/realtime.go
//
// 注意事项:
//
//	测试成功不代表语音质量或真实用量验收通过；此探测不触发模型生成。

package orchestrator

import (
	"context"
	"errors"
	"time"

	"github.com/gorilla/websocket"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	protocol "github.com/looplj/axonhub/llm/realtime"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

func (p *TestChannelOrchestrator) testRealtimeChannel(ctx context.Context, ch *biz.Channel, model string, proxy *httpclient.ProxyConfig) (*TestChannelResult, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	fail := func() (*TestChannelResult, error) {
		return &TestChannelResult{Latency: time.Since(start).Seconds(), Success: false, Error: lo.ToPtr("Realtime handshake, configuration or conversation acknowledgement failed")}, nil
	}
	entry, ok := ch.GetModelEntries()[model]
	if !ok {
		return nil, biz.ErrInvalidModel
	}
	for _, ep := range ch.ResolveEndpoints() {
		if ep.APIFormat != string(channelTestAPIFormat(ch, model)) {
			continue
		}
		endpointURL := protocol.EndpointURL
		if ep.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket.String() {
			endpointURL = openairt.EndpointURL
		}
		target, err := endpointURL(ep.BaseURL, ep.Path, entry.ActualModel)
		if err != nil {
			return fail()
		}
		key, err := ch.RealtimeAPIKey(ctx)
		if err != nil {
			return fail()
		}
		if proxy == nil && ch.Settings != nil {
			proxy = ch.Settings.Proxy
		}
		conn, err := rt.Dial(ctx, target, key, proxy, 15*time.Second)
		if err != nil {
			return fail()
		}
		defer conn.Close()
		service := &RealtimeService{Channels: p.channelService, Quota: nil, Usage: p.usageLogService, Requests: p.requestService}
		conversation, _, err := service.configure(ctx, conn, &RealtimeTarget{Channel: ch, Model: entry.ActualModel, ClientModel: model, APIFormat: llm.APIFormat(ep.APIFormat)}, rt.Config{}.Defaults())
		if err != nil {
			return fail()
		}
		defer conversation.Close()
		if ep.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket.String() {
			configEvent := []byte(`{"type":"session.update","session":{"type":"realtime","output_modalities":["text"],"audio":{"input":{"turn_detection":null}}}}`)
			wire, err := conversation.adapter.Client(configEvent)
			if err != nil {
				return fail()
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, wire); err != nil {
				return fail()
			}
			_, ack, err := conn.ReadMessage()
			if err != nil || gjson.GetBytes(ack, "type").String() != "session.updated" {
				return fail()
			}
		}
		event, err := conversation.adapter.Client([]byte(`{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"ping"}]}}`))
		if err != nil {
			return fail()
		}
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, event); err != nil {
			return fail()
		}
		for range 8 {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return fail()
			}
			switch gjson.GetBytes(data, "type").String() {
			case "error":
				return fail()
			case "conversation.item.created":
				return &TestChannelResult{Latency: time.Since(start).Seconds(), Success: true, Message: lo.ToPtr("Realtime WebSocket handshake, PCM configuration and conversation event acknowledged")}, nil
			}
		}
		return fail()
	}
	return nil, errors.New("Realtime endpoint is not explicitly configured")
}
