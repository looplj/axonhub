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
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

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
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

type reconnectDialer struct {
	url string
	err error
}

func (d reconnectDialer) Connect(ctx context.Context, _ string, _ string, _ *httpclient.ProxyConfig, _ time.Duration, _ ...string) (*websocket.Conn, error) {
	if d.err != nil {
		return nil, d.err
	}
	c, _, err := websocket.Dial(ctx, d.url, nil)
	if err == nil {
		c.SetReadLimit(rt.MaxMessageBytes)
	}
	return c, err
}

func apiRealtimeReadWithTimeout(conn *websocket.Conn, timeout time.Duration) (websocket.MessageType, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return conn.Read(ctx)
}

func apiRealtimeWriteText(conn *websocket.Conn, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data)
}

func newRealtimeReconnectFixture(t *testing.T, configureKey ...func(*ent.APIKeyCreate)) (*ent.Client, func() (*websocket.Conn, *http.Response, error), *atomic.Int32, <-chan struct{}) {
	t.Helper()
	return newRealtimeReconnectFixtureWithDuration(t, 0, configureKey...)
}

func newRealtimeReconnectFixtureWithDuration(t *testing.T, maxDuration time.Duration, configureKey ...func(*ent.APIKeyCreate)) (*ent.Client, func() (*websocket.Conn, *http.Response, error), *atomic.Int32, <-chan struct{}) {
	t.Helper()
	return newRealtimeHTTPFixture(t, maxDuration, false, configureKey...)
}

func newRealtimeHTTPFixture(t *testing.T, maxDuration time.Duration, translation bool, configureKey ...func(*ent.APIKeyCreate)) (*ent.Client, func() (*websocket.Conn, *http.Response, error), *atomic.Int32, <-chan struct{}) {
	return newRealtimeHTTPFixtureWithPrice(t, maxDuration, translation, true, configureKey...)
}

func newRealtimeHTTPFixtureWithPrice(t *testing.T, maxDuration time.Duration, translation, withPrice bool, configureKey ...func(*ent.APIKeyCreate)) (*ent.Client, func() (*websocket.Conn, *http.Response, error), *atomic.Int32, <-chan struct{}) {
	return newRealtimeHTTPFixtureOptions(t, maxDuration, translation, withPrice, false, nil, configureKey...)
}

