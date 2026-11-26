// 职责: 使用真实本地 WebSocket 和数据库验证实时协议、多轮、工具关联与恰好一次账单。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//	internal/server/biz/realtime_billing.go
//	internal/server/realtime/session.go

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
)

func realtimeReadWithTimeout(conn *websocket.Conn, timeout time.Duration) (websocket.MessageType, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return conn.Read(ctx)
}

func realtimeWriteText(conn *websocket.Conn, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data)
}

func realtimeWriteJSON(conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return realtimeWriteText(conn, data)
}

func realtimeSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err == nil {
			conn.SetReadLimit(rt.MaxMessageBytes)
			accepted <- conn
		}
	}))
	t.Cleanup(srv.Close)
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)
	conn.SetReadLimit(rt.MaxMessageBytes)
	peer := <-accepted
	t.Cleanup(func() { _ = conn.CloseNow(); _ = peer.CloseNow() })
	return conn, peer
}

func TestRealtimeCheckRevalidatesCurrentChannelTags(t *testing.T) {
	ctx, db := setupTest(t)
	project := db.Project.Create().SetName("Realtime tag refresh").SaveX(ctx)
	profile := &objects.APIKeyProfiles{ActiveProfile: "tag-limited", Profiles: []objects.APIKeyProfile{{
		Name: "tag-limited", ChannelTags: []string{"allowed"}, ChannelTagsMatchMode: objects.ChannelTagsMatchModeAny,
	}}}
	key := db.APIKey.Create().SetName("Realtime tag refresh").SetKey("synthetic-tag-client-key").SetUserID(1).SetProjectID(project.ID).SetType(apikey.TypeUser).SetScopes([]string{"read_channels", "write_requests"}).SetProfiles(profile).SetStatus(apikey.StatusEnabled).SaveX(ctx)
	row := db.Channel.Create().SetType(channel.TypeOpenai).SetName("Realtime tag refresh").SetBaseURL("https://api.openai.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-upstream-key"}).SetSupportedModels([]string{"voice"}).SetDefaultTestModel("voice").SetStatus(channel.StatusEnabled).SetTags([]string{"allowed"}).SetSettings(&objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}).SaveX(ctx)
	channels := newTestChannelServiceForChannels(db)
	channelAtConnect, err := channels.GetChannel(ctx, row.ID)
	require.NoError(t, err)
	channels.SetEnabledChannelsForTest([]*biz.Channel{channelAtConnect})
	system := newTestSystemService(db)
	cache := xcache.Config{Mode: xcache.ModeMemory}
	projects := biz.NewProjectService(biz.ProjectServiceParams{CacheConfig: cache, Ent: db})
	keys := biz.NewAPIKeyService(biz.APIKeyServiceParams{CacheConfig: cache, Ent: db, ProjectService: projects, KeyPrefix: "ah"})
	t.Cleanup(keys.Stop)
	auth := biz.NewAuthService(biz.AuthServiceParams{SystemService: system, APIKeyService: keys, Ent: db})
	key.Edges.Project = project
	requestCtx := contexts.WithProjectID(contexts.WithAPIKey(ent.NewContext(context.Background(), db), key), project.ID)
	requestCtx, err = authz.WithPrincipal(requestCtx, authz.Principal{Type: authz.PrincipalTypeAPIKey, APIKeyID: &key.ID, ProjectID: &project.ID})
	require.NoError(t, err)
	service := &RealtimeService{Auth: auth, Channels: channels, Usage: biz.NewUsageLogService(db, system, channels)}
	c := &RealtimeConversation{service: service, ctx: requestCtx, state: rt.NewState(), target: &RealtimeTarget{Channel: channelAtConnect, APIKey: key}, lastCheck: time.Time{}}
	require.NoError(t, c.Check(requestCtx))
	accountingKey, ok := contexts.GetAPIKey(c.ctx)
	require.True(t, ok)
	require.Same(t, key, accountingKey, "reauthorization must preserve the session accounting identity")
	require.NotSame(t, key, c.latestKey, "the authorization check must use the refreshed key")

	currentChannel := &biz.Channel{Channel: &ent.Channel{ID: row.ID, Type: channel.TypeOpenai, Tags: []string{"forbidden"}}}
	channels.SetEnabledChannelsForTest([]*biz.Channel{currentChannel})
	c.lastCheck = time.Time{}
	require.ErrorContains(t, c.Check(requestCtx), "realtime channel permission revoked")
}

