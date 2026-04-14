package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRequestPreviewHandlers_ReplayOnConnect(t *testing.T) {
	setup := newRequestPreviewTestSetup(t)
	defer biz.DefaultStreamPreviewRegistry.Unregister(biz.RequestKey(setup.req.ID))

	buffer := biz.NewChunkBuffer()
	buffer.Append(&httpclient.StreamEvent{Type: "message", Data: []byte(`{"index":1}`)})
	buffer.Append(&httpclient.StreamEvent{Type: "message", Data: llm.DoneStreamEvent.Data})
	buffer.Close()
	biz.DefaultStreamPreviewRegistry.RegisterBuffer(biz.RequestKey(setup.req.ID), buffer)

	resp := performPreviewRequest(t, setup.router, setup.req.ID)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	require.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	require.Equal(t, "keep-alive", resp.Header.Get("Connection"))

	reader := bufio.NewReader(resp.Body)
	firstEvent := readSSEEvent(t, reader)
	require.Equal(t, "preview.replay", firstEvent.Event)
	require.JSONEq(t, `{"index":1}`, firstEvent.Data)

	secondEvent := readSSEEvent(t, reader)
	require.Equal(t, "preview.completed", secondEvent.Event)
	require.JSONEq(t, `{"status":"completed"}`, secondEvent.Data)
}

func TestRequestPreviewHandlers_IncrementalDeliveryAfterReplay(t *testing.T) {
	setup := newRequestPreviewTestSetup(t)
	defer biz.DefaultStreamPreviewRegistry.Unregister(biz.RequestKey(setup.req.ID))

	buffer := biz.NewChunkBuffer()
	buffer.Append(&httpclient.StreamEvent{Type: "message", Data: []byte(`{"index":1}`)})
	biz.DefaultStreamPreviewRegistry.RegisterBuffer(biz.RequestKey(setup.req.ID), buffer)

	server := httptest.NewServer(setup.router)
	defer server.Close()

	resp, err := server.Client().Get(fmt.Sprintf("%s/admin/requests/%d/preview", server.URL, setup.req.ID))
	require.NoError(t, err)
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	replayEvent := readSSEEvent(t, reader)
	require.Equal(t, "preview.replay", replayEvent.Event)
	require.JSONEq(t, `{"index":1}`, replayEvent.Data)

	buffer.Append(&httpclient.StreamEvent{Type: "message", Data: []byte(`{"index":2}`)})
	buffer.Append(&httpclient.StreamEvent{Type: "message", Data: llm.DoneStreamEvent.Data})
	buffer.Close()

	incrementalEvent := readSSEEvent(t, reader)
	require.Equal(t, "preview.chunk", incrementalEvent.Event)
	require.JSONEq(t, `{"index":2}`, incrementalEvent.Data)

	completedEvent := readSSEEvent(t, reader)
	require.Equal(t, "preview.completed", completedEvent.Event)
	require.JSONEq(t, `{"status":"completed"}`, completedEvent.Data)
}

func TestRequestPreviewHandlers_WaitsForFirstChunkWhenProcessing(t *testing.T) {
	setup := newRequestPreviewTestSetup(t)
	defer biz.DefaultStreamPreviewRegistry.Unregister(biz.RequestKey(setup.req.ID))

	server := httptest.NewServer(setup.router)
	defer server.Close()

	resp, err := server.Client().Get(fmt.Sprintf("%s/admin/requests/%d/preview", server.URL, setup.req.ID))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")

	buffer := biz.DefaultStreamPreviewRegistry.GetBuffer(biz.RequestKey(setup.req.ID))
	require.NotNil(t, buffer)

	buffer.Append(&httpclient.StreamEvent{Type: "message", Data: []byte(`{"index":1}`)})
	buffer.Append(&httpclient.StreamEvent{Type: "message", Data: llm.DoneStreamEvent.Data})
	buffer.Close()

	reader := bufio.NewReader(resp.Body)
	firstEvent := readSSEEvent(t, reader)
	require.Equal(t, "preview.chunk", firstEvent.Event)
	require.JSONEq(t, `{"index":1}`, firstEvent.Data)

	completedEvent := readSSEEvent(t, reader)
	require.Equal(t, "preview.completed", completedEvent.Event)
	require.JSONEq(t, `{"status":"completed"}`, completedEvent.Data)
}

