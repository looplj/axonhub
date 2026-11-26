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

	"github.com/coder/websocket"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	protocol "github.com/looplj/axonhub/llm/realtime"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

func (p *TestChannelOrchestrator) realtimeProbeConfig(ep objects.ChannelEndpoint) rt.Config {
	config := p.realtimeConfig.Defaults()
	if ep.HandshakeTimeoutMs != nil && *ep.HandshakeTimeoutMs > 0 {
		config.HandshakeTimeout = time.Duration(*ep.HandshakeTimeoutMs) * time.Millisecond
	}
	return config
}

func (p *TestChannelOrchestrator) testRealtimeChannel(ctx context.Context, ch *biz.Channel, model string, proxy *httpclient.ProxyConfig) (*TestChannelResult, error) {
	start := time.Now()
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
		translation := ep.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket.String() && entry.ActualModel == openairt.TranslationModel
		endpointURL := protocol.EndpointURL
		if ep.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket.String() {
			endpointURL = openairt.EndpointURL
		}
		if translation {
			endpointURL = openairt.TranslationEndpointURL
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
		config := p.realtimeProbeConfig(ep)
		dialCtx, cancel := context.WithTimeout(ctx, config.HandshakeTimeout)
		defer cancel()
		conn, err := rt.Dial(dialCtx, target, key, proxy, config.HandshakeTimeout)
		if err != nil {
			return fail()
		}
		defer func() { _ = conn.CloseNow() }()
		service := &RealtimeService{Channels: p.channelService, Quota: nil, Usage: p.usageLogService, Requests: p.requestService}
		conversation, _, err := service.configure(dialCtx, conn, &RealtimeTarget{Channel: ch, Model: entry.ActualModel, ClientModel: model, APIFormat: llm.APIFormat(ep.APIFormat), Translation: translation}, config)
		if err != nil {
			return fail()
		}
		defer conversation.Close()
		if translation {
			if err := probeTranslationSession(ctx, conn, conversation.adapter); err != nil {
				return fail()
			}
			return &TestChannelResult{Latency: time.Since(start).Seconds(), Success: true, Message: lo.ToPtr("Realtime Translation configuration and close acknowledged")}, nil
		}
		if ep.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket.String() {
			configEvent := []byte(`{"type":"session.update","session":{"type":"realtime","output_modalities":["text"],"audio":{"input":{"turn_detection":null}}}}`)
			wire, err := conversation.adapter.Client(configEvent)
			if err != nil {
				return fail()
			}
			stepCtx, stepCancel := context.WithTimeout(ctx, 5*time.Second)
			err = conn.Write(stepCtx, websocket.MessageText, wire)
			if err != nil {
				stepCancel()
				return fail()
			}
			kind, ack, err := conn.Read(stepCtx)
			stepCancel()
			if err != nil || kind != websocket.MessageText || gjson.GetBytes(ack, "type").String() != "session.updated" {
				return fail()
			}
		}
		event, err := conversation.adapter.Client([]byte(`{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"ping"}]}}`))
		if err != nil {
			return fail()
		}
		probeCtx, probeCancel := context.WithTimeout(ctx, 5*time.Second)
		err = conn.Write(probeCtx, websocket.MessageText, event)
		if err != nil {
			probeCancel()
			return fail()
		}
		err = waitRealtimeItemAcknowledgement(probeCtx, conn, llm.APIFormat(ep.APIFormat))
		probeCancel()
		if err != nil {
			return fail()
		}
		return &TestChannelResult{Latency: time.Since(start).Seconds(), Success: true, Message: lo.ToPtr("Realtime WebSocket handshake, PCM configuration and conversation event acknowledged")}, nil
	}
	return nil, errors.New("Realtime endpoint is not explicitly configured")
}

func waitRealtimeItemAcknowledgement(ctx context.Context, conn *websocket.Conn, format llm.APIFormat) error {
	for range 8 {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		kind := gjson.GetBytes(data, "type").String()
		if kind == "error" {
			return errors.New("upstream rejected realtime probe item")
		}
		if format == llm.APIFormatOpenAIRealtimeWebSocket {
			if kind == "conversation.item.done" {
				return nil
			}
		} else if kind == "conversation.item.created" {
			return nil
		}
	}
	return errors.New("upstream did not acknowledge realtime probe item")
}

func probeTranslationSession(ctx context.Context, conn *websocket.Conn, adapter realtimeAdapter) error {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	update := []byte(`{"type":"session.update","session":{"audio":{"output":{"language":"es"}}}}`)
	closeEvent := []byte(`{"type":"session.close"}`)
	for _, step := range []struct {
		wire []byte
		want string
	}{{update, "session.updated"}, {closeEvent, "session.closed"}} {
		data, err := adapter.Client(step.wire)
		if err != nil {
			return err
		}
		if err := conn.Write(probeCtx, websocket.MessageText, data); err != nil {
			return err
		}
		kind, response, err := conn.Read(probeCtx)
		if err != nil {
			return err
		}
		if kind != websocket.MessageText || gjson.GetBytes(response, "type").String() != step.want {
			return errors.New("upstream did not acknowledge translation probe")
		}
		if _, err := adapter.Server(response); err != nil {
			return err
		}
	}
	return nil
}
