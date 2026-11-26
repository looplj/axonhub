// 职责: 校验渠道自有的 Realtime 配置并派生仅供运行时使用的连接描述。
// 关联文件:
//
//	internal/objects/channel.go
//	internal/objects/channel_realtime.go
//	internal/server/biz/channel_realtime_migration.go
//	internal/server/biz/channel_endpoint.go
//
// 注意事项: Realtime 不作为持久化端点协议；禁用配置不会产生可选运行时端点。

// Package biz implements AxonHub business services and domain rules.
//
//nolint:godoclint // The repository requires a responsibility header before the package comment.
package biz

import (
	"errors"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	qwenrt "github.com/looplj/axonhub/llm/realtime"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

func ValidateChannelRealtime(kind channel.Type, cfg *objects.ChannelRealtimeConfig) error {
	if cfg == nil {
		return nil
	}
	if !cfg.Enabled && cfg.Protocol == "" && cfg.BaseURL == "" && cfg.Path == "" && cfg.HandshakeTimeoutMs == nil && cfg.ClientEventsPerSecond == nil {
		return nil
	}
	if cfg.HandshakeTimeoutMs != nil && (*cfg.HandshakeTimeoutMs < 1000 || *cfg.HandshakeTimeoutMs > 60000) {
		return errors.New("Realtime handshake timeout must be between 1000 and 60000 ms")
	}
	if cfg.ClientEventsPerSecond != nil && (*cfg.ClientEventsPerSecond < 256 || *cfg.ClientEventsPerSecond > 4096) {
		return errors.New("Realtime client events per second must be between 256 and 4096")
	}
	switch cfg.Protocol {
	case llm.APIFormatOpenAIRealtimeWebSocket.String():
		if kind != channel.TypeOpenai {
			return errors.New("OpenAI Realtime configuration requires the openai channel type")
		}
		return openairt.ValidateEndpoint(cfg.BaseURL, cfg.Path)
	case llm.APIFormatQwenRealtimeWebSocket.String():
		if kind != channel.TypeBailian {
			return errors.New("Qwen Realtime configuration requires the bailian channel type")
		}
		return qwenrt.ValidateEndpoint(cfg.BaseURL, cfg.Path)
	default:
		return errors.New("unsupported channel Realtime protocol")
	}
}

// RealtimeOnly uses the saved protocol selection, even if its config is invalid.
func (ch *Channel) RealtimeOnly() bool {
	return ch != nil && ch.Channel != nil && ch.Settings != nil && ch.Settings.Realtime != nil && ch.Settings.Realtime.Enabled
}

// RealtimeEndpoint derives transport metadata from the channel-owned setting.
// It never consults Channel.Endpoints, including legacy data awaiting migration.
func (ch *Channel) RealtimeEndpoint() *objects.ChannelEndpoint {
	if ch.Settings == nil || ch.Settings.Realtime == nil || !ch.Settings.Realtime.Enabled {
		return nil
	}
	cfg := ch.Settings.Realtime
	if ValidateChannelRealtime(ch.Type, cfg) != nil {
		return nil
	}
	host, path := cfg.BaseURL, cfg.Path
	if cfg.Protocol == llm.APIFormatOpenAIRealtimeWebSocket.String() {
		if host == "" {
			host = "wss://api.openai.com"
		}
		if path == "" {
			path = "/v1/realtime"
		}
	}
	ep := &objects.ChannelEndpoint{APIFormat: cfg.Protocol, BaseURL: host, Path: path, Transport: objects.ChannelEndpointTransportWebSocket}
	if cfg.HandshakeTimeoutMs != nil {
		timeout := *cfg.HandshakeTimeoutMs
		ep.HandshakeTimeoutMs = &timeout
	}
	return ep
}