func newRealtimeHTTPFixtureOptions(t *testing.T, maxDuration time.Duration, translation, withPrice, handshakeFailure bool, dialError error, configureKey ...func(*ent.APIKeyCreate)) (*ent.Client, func() (*websocket.Conn, *http.Response, error), *atomic.Int32, <-chan struct{}) {
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
	model := "gpt-realtime-2.1-mini"
	if translation {
		model = openairt.TranslationModel
	}
	row := db.Channel.Create().SetName("reconnect").SetType(channel.TypeOpenai).SetStatus(channel.StatusEnabled).SetBaseURL("https://api.openai.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-provider"}).SetSupportedModels([]string{model}).SetDefaultTestModel(model).SetSettings(&objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}).SaveX(seed)
	price := objects.ModelPrice{}
	for _, code := range objects.AllRealtimePriceItemCodes() {
		rate := decimal.NewFromInt(1)
		price.Items = append(price.Items, objects.ModelPriceItem{ItemCode: code, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &rate}})
	}
	if translation {
		rate := decimal.RequireFromString("0.034")
		price.Items = []objects.ModelPriceItem{{ItemCode: objects.PriceItemCodeRealtimeTranslationAudioSamples, Pricing: objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: &rate}}}
	}
	if withPrice {
		db.ChannelModelPrice.Create().SetChannelID(row.ID).SetModelID(model).SetPrice(price).SetReferenceID("test").SaveX(seed)
	}
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
		if handshakeFailure {
			http.Error(w, "synthetic upstream unavailable", http.StatusServiceUnavailable)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		c.SetReadLimit(rt.MaxMessageBytes)
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		if translation {
			serveTranslationFixture(r.Context(), c)
			return
		}
		_ = c.Write(r.Context(), websocket.MessageText, []byte(`{"type":"session.created","session":{"id":"s","type":"realtime","audio":{"input":{"format":{"type":"audio/pcm","rate":24000}},"output":{"format":{"type":"audio/pcm","rate":24000}}}}}`))
		for {
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if !withPrice && gjson.GetBytes(data, "type").String() == "session.update" {
				if gjson.GetBytes(data, "session.model").Exists() || gjson.GetBytes(data, "session.audio.input.transcription").Exists() {
					t.Error("gateway forwarded SDK model or independent transcription settings")
					return
				}
				ack := []byte(`{"type":"session.updated","session":` + gjson.GetBytes(data, "session").Raw + `}`)
				if c.Write(r.Context(), websocket.MessageText, ack) != nil {
					return
				}
			}
			if !withPrice && gjson.GetBytes(data, "type").String() == "response.create" {
				created := []byte(`{"type":"response.created","response":{"id":"unpriced-response"}}`)
				done := []byte(`{"type":"response.done","response":{"id":"unpriced-response","status":"completed","usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18,"input_token_details":{"text_tokens":11,"audio_tokens":0},"output_token_details":{"text_tokens":7,"audio_tokens":0}}}}`)
				if c.Write(r.Context(), websocket.MessageText, created) != nil || c.Write(r.Context(), websocket.MessageText, done) != nil {
					return
				}
			}
			select {
			case observed <- struct{}{}:
			default:
			}
		}
	}))
	t.Cleanup(upstream.Close)
	service := &orchestrator.RealtimeService{Transport: reconnectDialer{url: "ws" + strings.TrimPrefix(upstream.URL, "http"), err: dialError}, Channels: channels, Selector: orchestrator.NewDefaultSelector(channels, models, system), Usage: usage, Requests: requests, Auth: auth, Quota: biz.NewQuotaService(db, system)}
	h := &RealtimeHandlers{service: service, cache: cache, drained: make(chan struct{}), sessions: make(map[uint64]context.CancelFunc)}
	close(h.drained)
	router := gin.New()
	router.Use(middleware.WithEntClient(db))
	path := "/v1/realtime"
	config := rt.Config{MaxDuration: maxDuration, Limits: rt.Limits{APIKey: 1, ConnectionsPerMinute: 100}}
	handler := h.Handler(config, false)
	if translation {
		path += "/translations"
		handler = h.TranslationHandler(config)
	}
	router.GET(path, RealtimeBoundary(), middleware.WithIPBlocklist(system), middleware.WithStrictAPIKeyConfig(auth, &middleware.APIKeyConfig{Headers: []string{"Authorization"}, RequireBearer: true}), middleware.WithSource(request.SourceAPI), handler)
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
	url := "ws" + strings.TrimPrefix(server.URL, "http") + path + "?model=" + model
	dial := func() (*websocket.Conn, *http.Response, error) {
		dialCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c, resp, err := websocket.Dial(dialCtx, url, &websocket.DialOptions{
			HTTPHeader: http.Header{
				"Authorization":    []string{"Bearer " + key.Key},
				"X-Client-Name":    []string{"test-client"},
				"X-Client-Version": []string{"2.1"},
			},
		})
		if err == nil {
			c.SetReadLimit(rt.MaxMessageBytes)
		}
		return c, resp, err
	}
	return db, dial, &peak, observed
}

