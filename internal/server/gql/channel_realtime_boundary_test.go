// 职责: 通过真实 GraphQL HTTP 请求验证渠道 Realtime 保存与端点协议边界。
// 关联文件:
//
//	internal/server/gql/axonhub.graphql
//	internal/server/biz/channel_endpoint.go
//	internal/server/biz/channel_settings.go
//
// 注意事项: 端点拒绝必须是非200且数据库不变，不能只验证客户端下拉或错误字符串。

package gql

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
)

func realtimeConfigGraphQLFixture(t *testing.T) (*ent.Client, context.Context, func(string, map[string]any) (int, []byte)) {
	t.Helper()
	db := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=1")
	t.Cleanup(func() { _ = db.Close() })
	seed := authz.WithTestBypass(ent.NewContext(context.Background(), db))
	owner := db.User.Create().SetEmail("owner@example.test").SetPassword("synthetic").SetIsOwner(true).SaveX(seed)
	cache := xcache.Config{Mode: xcache.ModeMemory}
	system := biz.NewSystemService(biz.SystemServiceParams{Ent: db, CacheConfig: cache})
	channels := biz.NewChannelService(biz.ChannelServiceParams{Ent: db, CacheConfig: cache, SystemService: system, ModelCache: biz.NewModelCacheService(biz.ModelCacheServiceParams{Ent: db}), HttpClient: httpclient.NewHttpClient()})
	h := NewGraphqlHandlers(Dependencies{Ent: db, ChannelService: channels, SystemService: system})
	ctx := contexts.WithUser(authz.NewUserContext(ent.NewContext(context.Background(), db), owner.ID), owner)
	execute := func(query string, vars map[string]any) (int, []byte) {
		payload, e := json.Marshal(map[string]any{"query": query, "variables": vars})
		require.NoError(t, e)
		req := httptest.NewRequest(http.MethodPost, "/admin/graphql", bytes.NewReader(payload)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.Graphql.ServeHTTP(w, req)
		return w.Code, w.Body.Bytes()
	}
	return db, seed, execute
}

func TestRealtimeEndpointMutationsRejectedBeforeWrite(t *testing.T) {
	for _, kind := range []string{"openai", "bailian"} {
		for _, mutation := range []string{"save", "update", "append", "create"} {
			t.Run(kind+"/"+mutation, func(t *testing.T) {
				db, seed, execute := realtimeConfigGraphQLFixture(t)
				format, base, path := "openai/realtime-ws", "wss://api.openai.com", "/v1/realtime"
				if kind == "bailian" {
					format, base, path = "qwen/realtime-ws", "wss://llm-probe.cn-beijing.maas.aliyuncs.com", "/api-ws/v1/realtime"
				}
				endpoint := map[string]any{"apiFormat": format, "baseURL": base, "path": path, "transport": "websocket"}
				input := map[string]any{"type": kind, "name": "probe", "credentials": map[string]any{"apiKey": "synthetic"}, "supportedModels": []string{"voice"}, "defaultTestModel": "voice"}
				status, body := execute(`mutation($i:CreateChannelInput!){createChannel(input:$i){id}}`, map[string]any{"i": input})
				require.Equal(t, 200, status, string(body))
				id := gjson.GetBytes(body, "data.createChannel.id").String()
				require.NotEmpty(t, id, string(body))
				query := `mutation($i:SaveChannelEndpointsInput!){saveChannelEndpoints(input:$i){id}}`
				vars := map[string]any{"i": map[string]any{"channelID": id, "endpoints": []any{endpoint}}}
				switch mutation {
				case "update", "append":
					field := "endpoints"
					if mutation == "append" {
						field = "appendEndpoints"
					}
					query = `mutation($id:ID!,$i:UpdateChannelInput!){updateChannel(id:$id,input:$i){id}}`
					vars = map[string]any{"id": id, "i": map[string]any{field: []any{endpoint}}}
				case "create":
					input["name"] = "second"
					input["endpoints"] = []any{endpoint}
					query = `mutation($i:CreateChannelInput!){createChannel(input:$i){id}}`
					vars = map[string]any{"i": input}
				}
				status, body = execute(query, vars)
				require.NotEqual(t, 200, status, string(body))
				require.Contains(t, string(body), "ENDPOINT_REALTIME_NOT_ALLOWED")
				require.Empty(t, db.Channel.Query().FirstX(seed).Endpoints)
				require.Equal(t, 1, db.Channel.Query().CountX(seed))
			})
		}
	}
}

func TestChannelRealtimeConfigurationPersists(t *testing.T) {
	for _, kind := range []string{"openai", "bailian"} {
		t.Run(kind, func(t *testing.T) {
			db, seed, execute := realtimeConfigGraphQLFixture(t)
			protocol, base, path := "openai/realtime-ws", "wss://api.openai.com", "/v1/realtime"
			if kind == "bailian" {
				protocol, base, path = "qwen/realtime-ws", "wss://llm-probe.cn-beijing.maas.aliyuncs.com", "/api-ws/v1/realtime"
			}
			config := map[string]any{"enabled": true, "protocol": protocol, "baseURL": base, "path": path, "handshakeTimeoutMs": 3456, "clientEventsPerSecond": 1024}
			status, body := execute(`mutation($i:CreateChannelInput!){createChannel(input:$i){id settings{realtime{enabled protocol baseURL path handshakeTimeoutMs clientEventsPerSecond}} endpoints{apiFormat}}}`, map[string]any{"i": map[string]any{"type": kind, "name": "configured", "credentials": map[string]any{"apiKey": "synthetic"}, "supportedModels": []string{"voice"}, "defaultTestModel": "voice", "settings": map[string]any{"realtime": config}}})
			require.Equal(t, 200, status, string(body))
			require.False(t, gjson.GetBytes(body, "errors").Exists(), string(body))
			id := gjson.GetBytes(body, "data.createChannel.id").String()
			require.NotEmpty(t, id)
			require.Equal(t, protocol, gjson.GetBytes(body, "data.createChannel.settings.realtime.protocol").String(), "create must persist the independent channel protocol before any update")
			require.True(t, gjson.GetBytes(body, "data.createChannel.settings.realtime.enabled").Bool())
			require.Equal(t, int64(1024), gjson.GetBytes(body, "data.createChannel.settings.realtime.clientEventsPerSecond").Int())
			config["handshakeTimeoutMs"] = 4567
			config["clientEventsPerSecond"] = 300
			status, body = execute(`mutation($id:ID!,$i:UpdateChannelInput!){updateChannel(id:$id,input:$i){id settings{realtime{enabled protocol handshakeTimeoutMs clientEventsPerSecond}}}}`, map[string]any{"id": id, "i": map[string]any{"settings": map[string]any{"realtime": config}}})
			require.Equal(t, 200, status, string(body))
			require.False(t, gjson.GetBytes(body, "errors").Exists(), string(body))
			status, body = execute(`query($id:ID!){node(id:$id){... on Channel{settings{realtime{enabled protocol baseURL handshakeTimeoutMs clientEventsPerSecond}} endpoints{apiFormat}}}}`, map[string]any{"id": id})
			require.Equal(t, 200, status)
			require.Equal(t, protocol, gjson.GetBytes(body, "data.node.settings.realtime.protocol").String())
			require.Equal(t, int64(4567), gjson.GetBytes(body, "data.node.settings.realtime.handshakeTimeoutMs").Int())
			require.Equal(t, int64(300), gjson.GetBytes(body, "data.node.settings.realtime.clientEventsPerSecond").Int())
			stored := db.Channel.Query().OnlyX(seed)
			require.Empty(t, stored.Endpoints)
			raw, err := json.Marshal(stored.Settings)
			require.NoError(t, err)
			require.Equal(t, protocol, gjson.GetBytes(raw, "realtime.protocol").String())
			for _, invalidRate := range []int{100, 5000} {
				config["clientEventsPerSecond"] = invalidRate
				status, body = execute(`mutation($id:ID!,$i:UpdateChannelInput!){updateChannel(id:$id,input:$i){id}}`, map[string]any{"id": id, "i": map[string]any{"settings": map[string]any{"realtime": config}}})
				require.Equal(t, 200, status)
				require.Contains(t, string(body), "Realtime client events per second must be between 256 and 4096")
			}
			config["clientEventsPerSecond"] = 300
			// Existing model routing belongs to the channel and must survive
			// ordinary HTTP endpoint edits after the realtime migration.
			status, body = execute(`mutation($id:ID!,$i:UpdateChannelSettingsInput!){updateChannelSettings(id:$id,input:$i){id}}`, map[string]any{"id": id, "i": map[string]any{"modelProtocols": []any{map[string]any{"model": "voice", "apiFormats": []string{protocol}}}}})
			require.Equal(t, 200, status, string(body))
			require.False(t, gjson.GetBytes(body, "errors").Exists(), string(body))
			status, body = execute(`mutation($i:SaveChannelEndpointsInput!){saveChannelEndpoints(input:$i){id settings{realtime{protocol}} endpoints{apiFormat} defaultEndpoints{apiFormat}}}`, map[string]any{"i": map[string]any{"channelID": id, "endpoints": []any{map[string]any{"apiFormat": "openai/chat_completions", "path": "/chat/completions"}}}})
			require.Equal(t, 200, status, string(body))
			require.False(t, gjson.GetBytes(body, "errors").Exists(), string(body))
			require.Equal(t, protocol, gjson.GetBytes(body, "data.saveChannelEndpoints.settings.realtime.protocol").String())
			for _, ep := range gjson.GetBytes(body, "data.saveChannelEndpoints.defaultEndpoints").Array() {
				require.NotContains(t, ep.Get("apiFormat").String(), "realtime", "public endpoint defaults must not expose channel protocols")
			}
		})
	}
}

func TestSaveChannelEndpointsAcceptsLegacyStringVariable(t *testing.T) {
	db, seed, execute := realtimeConfigGraphQLFixture(t)
	input := map[string]any{"type": "openai", "name": "legacy-client", "credentials": map[string]any{"apiKey": "synthetic"}, "supportedModels": []string{"voice"}, "defaultTestModel": "voice"}
	status, body := execute(`mutation($i:CreateChannelInput!){createChannel(input:$i){id}}`, map[string]any{"i": input})
	require.Equal(t, 200, status, string(body))
	id := gjson.GetBytes(body, "data.createChannel.id").String()
	require.NotEmpty(t, id, string(body))
	query := `mutation($id:ID!,$format:String!){saveChannelEndpoints(input:{channelID:$id,endpoints:[{apiFormat:$format}]}){id}}`
	status, body = execute(query, map[string]any{"id": id, "format": "openai/chat_completions"})
	require.Equal(t, 200, status, string(body))
	require.Equal(t, id, gjson.GetBytes(body, "data.saveChannelEndpoints.id").String(), string(body))
	status, body = execute(query, map[string]any{"id": id, "format": "openai/realtime-ws"})
	require.Equal(t, 422, status, string(body))
	require.Equal(t, "ENDPOINT_REALTIME_NOT_ALLOWED", gjson.GetBytes(body, "errors.0.extensions.code").String())
	require.Equal(t, "openai/chat_completions", db.Channel.Query().OnlyX(seed).Endpoints[0].APIFormat)
}

func TestUpdateXAISubscriptionRejectsRealtimeBeforeNormalizingEndpoints(t *testing.T) {
	db, seed, execute := realtimeConfigGraphQLFixture(t)
	input := map[string]any{"type": "xai_subscription", "name": "xai-legacy-client", "credentials": map[string]any{"apiKey": "synthetic"}, "supportedModels": []string{"grok"}, "defaultTestModel": "grok"}
	status, body := execute(`mutation($i:CreateChannelInput!){createChannel(input:$i){id}}`, map[string]any{"i": input})
	require.Equal(t, 200, status, string(body))
	id := gjson.GetBytes(body, "data.createChannel.id").String()
	require.NotEmpty(t, id, string(body))

	for _, explicitType := range []bool{false, true} {
		patch := map[string]any{"name": "must-not-save", "endpoints": []any{map[string]any{"apiFormat": "openai/realtime-ws"}}}
		if explicitType {
			patch["type"] = "xai_subscription"
		}
		status, body = execute(`mutation($id:ID!,$i:UpdateChannelInput!){updateChannel(id:$id,input:$i){id}}`, map[string]any{"id": id, "i": patch})
		require.Equal(t, 422, status, "explicit type: %t; %s", explicitType, body)
		require.Equal(t, "ENDPOINT_REALTIME_NOT_ALLOWED", gjson.GetBytes(body, "errors.0.extensions.code").String())
		saved := db.Channel.Query().OnlyX(seed)
		require.Equal(t, "xai-legacy-client", saved.Name)
		require.Empty(t, saved.Endpoints)
	}
}
