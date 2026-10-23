package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func responsesIntegrationServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	var active sync.WaitGroup
	var server *httptest.Server
	t.Cleanup(func() {
		server.Close()
		active.Wait()
		conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
		if conn != nil {
			require.NoError(t, conn.Close())
		}
		require.Error(t, err, "owned listener still accepts connections")
		t.Log("teardown: listener closed; HTTP handler barrier drained")
	})
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		active.Add(1)
		defer active.Done()
		handler.ServeHTTP(w, r)
	}))
	return server
}

func responsesIntegrationGateway(t *testing.T, client *ent.Client, handlers *OpenAIHandlers, project *ent.Project) *httptest.Server {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := authz.WithTestBypass(ent.NewContext(c.Request.Context(), client))
		ctx = contexts.WithAPIKey(ctx, &ent.APIKey{ID: 101, ProjectID: project.ID})
		ctx = shared.WithSessionScope(ctx, "api_key:101")
		c.Request = c.Request.WithContext(contexts.WithProjectID(ctx, project.ID))
		c.Next()
	})
	router.POST("/v1/responses", handlers.CreateResponse)
	router.GET("/v1/responses", handlers.CreateResponseWebSocket(5*time.Second))
	router.POST("/v1/images/generations", handlers.CreateImage)
	router.POST("/v1/images/edits", handlers.CreateImageEdit)
	return responsesIntegrationServer(t, router)
}

func responsesIntegrationPost(t *testing.T, server *httptest.Server, path, body string, deadlines ...time.Duration) (int, []byte) {
	t.Helper()
	deadline := 10 * time.Second
	if len(deadlines) > 0 {
		deadline = deadlines[0]
	}
	ctx, cancel := context.WithTimeout(t.Context(), deadline)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	data, err := readRequestBody(&http.Request{Body: response.Body})
	require.NoError(t, err)
	return response.StatusCode, data
}

func responsesIntegrationDial(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	t.Cleanup(func() {
		if err := conn.Close(); !errors.Is(err, net.ErrClosed) {
			require.NoError(t, err)
		}
	})
	return conn
}

func responsesIntegrationTurn(t *testing.T, conn *websocket.Conn, body string) []byte {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(body)))
	for {
		_, event, err := conn.ReadMessage()
		require.NoError(t, err)
		switch gjson.GetBytes(event, "type").String() {
		case "response.completed", "response.failed", "error":
			return event
		}
	}
}

func responsesIntegrationAwait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("owned upstream connection/handler did not close")
	}
}