func TestRealtimeHTTPReconnectWhileAuditStillDraining(t *testing.T) {
	db, dial, peak, observed := newRealtimeReconnectFixture(t)
	first, _, err := dial()
	require.NoError(t, err)
	defer func() { _ = first.CloseNow() }()
	_, _, err = apiRealtimeReadWithTimeout(first, 5*time.Second)
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
	require.NoError(t, apiRealtimeWriteText(first, []byte(`{"type":"response.create"}`)))
	select {
	case <-observed:
	case <-time.After(3 * time.Second):
		_, rejected, readErr := apiRealtimeReadWithTimeout(first, time.Second)
		t.Fatalf("client event was not forwarded: %s %v", rejected, readErr)
	}
	require.NoError(t, first.CloseNow())
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("disconnect audit did not enter finalization")
	}
	next, response, err := dial()
	if next != nil {
		defer func() { _ = next.CloseNow() }()
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
	defer func() { _ = first.CloseNow() }()
	_, _, err = apiRealtimeReadWithTimeout(first, 5*time.Second)
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
	require.NoError(t, apiRealtimeWriteText(first, []byte(`{"type":"response.create"}`)))
	require.NoError(t, first.CloseNow())
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
		defer func() { _ = r.conn.CloseNow() }()
	}
	require.False(t, early, "one handshake must wait for the old transport, not reject or overbook")
	require.NoError(t, r.err, "the same single HTTP handshake must return 101, with no client retry")
	require.Equal(t, http.StatusSwitchingProtocols, r.response.StatusCode)
	require.Equal(t, int32(1), peak.Load())
}

func TestRealtimeHTTPLiveConcurrentSessionStillRejected(t *testing.T) {
	db, dial, peak, _ := newRealtimeReconnectFixture(t)
	first, _, err := dial()
	require.NoError(t, err)
	defer func() { _ = first.CloseNow() }()
	second, response, err := dial()
	if second != nil {
		defer func() { _ = second.CloseNow() }()
	}
	require.Error(t, err)
	require.NotNil(t, response)
	require.Equal(t, http.StatusTooManyRequests, response.StatusCode)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, "rate_limit_exceeded", gjson.GetBytes(body, "error.type").String())
	require.Equal(t, "rate_limit_exceeded", gjson.GetBytes(body, "error.code").String())
	require.Contains(t, gjson.GetBytes(body, "error.message").String(), "API key connections")
	require.Equal(t, int32(1), peak.Load(), "reconnect grace must not bypass concurrency")

	seed := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	denied := db.Request.Query().Where(request.StatusEQ(request.StatusFailed)).OnlyX(seed)
	require.Equal(t, "openai/realtime", denied.Format)
	require.Equal(t, "admission_denied", gjson.GetBytes(denied.ResponseBody, "error.error_code").String())
	require.Equal(t, "admission_denied", gjson.GetBytes(denied.RequestBody, "realtime.close_reason").String())
	require.NotNil(t, denied.MetricsLatencyMs)
	require.Zero(t, db.UsageLog.Query().CountX(seed), "denied admissions must not create usage or affect quotas")

	_ = first.CloseNow()
	reconnected, reconnectResponse, reconnectErr := dial()
	require.NoError(t, reconnectErr, "an audit row for a denied attempt must not retain an admission lease")
	require.Equal(t, http.StatusSwitchingProtocols, reconnectResponse.StatusCode)
	_ = reconnected.CloseNow()
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
		defer func() { _ = c.CloseNow() }()
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
	defer func() { _ = first.CloseNow() }()
	_, _, err = apiRealtimeReadWithTimeout(first, 3*time.Second)
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
	require.NoError(t, apiRealtimeWriteText(first, []byte(`{"type":"response.create"}`)))
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("audit INSERT did not reach the write boundary")
	}

	// The terminal diagnostic precedes the close handshake.
	_, frame, err := apiRealtimeReadWithTimeout(first, 3*time.Second)
	require.NoError(t, err)
	require.Equal(t, "internal_error", gjson.GetBytes(frame, "error.code").String())
	_, _, err = apiRealtimeReadWithTimeout(first, 3*time.Second)
	var closed websocket.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, websocket.StatusInternalError, closed.Code)
	select {
	case err = <-canceled:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("session deadline did not cancel the blocked audit INSERT")
	}

	second, response, err := dial()
	if second != nil {
		defer func() { _ = second.CloseNow() }()
	}
	require.NoError(t, err, "the same API key must immediately regain its lease")
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	require.Equal(t, int32(1), peak.Load())
}

