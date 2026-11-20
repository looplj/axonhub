// 职责: 通过严格鉴权的真实 HTTP/WS 入口验证断线交接不被收尾审计阻塞。
// 关联文件:
//
//	internal/server/api/realtime.go
//	internal/server/realtime/admission.go
//	internal/server/orchestrator/realtime.go
//
// 注意事项: 只将上游网络替换为本地 WS；鉴权、选路、租约和数据库执行生产路径。

package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm/httpclient"
)

type reconnectDialer struct{ url string }

func (d reconnectDialer) Connect(ctx context.Context, _ string, _ string, _ *httpclient.ProxyConfig, _ time.Duration, _ ...string) (*websocket.Conn, error) {
	c, _, err := websocket.DefaultDialer.DialContext(ctx, d.url, nil)
	return c, err
}

func newRealtimeReconnectFixture(t *testing.T, configureKey ...func(*ent.APIKeyCreate)) (*ent.Client, func() (*websocket.Conn, *http.Response, error), *atomic.Int32, <-chan struct{}) {
	t.Helper()
	return newRealtimeReconnectFixtureWithDuration(t, 0, configureKey...)
}

func newRealtimeReconnectFixtureWithDuration(t *testing.T, maxDuration time.Duration, configureKey ...func(*ent.APIKeyCreate)) (*ent.Client, func() (*websocket.Conn, *http.Response, error), *atomic.Int32, <-chan struct{}) {
	t.Helper()
	db := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=1")
	t.Cleanup(func() { _ = db.Close() })
	seed := authz.WithTestBypass(ent.NewContext(context.Background(), db))
	p := db.Project.Create().SetName("reconnect").SaveX(seed)
	createKey := db.APIKey.Create().SetName("reconnect").SetKey("synthetic-reconnect").SetType("user").SetProjectID(p.ID).SetScopes([]string{"read_channels", "write_requests"})
	for _, configure := range configureKey {
		configure(createKey)
	}
	key := createKey.SaveX(seed)
	const model = "gpt-realtime-2.1-mini"
	row := db.Channel.Create().SetName("reconnect").SetType(channel.TypeOpenai).SetStatus(channel.StatusEnabled).SetBaseURL("https://api.openai.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-provider"}).SetSupportedModels([]string{model}).SetDefaultTestModel(model).SetSettings(&objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}).SaveX(seed)
	price := objects.ModelPrice{}
	for _, code := range objects.AllRealtimePriceItemCodes() {
		rate := decimal.NewFromInt(1)
		price.Items = append(price.Items, objects.ModelPriceItem{ItemCode: code, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &rate}})
	}
	db.ChannelModelPrice.Create().SetChannelID(row.ID).SetModelID(model).SetPrice(price).SetReferenceID("test").SaveX(seed)
	cache := xcache.Config{Mode: xcache.ModeMemory}
	system := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cache, Ent: db})
	require.NoError(t, system.SetModelSettings(seed, biz.SystemModelSettings{FallbackToChannelsOnModelNotFound: true, QueryAllChannelModels: true}))
	channels := biz.NewChannelService(biz.ChannelServiceParams{ModelCache: biz.NewModelCacheService(biz.ModelCacheServiceParams{Ent: db}), CacheConfig: cache, Ent: db, SystemService: system, HttpClient: httpclient.NewHttpClient()})
	built, err := channels.GetChannel(seed, row.ID)
	require.NoError(t, err)
	channels.PreloadModelPricesForTest(seed, built)
	channels.SetEnabledChannelsForTest([]*biz.Channel{built})
	usage := biz.NewUsageLogService(db, system, channels)
	storage := &biz.DataStorageService{AbstractService: &biz.AbstractService{}, SystemService: system, Cache: xcache.NewFromConfig[ent.DataStorage](cache)}
	requests := biz.NewRequestService(db, cache, system, usage, storage, biz.NewLiveStreamRegistry())
	projects := biz.NewProjectService(biz.ProjectServiceParams{CacheConfig: cache, Ent: db})
	keys := biz.NewAPIKeyService(biz.APIKeyServiceParams{CacheConfig: cache, Ent: db, ProjectService: projects, KeyPrefix: "ah"})
	t.Cleanup(keys.Stop)
	auth := biz.NewAuthService(biz.AuthServiceParams{SystemService: system, APIKeyService: keys, Ent: db})
	models := biz.NewModelService(biz.ModelServiceParams{ModelCache: biz.NewModelCacheService(biz.ModelCacheServiceParams{Ent: db}), ChannelService: channels, SystemService: system, Ent: db})
	var active, peak atomic.Int32
	observed := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"type":"session.created","session":{"id":"s","type":"realtime","audio":{"input":{"format":{"type":"audio/pcm","rate":24000}},"output":{"format":{"type":"audio/pcm","rate":24000}}}}}`))
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
			select {
			case observed <- struct{}{}:
			default:
			}
		}
	}))
	t.Cleanup(upstream.Close)
	service := &orchestrator.RealtimeService{Transport: reconnectDialer{"ws" + strings.TrimPrefix(upstream.URL, "http")}, Channels: channels, Selector: orchestrator.NewDefaultSelector(channels, models, system), Usage: usage, Requests: requests, Auth: auth, Quota: biz.NewQuotaService(db, system)}
	h := &RealtimeHandlers{service: service, cache: cache, drained: make(chan struct{}), sessions: make(map[uint64]context.CancelFunc)}
	close(h.drained)
	router := gin.New()
	router.Use(middleware.WithEntClient(db))
	router.GET("/v1/realtime", RealtimeBoundary(), middleware.WithIPBlocklist(system), middleware.WithStrictAPIKeyConfig(auth, &middleware.APIKeyConfig{Headers: []string{"Authorization"}, RequireBearer: true}), middleware.WithSource(request.SourceAPI), h.Handler(rt.Config{MaxDuration: maxDuration, Limits: rt.Limits{APIKey: 1, ConnectionsPerMinute: 100}}, false))
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		h.mu.Lock()
		drained := h.drained
		h.mu.Unlock()
		select {
		case <-drained:
		case <-time.After(3 * time.Second):
			t.Error("realtime handlers did not drain")
		}
	})
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/realtime?model=" + model
	dial := func() (*websocket.Conn, *http.Response, error) {
		return websocket.DefaultDialer.Dial(url, http.Header{"Authorization": []string{"Bearer " + key.Key}})
	}
	return db, dial, &peak, observed
}

func TestRealtimeHTTPReconnectWhileAuditStillDraining(t *testing.T) {
	db, dial, peak, observed := newRealtimeReconnectFixture(t)
	first, _, err := dial()
	require.NoError(t, err)
	defer first.Close()
	_, _, err = first.ReadMessage()
	require.NoError(t, err)
	gate := make(chan struct{})
	entered := make(chan struct{})
	var once sync.Once
	defer close(gate)
	db.Request.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			if m.Op().Is(ent.OpUpdateOne | ent.OpUpdate) {
				once.Do(func() { close(entered); <-gate })
			}
			return next.Mutate(ctx, m)
		})
	})
	require.NoError(t, first.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create"}`)))
	select {
	case <-observed:
	case <-time.After(3 * time.Second):
		_ = first.SetReadDeadline(time.Now().Add(time.Second))
		_, rejected, readErr := first.ReadMessage()
		t.Fatalf("client event was not forwarded: %s %v", rejected, readErr)
	}
	require.NoError(t, first.UnderlyingConn().Close())
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("disconnect audit did not enter finalization")
	}
	next, response, err := dial()
	if next != nil {
		defer next.Close()
	}
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	require.NoError(t, err, fmt.Sprintf("new connection must succeed while disconnected predecessor only drains audit, got HTTP %d", status))
	require.Equal(t, http.StatusSwitchingProtocols, status)
	require.Equal(t, int32(1), peak.Load(), "never overlap live upstream connections")
}

