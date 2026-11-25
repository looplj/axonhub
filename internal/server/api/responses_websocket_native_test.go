// 职责: 验证 Responses WebSocket 原生传输的消息大小、握手头与来源校验。
// 关联文件:
//
//	internal/server/api/responses_websocket.go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResponsesWebSocketNativeLargeMessage(t *testing.T) {
	input := strings.Repeat("x", 64<<10)
	server := newResponsesWebSocketTestServer(t, func(_ context.Context, request *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		var payload struct{ Input string }
		if err := json.Unmarshal(request.Body, &payload); err != nil {
			return orchestrator.ChatCompletionResult{}, err
		}
		body, err := json.Marshal(map[string]string{"status": "completed", "output": payload.Input})
		return orchestrator.ChatCompletionResult{ChatCompletion: &httpclient.Response{Body: body}}, err
	}, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
	require.NoError(t, err)
	defer conn.CloseNow()
	conn.SetReadLimit(128 << 20)
	message, err := json.Marshal(map[string]string{"type": "response.create", "model": "test", "input": input})
	require.NoError(t, err)
	require.NoError(t, conn.Write(ctx, websocket.MessageText, message))
	_, message, err = conn.Read(ctx)
	require.NoError(t, err)
	var event struct {
		Type     string
		Response struct{ Output string }
	}
	require.NoError(t, json.Unmarshal(message, &event))
	require.Equal(t, "response.completed", event.Type)
	require.Equal(t, input, event.Response.Output)
}

func TestResponsesWebSocketNativeHandshake(t *testing.T) {
	router := gin.New()
	router.GET("/v1/responses", func(c *gin.Context) {
		c.Header("X-Request-Id", "synthetic-request")
		serveResponsesWebSocket(c, time.Second, nil, nil)
	})
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for _, test := range []struct {
		name   string
		origin string
		status int
	}{
		{name: "no origin", status: http.StatusSwitchingProtocols},
		{name: "same origin", origin: server.URL, status: http.StatusSwitchingProtocols},
		{name: "cross origin", origin: "https://untrusted.example", status: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			headers := make(http.Header)
			if test.origin != "" {
				headers.Set("Origin", test.origin)
			}
			conn, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", &websocket.DialOptions{HTTPHeader: headers})
			require.NotNil(t, response)
			require.Equal(t, test.status, response.StatusCode)
			require.Equal(t, "synthetic-request", response.Header.Get("X-Request-ID"))
			if test.status == http.StatusSwitchingProtocols {
				require.NoError(t, err)
				require.NoError(t, conn.CloseNow())
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestResponsesWebSocketUpgradeTokens(t *testing.T) {
	server := newResponsesWebSocketTestServer(t, nil, nil)
	for _, upgrade := range [][]string{{"h2c, websocket"}, {"h2c", "websocket"}} {
		t.Run(strings.Join(upgrade, ";"), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/v1/responses", nil)
			require.NoError(t, err)
			request.Header = http.Header{
				"Connection":            {"keep-alive, Upgrade"},
				"Upgrade":               upgrade,
				"Sec-Websocket-Version": {"13"},
				"Sec-Websocket-Key":     {"dGhlIHNhbXBsZSBub25jZQ=="},
			}
			response, err := http.DefaultClient.Do(request)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
		})
	}
}