func serveTranslationFixture(ctx context.Context, conn *websocket.Conn) {
	created := `{"type":"session.created","session":{"id":"s","type":"translation","model":"gpt-realtime-translate"}}`
	if conn.Write(ctx, websocket.MessageText, []byte(created)) != nil {
		return
	}
	for {
		_, event, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var response string
		switch gjson.GetBytes(event, "type").String() {
		case "session.update":
			response = `{"type":"session.updated","session":{"id":"s","type":"translation","model":"gpt-realtime-translate","audio":{"output":{"language":"es"}}}}`
		case "session.input_audio_buffer.append":
			response = `{"type":"session.output_audio.delta","delta":"AAA="}`
		case "session.close":
			if conn.Write(ctx, websocket.MessageText, []byte(`{"type":"session.output_transcript.delta","delta":"hola"}`)) != nil {
				return
			}
			_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"session.closed"}`))
			// Leave transport closure to the gateway after it drains session.closed.
			_, _, _ = conn.Read(ctx)
			return
		default:
			return
		}
		if conn.Write(ctx, websocket.MessageText, []byte(response)) != nil {
			return
		}
	}
}

func TestRealtimeHTTPTranslationLifecycleAndBilling(t *testing.T) {
	t.Run("graceful", func(t *testing.T) { testRealtimeHTTPTranslationBilling(t, true, true) })
	t.Run("disconnected", func(t *testing.T) { testRealtimeHTTPTranslationBilling(t, false, true) })
	t.Run("without-price", func(t *testing.T) { testRealtimeHTTPTranslationBilling(t, true, false) })
}

func testRealtimeHTTPTranslationBilling(t *testing.T, graceful, withPrice bool) {
	t.Helper()
	db, dial, _, _ := newRealtimeHTTPFixtureWithPrice(t, 0, true, withPrice)
	conn, response, err := dial()
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	expect := func(kind string) {
		t.Helper()
		_, data, err := apiRealtimeReadWithTimeout(conn, 5*time.Second)
		require.NoError(t, err)
		require.Equal(t, kind, gjson.GetBytes(data, "type").String(), "%s", data)
	}
	send := func(data string) {
		t.Helper()
		require.NoError(t, apiRealtimeWriteText(conn, []byte(data)))
	}
	expect("session.created")
	send(`{"type":"response.create"}`)
	expect("error")
	send(`{"type":"session.update","session":{"audio":{"output":{"language":"es"}}}}`)
	expect("session.updated")
	audio := base64.StdEncoding.EncodeToString(make([]byte, 48000))
	send(`{"type":"session.input_audio_buffer.append","audio":"` + audio + `"}`)
	expect("session.output_audio.delta")
	wantStatus := request.StatusFailed
	if graceful {
		send(`{"type":"session.close"}`)
		expect("session.output_transcript.delta")
		expect("session.closed")
		_, _, err = apiRealtimeReadWithTimeout(conn, 5*time.Second)
		require.Equal(t, websocket.StatusNormalClosure, websocket.CloseStatus(err), "%v", err)
		wantStatus = request.StatusCompleted
	} else {
		require.NoError(t, conn.CloseNow())
	}

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	require.Eventually(t, func() bool {
		row, err := db.Request.Query().Only(ctx)
		return err == nil && row.Status == wantStatus
	}, 3*time.Second, 10*time.Millisecond)
	usage := db.UsageLog.Query().OnlyX(ctx)
	require.Equal(t, openairt.TranslationModel, usage.ModelID)
	require.Zero(t, usage.TotalTokens)
	if withPrice {
		require.Len(t, usage.CostItems, 1)
		item := usage.CostItems[0]
		require.Equal(t, objects.PriceItemCodeRealtimeTranslationAudioSamples, item.ItemCode)
		require.EqualValues(t, 24000, item.Quantity)
		require.Equal(t, "0.0005666666666667", item.Subtotal.String())
		require.NotNil(t, usage.TotalCost)
	} else {
		require.Empty(t, usage.CostItems)
		require.Nil(t, usage.TotalCost)
	}
	row := db.Request.Query().OnlyX(ctx)
	require.Equal(t, "gateway_sent_pcm", gjson.GetBytes(row.ResponseBody, "metering_source").String())
	require.NotContains(t, string(row.RequestBody), audio)
}

func TestRealtimeHTTPDisconnectedInputStoresSessionAuditWithoutUsage(t *testing.T) {
	db, dial, _, observed := newRealtimeReconnectFixture(t)
	conn, _, err := dial()
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	_, _, err = apiRealtimeReadWithTimeout(conn, 3*time.Second)
	require.NoError(t, err)
	require.NoError(t, apiRealtimeWriteText(conn, []byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`)))
	select {
	case <-observed:
	case <-time.After(2 * time.Second):
		t.Fatal("provider did not receive input")
	}
	require.NoError(t, conn.CloseNow())

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	var row *ent.Request
	require.Eventually(t, func() bool {
		var err error
		row, err = db.Request.Query().Only(ctx)
		return err == nil && gjson.GetBytes(row.RequestBody, "realtime.close_code").Exists()
	}, 3*time.Second, 10*time.Millisecond)
	require.Equal(t, request.StatusFailed, row.Status)
	require.NotNil(t, row.MetricsLatencyMs)
	require.Equal(t, "test-client 2.1", row.UserAgent)
	require.Equal(t, "client_closed", gjson.GetBytes(row.ResponseBody, "error_code").String())
	require.Equal(t, "openai/realtime-ws", gjson.GetBytes(row.RequestBody, "upstream_format").String())
	require.EqualValues(t, 2, gjson.GetBytes(row.RequestBody, "realtime.audio_in_bytes").Int())
	require.True(t, gjson.GetBytes(row.RequestBody, "realtime.interrupted").Bool())
	require.Equal(t, 0, db.UsageLog.Query().CountX(ctx))
}