func TestRealtimeConfigurePreservesSessionCreatedTimeout(t *testing.T) {
	upstream, _ := realtimeSocketPair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	target := &RealtimeTarget{
		APIFormat:   llm.APIFormatOpenAIRealtimeWebSocket,
		Model:       "gpt-realtime-2.1",
		ClientModel: "gpt-realtime-2.1",
	}
	_, _, err := (&RealtimeService{}).configure(ctx, upstream, target, rt.Config{HandshakeTimeout: time.Second}.Defaults())
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRealtimeNativeMultipleTurnsAccountingAndFunctionIDs(t *testing.T) {
	for _, model := range []string{"gpt-realtime-2.1", "gpt-realtime-2.1-mini"} {
		t.Run(model, func(t *testing.T) {
			ctx, db := setupTest(t)
			project := db.Project.Create().SetName("Realtime").SaveX(ctx)
			ctx = contexts.WithProjectID(ctx, project.ID)
			key := db.APIKey.Create().SetName("Realtime").SetKey("synthetic-client-key").SetUserID(1).SetProjectID(project.ID).SetType(apikey.TypeUser).SetScopes([]string{"read_channels", "write_requests"}).SetStatus(apikey.StatusEnabled).SaveX(ctx)
			ctx = contexts.WithAPIKey(ctx, key)
			row := db.Channel.Create().SetType(channel.TypeOpenai).SetName("Realtime").SetBaseURL("https://api.openai.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-upstream-key"}).SetSupportedModels([]string{model}).SetDefaultTestModel(model).SetStatus(channel.StatusEnabled).SetSettings(&objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}).SaveX(ctx)
			channels := newTestChannelServiceForChannels(db)
			ch, err := channels.GetChannel(ctx, row.ID)
			require.NoError(t, err)
			channels.SetEnabledChannelsForTest([]*biz.Channel{ch})
			system := newTestSystemService(db)
			usage := biz.NewUsageLogService(db, system, channels)
			requests := newTestRequestServiceForChannels(db, system)
			price := objects.ModelPrice{}
			for _, code := range []string{"realtime_input_text_tokens", "realtime_input_audio_tokens", "realtime_output_text_tokens", "realtime_output_audio_tokens", "realtime_input_image_tokens", "realtime_cached_input_text_tokens", "realtime_cached_input_audio_tokens", "realtime_cached_input_image_tokens"} {
				rate := decimal.NewFromInt(1)
				price.Items = append(price.Items, objects.ModelPriceItem{ItemCode: objects.PriceItemCode(code), Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &rate}})
			}
			cache := xcache.Config{Mode: xcache.ModeMemory}
			projects := biz.NewProjectService(biz.ProjectServiceParams{CacheConfig: cache, Ent: db})
			keys := biz.NewAPIKeyService(biz.APIKeyServiceParams{CacheConfig: cache, Ent: db, ProjectService: projects, KeyPrefix: "ah"})
			t.Cleanup(keys.Stop)
			auth := biz.NewAuthService(biz.AuthServiceParams{SystemService: system, APIKeyService: keys, Ent: db})
			service := &RealtimeService{Channels: channels, Usage: usage, Requests: requests, Auth: auth}
			key.Edges.Project = project
			requestCtx := contexts.WithProjectID(contexts.WithAPIKey(ent.NewContext(context.Background(), db), key), project.ID)
			requestCtx, err = authz.WithPrincipal(requestCtx, authz.Principal{Type: authz.PrincipalTypeAPIKey, APIKeyID: &key.ID, ProjectID: &project.ID})
			require.NoError(t, err)
			upstream, provider := realtimeSocketPair(t)
			require.NoError(t, realtimeWriteJSON(provider, map[string]any{"type": "session.created", "session": map[string]any{"id": "s1", "type": "realtime", "model": model}}))
			c, initial, err := service.configure(requestCtx, upstream, &RealtimeTarget{Channel: ch, Model: model, ClientModel: "voice", APIFormat: llm.APIFormatOpenAIRealtimeWebSocket, APIKey: key, Price: &biz.RealtimePriceSnapshot{Price: price, ReferenceID: "test-price-v1"}}, rt.Config{}.Defaults())
			require.NoError(t, err)
			require.Len(t, initial, 1)
			defer c.Close()
			// Drive the real conversation client hook, auth/quota checks and both sockets.
			client, downstream := realtimeSocketPair(t)
			sessionCtx, cancel := context.WithCancel(requestCtx)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("session panic: %T", r)
						done <- fmt.Errorf("panic: %T", r)
					}
				}()
				done <- rt.Run(sessionCtx, downstream, upstream, rt.Options{IdleTimeout: time.Second}, rt.Hooks{Client: c.Client, Server: c.Server})
			}()
			update := []byte(`{"type":"session.update","session":{"type":"realtime","output_modalities":["text"],"future_official_field":true}}`)
			require.NoError(t, realtimeWriteText(client, update))
			_, wire, err := realtimeReadWithTimeout(provider, time.Second)
			require.NoError(t, err)
			require.Contains(t, string(wire), "future_official_field")
			var sent struct {
				EventID string `json:"event_id"`
			}
			require.NoError(t, json.Unmarshal(wire, &sent))
			require.NotEmpty(t, sent.EventID)
			require.NoError(t, realtimeWriteJSON(provider, map[string]any{"type": "error", "error": map[string]string{"event_id": sent.EventID, "code": "unknown_parameter", "message": "Unknown parameter: 'session.future_official_field'."}}))
			_, rejected, err := realtimeReadWithTimeout(client, time.Second)
			require.NoError(t, err)
			require.Contains(t, string(rejected), "unknown_parameter")
			require.Contains(t, string(rejected), "Unknown parameter: 'session.future_official_field'.")
			require.NoError(t, realtimeWriteText(client, update))
			_, retried, err := realtimeReadWithTimeout(provider, time.Second)
			require.NoError(t, err)
			previousID := sent.EventID
			sent.EventID = ""
			require.NoError(t, json.Unmarshal(retried, &sent))
			require.NotEmpty(t, sent.EventID)
			require.NotEqual(t, previousID, sent.EventID)
			require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"session.updated","session":{"id":"s1","type":"realtime"}}`)))
			_, _, err = realtimeReadWithTimeout(client, time.Second)
			require.NoError(t, err)
			read := func(expected string) {
				t.Helper()
				_, raw, err := realtimeReadWithTimeout(client, 5*time.Second)
				require.NoError(t, err)
				require.JSONEq(t, expected, string(raw))
			}
			for attempt := range 9 {
				eventID := fmt.Sprintf("rejected-%d", attempt)
				create := fmt.Sprintf(`{"type":"response.create","event_id":%q}`, eventID)
				require.NoError(t, realtimeWriteText(client, []byte(create)))
				_, forwarded, err := realtimeReadWithTimeout(provider, 2*time.Second)
				require.NoError(t, err)
				require.JSONEq(t, create, string(forwarded))
				require.NoError(t, realtimeWriteJSON(provider, map[string]any{"type": "error", "error": map[string]string{"event_id": eventID}}))
				_, rejected, err := realtimeReadWithTimeout(client, 2*time.Second)
				require.NoError(t, err)
				require.Contains(t, string(rejected), eventID)
			}
			require.Zero(t, db.UsageLog.Query().CountX(ctx))
			require.Equal(t, 9, db.Request.Query().Where(request.StatusEQ(request.StatusFailed)).CountX(ctx))
			require.NoError(t, realtimeWriteText(client, []byte(`{"type":"response.create"}`)))
			_, initialCreate, err := realtimeReadWithTimeout(provider, 5*time.Second)
			require.NoError(t, err)
			require.Contains(t, string(initialCreate), `"event_id":"evt_axonhub_`)
			require.Contains(t, string(initialCreate), `"type":"response.create"`)
			for turn := 1; turn <= 2; turn++ {
				created := fmt.Sprintf(`{"type":"response.created","response":{"id":"r%d","status":"in_progress"}}`, turn)
				require.NoError(t, realtimeWriteText(provider, []byte(created)))
				read(created)
				events := []string{fmt.Sprintf(`{"type":"response.output_text.delta","response_id":"r%d","item_id":"item1","delta":"private transcript","future":true}`, turn), fmt.Sprintf(`{"type":"response.function_call_arguments.delta","response_id":"r%d","item_id":"item1","call_id":"call1","delta":"{}"}`, turn), fmt.Sprintf(`{"type":"response.function_call_arguments.done","response_id":"r%d","item_id":"item1","call_id":"call1","arguments":"{}"}`, turn)}
				if turn == 2 {
					events = events[:1]
				}
				for _, event := range events {
					require.NoError(t, realtimeWriteText(provider, []byte(event)))
					read(event)
				}
				terminal := fmt.Sprintf(`{"type":"response.done","response":{"id":"r%d","status":"completed","usage":{"input_tokens":100,"output_tokens":30,"total_tokens":130,"input_token_details":{"text_tokens":40,"audio_tokens":50,"image_tokens":10,"cached_tokens":30,"cached_tokens_details":{"text_tokens":10,"audio_tokens":15,"image_tokens":5}},"output_token_details":{"text_tokens":10,"audio_tokens":20}}}}`, turn)
				require.NoError(t, realtimeWriteText(provider, []byte(terminal)))
				read(terminal)
				require.NoError(t, realtimeWriteText(provider, []byte(terminal)))
				barrier := fmt.Sprintf(`{"type":"rate_limits.updated","barrier":%d}`, turn)
				require.NoError(t, realtimeWriteText(provider, []byte(barrier)))
				read(barrier) // FIFO proves the duplicate frame has traversed the real Server hook.
				require.Equal(t, turn, db.UsageLog.Query().CountX(ctx))
				if turn == 1 {
					executeTool := func(arguments string) string { require.JSONEq(t, "{}", arguments); return `{"ok":true}` }
					result := map[string]any{"type": "conversation.item.create", "item": map[string]any{"type": "function_call_output", "call_id": "call1", "output": executeTool("{}")}}
					require.NoError(t, realtimeWriteJSON(client, result))
					_, raw, err := realtimeReadWithTimeout(provider, 5*time.Second)
					require.NoError(t, err)
					var forwarded map[string]any
					require.NoError(t, json.Unmarshal(raw, &forwarded))
					require.Equal(t, "call1", forwarded["item"].(map[string]any)["call_id"])
					require.Equal(t, `{"ok":true}`, forwarded["item"].(map[string]any)["output"])
					require.NoError(t, realtimeWriteText(client, []byte(`{"type":"response.create","event_id":"accepted-create"}`)))
					_, raw, err = realtimeReadWithTimeout(provider, 5*time.Second)
					require.NoError(t, err)
					require.JSONEq(t, `{"type":"response.create","event_id":"accepted-create"}`, string(raw))
				}
			}
			// Duplicate completion frames must not produce an extra downstream event or ledger entry.
			require.Eventually(t, func() bool { return db.UsageLog.Query().CountX(ctx) == 2 }, time.Second, 10*time.Millisecond)
			for _, log := range db.UsageLog.Query().AllX(ctx) {
				require.EqualValues(t, 130, log.TotalTokens)
				require.Len(t, log.CostItems, 8)
				require.Equal(t, "test-price-v1", log.CostPriceReferenceID)
				require.NotNil(t, log.TotalCost)
				require.InDelta(t, 0.00013, *log.TotalCost, 0.000000001)
			}
			for _, r := range db.Request.Query().AllX(ctx) {
				if r.Status == request.StatusFailed {
					require.Contains(t, string(r.ResponseBody), `"response_create_rejected":true`)
					continue
				}
				require.Equal(t, request.StatusCompleted, r.Status)
				body, _ := json.Marshal(r.ResponseBody)
				require.NotContains(t, string(body), "private transcript")
				require.Equal(t, row.ID, r.ChannelID)
			}
			// Disconnect immediately after upstream accepts response.create, before
			// response.created or final usage arrives.
			require.NoError(t, realtimeWriteText(client, []byte(`{"type":"response.create","event_id":"accepted-create"}`)))
			_, pendingCreate, err := realtimeReadWithTimeout(provider, 5*time.Second)
			require.NoError(t, err)
			require.JSONEq(t, `{"type":"response.create","event_id":"accepted-create"}`, string(pendingCreate))
			require.NoError(t, client.CloseNow())
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("session workers did not exit")
			}
			// A normal mobile disconnect after response.create leaves uncertain usage,
			// not confirmed debt. Exercise another real WS configuration afterwards.
			c.Close()
			require.NoError(t, usage.EnsureRealtimeUsageReconciled(requestCtx, key.ID))
			require.Equal(t, 2, db.UsageLog.Query().CountX(ctx), "disconnect must not fabricate zero usage")
			reconnected, reconnectedProvider := realtimeSocketPair(t)
			require.NoError(t, realtimeWriteText(reconnectedProvider, []byte(fmt.Sprintf(`{"type":"session.created","session":{"id":"s2","type":"realtime","model":%q}}`, model))))
			next, initial, err := service.configure(requestCtx, reconnected, c.target, rt.Config{}.Defaults())
			require.NoError(t, err, "the same key must be able to establish its next session")
			require.Len(t, initial, 1)
			next.Close()
		})
	}
}
