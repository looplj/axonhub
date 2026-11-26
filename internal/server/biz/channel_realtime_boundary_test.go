// 职责: 验证运行时Realtime只由渠道配置派生，不能从遗留端点获得能力。
// 关联文件:
//
//	internal/server/biz/channel_endpoint.go
//	internal/server/biz/channel_realtime.go
//	internal/objects/channel_realtime.go

package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestRuntimeRealtimeOnlyComesFromChannelConfiguration(t *testing.T) {
	ch := &Channel{Channel: &ent.Channel{Type: channel.TypeOpenai, Endpoints: []objects.ChannelEndpoint{{APIFormat: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}}}
	count := func() int {
		n := 0
		for _, ep := range ch.ResolveEndpoints() {
			if objects.IsRealtimeAPIFormat(ep.APIFormat) {
				n++
			}
		}
		return n
	}
	require.Zero(t, count(), "legacy endpoints require migration, never a runtime fallback")
	ch.Settings = &objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}
	require.Equal(t, 1, count(), "only the channel-derived connection is offered")
	ch.Settings.Realtime.Enabled = false
	require.Zero(t, count())
	ch.Settings.Realtime.Enabled = true
	ch.Settings.Realtime.BaseURL = "wss://127.0.0.1"
	require.Zero(t, count(), "invalid channel config cannot fall back to an old endpoint")
}

func TestCreateChannelRejectsRealtimeBeforeProviderEndpointNormalization(t *testing.T) {
	svc, db := setupTestChannelService(t)
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), db))
	before := db.Channel.Query().CountX(ctx)
	_, err := svc.CreateChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeXaiSubscription,
		Name:             "direct-create-realtime-endpoint",
		BaseURL:          new("https://untrusted.example/v1"),
		Credentials:      objects.ChannelCredentials{APIKey: `{"access_token":"synthetic"}`},
		SupportedModels:  []string{"grok-4.5"},
		DefaultTestModel: "grok-4.5",
		Endpoints:        []objects.ChannelEndpoint{{APIFormat: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}},
	})
	require.ErrorContains(t, err, objects.ErrCodeEndpointRealtimeNotAllowed)
	require.Equal(t, before, db.Channel.Query().CountX(ctx), "rejected endpoint input must not create a channel")
}

func TestDuplicateChannelRejectsRealtimeBeforeProviderEndpointNormalization(t *testing.T) {
	svc, db := setupTestChannelService(t)
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), db))
	source := db.Channel.Create().
		SetType(channel.TypeXaiSubscription).
		SetName("direct-duplicate-realtime-source").
		SetBaseURL("https://api.x.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: `{"access_token":"synthetic"}`}).
		SetSupportedModels([]string{"grok-4.5"}).
		SetDefaultTestModel("grok-4.5").
		SaveX(ctx)
	before := db.Channel.Query().CountX(ctx)
	_, err := svc.DuplicateChannel(ctx, source.ID, ent.CreateChannelInput{
		Type:             channel.TypeXaiSubscription,
		Name:             "direct-duplicate-realtime-result",
		Credentials:      objects.ChannelCredentials{APIKey: `{"access_token":"synthetic"}`},
		SupportedModels:  []string{"grok-4.5"},
		DefaultTestModel: "grok-4.5",
		Endpoints:        []objects.ChannelEndpoint{{APIFormat: "qwen/realtime-ws", BaseURL: "wss://workspace.cn-beijing.maas.aliyuncs.com", Path: "/api-ws/v1/realtime"}},
	})
	require.ErrorContains(t, err, objects.ErrCodeEndpointRealtimeNotAllowed)
	require.Equal(t, before, db.Channel.Query().CountX(ctx), "rejected duplicate input must not create a channel")
}