func TestRealtimeHTTPUpstreamHandshakeFailureIsAuditedWithoutHoldingLease(t *testing.T) {
	db, dial, peak, _ := newRealtimeHTTPFixtureOptions(t, 0, false, true, true, nil)
	for range 2 {
		conn, response, err := dial()
		if conn != nil {
			_ = conn.CloseNow()
		}
		require.Error(t, err)
		require.NotNil(t, response)
		require.Equal(t, http.StatusBadGateway, response.StatusCode, "a previous handshake audit must not turn reconnect into 429")
		response.Body.Close()
	}
	seed := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	rows := db.Request.Query().Where(request.StatusEQ(request.StatusFailed)).AllX(seed)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, "upstream_handshake_failed", gjson.GetBytes(row.ResponseBody, "error.error_code").String())
		require.Equal(t, "upstream_handshake_failed", gjson.GetBytes(row.RequestBody, "realtime.close_reason").String())
		require.True(t, gjson.GetBytes(row.RequestBody, "realtime.close_code").Type == gjson.Null)
		require.NotNil(t, row.MetricsLatencyMs)
	}
	require.Zero(t, db.UsageLog.Query().CountX(seed), "handshake failures do not create billable usage")
	require.Zero(t, peak.Load())
}

func TestRealtimeHTTPHandshakeTimeoutReturns504AndReleasesLease(t *testing.T) {
	db, dial, peak, _ := newRealtimeHTTPFixtureOptions(t, 0, false, true, false,
		&rt.HandshakeError{Cause: context.DeadlineExceeded})
	for range 2 {
		conn, response, err := dial()
		if conn != nil {
			_ = conn.CloseNow()
		}
		require.Error(t, err)
		require.NotNil(t, response)
		require.Equal(t, http.StatusGatewayTimeout, response.StatusCode)
		body, readErr := io.ReadAll(response.Body)
		require.NoError(t, readErr)
		require.NoError(t, response.Body.Close())
		require.Equal(t, "upstream_handshake_timeout", gjson.GetBytes(body, "error.error_code").String())
		require.Contains(t, gjson.GetBytes(body, "error.message").String(), "timeout")
	}
	seed := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	rows := db.Request.Query().Where(request.StatusEQ(request.StatusFailed)).AllX(seed)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, "upstream_handshake_timeout", gjson.GetBytes(row.ResponseBody, "error.error_code").String())
	}
	require.Zero(t, db.UsageLog.Query().CountX(seed))
	require.Zero(t, peak.Load())
}

