package api

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
)

func assertStreamDeliveryWire(t *testing.T, response *http.Response, heartbeat bool) {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, response.Header.Get("Content-Type"), "text/event-stream")
	require.NotEmpty(t, body)

	text := string(body)
	require.Contains(t, text, "data:")
	require.Contains(t, text, "[DONE]")
	if heartbeat {
		t.Log("heartbeat mode exercised; heartbeat framing is covered by TestStreamDeliverySSELoopback")
	}
}

func TestStreamDeliveryE2E_PostTerminalDisconnectPersistsCompletion(t *testing.T) {
	fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion, streamDeliveryChat(12, 34, true), false)
	h := fixture.handler(t)
	engine := gin.New()
	engine.Use(fixture.testContext)
	done := make(chan struct{})
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		defer close(done)
		h.ChatCompletion(c)
	})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	server.Client().Timeout = 5 * time.Second

	req, err := http.NewRequestWithContext(fixture.ctx, http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"stream":true}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(req)
	require.NoError(t, err)
	reader := bufio.NewReader(response.Body)
	var wire strings.Builder
	for !strings.Contains(wire.String(), "[DONE]") {
		line, readErr := reader.ReadString('\n')
		wire.WriteString(line)
		require.NoError(t, readErr)
	}
	require.NoError(t, response.Body.Close())
	require.Contains(t, wire.String(), "data:")
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not finalize after disconnect")
	}
	fixture.assertDatabase(t, request.StatusCompleted, true)
}