func TestRequestPreviewHandlers_CorrectHeaders(t *testing.T) {
	setup := newRequestPreviewTestSetup(t)
	defer biz.DefaultStreamPreviewRegistry.Unregister(biz.RequestKey(setup.req.ID))

	buffer := biz.NewChunkBuffer()
	buffer.Append(&httpclient.StreamEvent{Type: "message", Data: []byte(`{"index":1}`)})
	buffer.Close()
	biz.DefaultStreamPreviewRegistry.RegisterBuffer(biz.RequestKey(setup.req.ID), buffer)

	resp := performPreviewRequest(t, setup.router, setup.req.ID)
	defer resp.Body.Close()

	require.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	require.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	require.Equal(t, "keep-alive", resp.Header.Get("Connection"))
}

func TestRequestPreviewHandlers_FallbackToStaticFetchForCompletedRequests(t *testing.T) {
	setup := newRequestPreviewTestSetup(t)

	_, err := setup.client.Request.UpdateOneID(setup.req.ID).
		SetStatus(request.StatusCompleted).
		SetResponseChunks([]objects.JSONRawMessage{objects.JSONRawMessage(`{"persisted":true}`)}).
		Save(setup.ctx)
	require.NoError(t, err)

	resp := performPreviewRequest(t, setup.router, setup.req.ID)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	var body RequestPreviewFallbackResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "static-fetch", body.Mode)
	require.Len(t, body.ResponseChunks, 1)
	require.JSONEq(t, `{"persisted":true}`, string(body.ResponseChunks[0]))
}

type requestPreviewTestSetup struct {
	client *ent.Client
	ctx    context.Context
	router *gin.Engine
	req    *ent.Request
}

func newRequestPreviewTestSetup(t *testing.T) requestPreviewTestSetup {
	t.Helper()
	gin.SetMode(gin.TestMode)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	channelService := biz.NewChannelServiceForTest(client)
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)
	dataStorageService := &biz.DataStorageService{
		AbstractService: &biz.AbstractService{},
		SystemService:   systemService,
		Cache:           xcache.NewFromConfig[ent.DataStorage](xcache.Config{Mode: xcache.ModeMemory}),
	}
	requestService := biz.NewRequestService(client, systemService, usageLogService, dataStorageService)
	handlers := NewRequestPreviewHandlers(RequestPreviewHandlersParams{RequestService: requestService})

	project, err := client.Project.Create().SetName("p1").SetDescription("d").Save(ctx)
	require.NoError(t, err)

	reqRow, err := client.Request.Create().
		SetProjectID(project.ID).
		SetModelID("m1").
		SetFormat("openai/chat.completion").
		SetSource("api").
		SetStatus(request.StatusProcessing).
		SetStream(true).
		SetClientIP("").
		SetRequestHeaders(objects.JSONRawMessage(`{}`)).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		Save(ctx)
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		requestCtx := ent.NewContext(c.Request.Context(), client)
		requestCtx = contexts.WithUser(requestCtx, &ent.User{ID: 1, IsOwner: true})
		requestCtx = contexts.WithProjectID(requestCtx, project.ID)
		c.Request = c.Request.WithContext(requestCtx)
		c.Next()
	})
	router.GET("/admin/requests/:request_id/preview", handlers.PreviewRequest)

	return requestPreviewTestSetup{
		client: client,
		ctx:    ctx,
		router: router,
		req:    reqRow,
	}
}

func performPreviewRequest(t *testing.T, router http.Handler, requestID int) *http.Response {
	t.Helper()
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	resp, err := server.Client().Get(fmt.Sprintf("%s/admin/requests/%d/preview", server.URL, requestID))
	require.NoError(t, err)
	return resp
}

type sseEvent struct {
	Event string
	Data  string
}

func readSSEEvent(t *testing.T, reader *bufio.Reader) sseEvent {
	t.Helper()

	type result struct {
		event sseEvent
		err   error
	}

	ch := make(chan result, 1)
	go func() {
		var event sseEvent
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}

			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				ch <- result{event: event}
				return
			}
			if strings.HasPrefix(line, "event:") {
				event.Event = strings.TrimPrefix(line, "event:")
			}
			if strings.HasPrefix(line, "data:") {
				event.Data = strings.TrimPrefix(line, "data:")
			}
		}
	}()

	select {
	case result := <-ch:
		require.NoError(t, result.err)
		return result.event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE event")
		return sseEvent{}
	}
}