func TestRealtimeHTTPImmediateReconnectWhileDisconnectDetectionIsPending(t *testing.T) {
	db, dial, peak, _ := newRealtimeReconnectFixture(t)
	first, _, err := dial()
	require.NoError(t, err)
	defer first.Close()
	_, _, err = first.ReadMessage()
	require.NoError(t, err)
	gate, entered := make(chan struct{}), make(chan struct{})
	var gated, unblocked sync.Once
	unblock := func() { unblocked.Do(func() { close(gate) }) }
	defer unblock()
	db.Request.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			if m.Op().Is(ent.OpCreate) {
				gated.Do(func() { close(entered); <-gate })
			}
			return next.Mutate(ctx, m)
		})
	})
	require.NoError(t, first.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create"}`)))
	require.NoError(t, first.UnderlyingConn().Close())
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request hook did not enter")
	}
	type result struct {
		conn     *websocket.Conn
		response *http.Response
		err      error
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("dial panic: %T", r)
				done <- result{err: fmt.Errorf("dial panic")}
			}
		}()
		c, r, e := dial()
		done <- result{c, r, e}
	}()
	var r result
	early := false
	select {
	case r = <-done:
		early = true
	case <-time.After(50 * time.Millisecond):
	}
	unblock()
	if !early {
		select {
		case r = <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("reconnect did not finish after transport drained")
		}
	}
	if r.conn != nil {
		defer r.conn.Close()
	}
	require.False(t, early, "one handshake must wait for the old transport, not reject or overbook")
	require.NoError(t, r.err, "the same single HTTP handshake must return 101, with no client retry")
	require.Equal(t, http.StatusSwitchingProtocols, r.response.StatusCode)
	require.Equal(t, int32(1), peak.Load())
}

