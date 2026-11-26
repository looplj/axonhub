// 职责: 定义渠道自有 Realtime 配置及端点输入的协议隔离边界。
// 关联文件:
//
//	internal/objects/channel.go
//	internal/server/biz/channel_endpoint.go
//	internal/server/gql/axonhub.graphql
//
// 注意事项: 数据库存储的旧端点仍可解码迁移；只有管理输入使用受限标量。

package objects

import (
	"strings"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

type ChannelRealtimeConfig struct {
	Enabled               bool   `json:"enabled"`
	Protocol              string `json:"protocol"`
	BaseURL               string `json:"baseURL"`
	Path                  string `json:"path"`
	HandshakeTimeoutMs    *int64 `json:"handshakeTimeoutMs,omitempty"`
	ClientEventsPerSecond *int64 `json:"clientEventsPerSecond,omitempty"`
}

const ErrCodeEndpointRealtimeNotAllowed = "ENDPOINT_REALTIME_NOT_ALLOWED"

func IsRealtimeAPIFormat(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "openai/realtime") || strings.HasPrefix(value, "qwen/realtime")
}

func RealtimeEndpointInputError() error {
	return &gqlerror.Error{Message: ErrCodeEndpointRealtimeNotAllowed + ": Realtime protocols must be configured on the channel, not in endpoints", Extensions: map[string]any{"code": ErrCodeEndpointRealtimeNotAllowed}}
}