func TestRealtimeHTTPUnpricedOpenAIRecordsUsageWithoutCost(t *testing.T) {
	db, dial, _, _ := newRealtimeHTTPFixtureWithPrice(t, 0, false, false)
	conn, response, err := dial()
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	_, created, err := apiRealtimeReadWithTimeout(conn, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, "session.created", gjson.GetBytes(created, "type").String())
	require.NoError(t, apiRealtimeWriteText(conn, []byte(`{"type":"response.cancel","event_id":"empty-cancel"}`)))
	_, canceled, err := apiRealtimeReadWithTimeout(conn, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, "response_cancel_not_active", gjson.GetBytes(canceled, "error.code").String())
	require.Equal(t, "empty-cancel", gjson.GetBytes(canceled, "error.event_id").String())
	require.NoError(t, apiRealtimeWriteText(conn, []byte(`{"type":"session.update","event_id":"wrong-model","session":{"type":"realtime","model":"gpt-realtime-2.1"}}`)))
	_, rejected, err := apiRealtimeReadWithTimeout(conn, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, "model_switch_not_allowed", gjson.GetBytes(rejected, "error.code").String())
	require.Equal(t, "wrong-model", gjson.GetBytes(rejected, "error.event_id").String())

	update := []byte(`{"type":"session.update","session":{"type":"realtime","model":"gpt-realtime-2.1-mini","audio":{"input":{"format":{"type":"audio/pcm","rate":24000},"transcription":{"model":"gpt-4o-mini-transcribe"},"turn_detection":null}}}}`)
	require.NoError(t, apiRealtimeWriteText(conn, update))
	require.NoError(t, apiRealtimeWriteText(conn, []byte(`{"type":"session.update","session":{"type":"realtime","model":"gpt-realtime-2.1-mini","tracing":"auto"}}`)))
	_, updated, err := apiRealtimeReadWithTimeout(conn, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, "session.updated", gjson.GetBytes(updated, "type").String())
	require.EqualValues(t, 24000, gjson.GetBytes(updated, "session.audio.input.format.rate").Int())
	_, updated, err = apiRealtimeReadWithTimeout(conn, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, "session.updated", gjson.GetBytes(updated, "type").String())
	require.Equal(t, "auto", gjson.GetBytes(updated, "session.tracing").String())
	require.NoError(t, apiRealtimeWriteText(conn, []byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`)))
	require.NoError(t, apiRealtimeWriteText(conn, []byte(`{"type":"input_audio_buffer.commit"}`)))
	require.NoError(t, apiRealtimeWriteText(conn, []byte(`{"type":"response.create"}`)))
	for {
		_, event, err := apiRealtimeReadWithTimeout(conn, 5*time.Second)
		require.NoError(t, err)
		if gjson.GetBytes(event, "type").String() == "response.done" {
			break
		}
	}
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	usage := db.UsageLog.Query().OnlyX(ctx)
	require.EqualValues(t, 18, usage.TotalTokens)
	require.Nil(t, usage.TotalCost)
	require.Empty(t, usage.CostItems)
	require.Empty(t, usage.CostPriceReferenceID)
}

func TestRealtimeHTTPTranslationHonorsModelPermissions(t *testing.T) {
	_, dial, peak, _ := newRealtimeHTTPFixture(t, 0, true, func(key *ent.APIKeyCreate) {
		key.SetProfiles(&objects.APIKeyProfiles{
			ActiveProfile: "voice-only",
			Profiles:      []objects.APIKeyProfile{{Name: "voice-only", ModelIDs: []string{"gpt-realtime-2.1"}}},
		})
	})
	conn, response, err := dial()
	require.Error(t, err)
	require.Nil(t, conn)
	require.NotNil(t, response)
	defer response.Body.Close()
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	require.Zero(t, peak.Load(), "model permissions must be checked before the provider handshake")
}