func TestRealtimeHTTPLiveConcurrentSessionStillRejected(t *testing.T) {
	_, dial, peak, _ := newRealtimeReconnectFixture(t)
	first, _, err := dial()
	require.NoError(t, err)
	defer first.Close()
	second, response, err := dial()
	if second != nil {
		defer second.Close()
	}
	require.Error(t, err)
	require.NotNil(t, response)
	require.Equal(t, http.StatusTooManyRequests, response.StatusCode)
	require.Equal(t, int32(1), peak.Load(), "reconnect grace must not bypass concurrency")
}

func TestRealtimeHTTPFailedRequestWithConfirmedUsageStillEnforcesQuota(t *testing.T) {
	one := int64(1)
	db, dial, peak, _ := newRealtimeReconnectFixture(t, func(c *ent.APIKeyCreate) {
		c.SetProfiles(&objects.APIKeyProfiles{ActiveProfile: "one", Profiles: []objects.APIKeyProfile{{Name: "one", Quota: &objects.APIKeyQuota{Requests: &one, Period: objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeAllTime}}}}})
	})
	seed := authz.WithTestBypass(ent.NewContext(context.Background(), db))
	key := db.APIKey.Query().OnlyX(seed)
	model := db.Channel.Query().OnlyX(seed).SupportedModels[0]
	r := db.Request.Create().SetAPIKeyID(key.ID).SetProjectID(key.ProjectID).SetChannelID(1).SetModelID(model).SetFormat("openai/realtime").SetStatus("failed").SetRequestBody(objects.JSONRawMessage(`{}`)).SaveX(seed)
	db.UsageLog.Create().SetRequestID(r.ID).SetAPIKeyID(key.ID).SetProjectID(key.ProjectID).SetChannelID(1).SetModelID(r.ModelID).SetFormat("openai/realtime").SetSource("api").SetPromptTokens(1).SetCompletionTokens(1).SetTotalTokens(2).SaveX(seed)
	c, response, err := dial()
	if c != nil {
		defer c.Close()
	}
	require.Error(t, err)
	require.NotNil(t, response)
	require.Equal(t, http.StatusForbidden, response.StatusCode, "confirmed request quota still applies to failed requests")
	require.Zero(t, peak.Load(), "quota rejection precedes upstream dialing")
}

func TestRealtimeHTTPAuditInsertCancellationReleasesLease(t *testing.T) {
	db, dial, peak, _ := newRealtimeReconnectFixtureWithDuration(t, 500*time.Millisecond)
	first, _, err := dial()
	require.NoError(t, err)
	defer first.Close()
	require.NoError(t, first.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, _, err = first.ReadMessage()
	require.NoError(t, err)

	entered := make(chan struct{})
	canceled := make(chan error, 1)
	cleanup := make(chan struct{})
	defer close(cleanup)
	db.Request.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			if !m.Op().Is(ent.OpCreate) {
				return next.Mutate(ctx, m)
			}
			close(entered)
			select {
			case <-ctx.Done():
				canceled <- ctx.Err()
				return nil, ctx.Err()
			case <-cleanup:
				return nil, context.Canceled
			}
		})
	})
	require.NoError(t, first.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create"}`)))
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("audit INSERT did not reach the write boundary")
	}

	// Read the server close so the peer can complete its close handshake.
	_, _, err = first.ReadMessage()
	require.Error(t, err)
	select {
	case err = <-canceled:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("session deadline did not cancel the blocked audit INSERT")
	}

	second, response, err := dial()
	if second != nil {
		defer second.Close()
	}
	require.NoError(t, err, "the same API key must immediately regain its lease")
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	require.Equal(t, int32(1), peak.Load())
}
