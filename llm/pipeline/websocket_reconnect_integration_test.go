package pipeline_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestPipelineWebSocketRecoveredReadFailureDoesNotSendThirdAttempt(t *testing.T) {
	ctx, cancel := context.WithTimeout(shared.WithSessionID(shared.WithSessionScope(t.Context(), "scope"), "session"), 5*time.Second)
	defer cancel()
	var upgrades, messages atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close()
		if !assert.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second))) {
			return
		}
		var payload map[string]any
		if !assert.NoError(t, conn.ReadJSON(&payload)) {
			return
		}
		messages.Add(1)
		if upgrade == 1 {
			if !assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.completed","response":{"id":"resp_seed","object":"response","model":"gpt-5","status":"completed","output":[]}}`))) {
				return
			}
			if !assert.NoError(t, conn.ReadJSON(&payload)) {
				return
			}
			messages.Add(1)
			return
		}
		if !assert.NotContains(t, payload, "previous_response_id") {
			return
		}
		input, ok := payload["input"].([]any)
		if !assert.True(t, ok) || !assert.Len(t, input, 2) {
			return
		}
	}))
	defer server.Close()
	inbound := responses.NewInboundTransformer()
	real, err := responses.NewOutboundTransformer(server.URL+"##", "test-key")
	require.NoError(t, err)
	executor := responses.NewWebSocketExecutor(nil)
	defer executor.Close()
	seed, err := inbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{"model":"gpt-5","stream":true,"input":[{"role":"user","content":"one"}]}`)})
	require.NoError(t, err)
	seedRequest, err := real.TransformRequest(ctx, seed)
	require.NoError(t, err)
	seedRequest, err = httpclient.FinalizeAuthHeaders(seedRequest)
	require.NoError(t, err)
	seedStream, err := executor.DoStream(ctx, seedRequest)
	require.NoError(t, err)
	require.True(t, seedStream.Next())
	require.NoError(t, seedStream.Err())
	prepareCalls := 0
	outbound := &retryTestOutbound{real: real, prepareCalls: &prepareCalls, canRetry: func(error) bool { return true }}
	p := pipeline.NewFactory(executor).Pipeline(inbound, outbound, pipeline.WithRetry(0, 2, 0))
	_, err = p.Process(ctx, &httpclient.Request{Body: []byte(`{"model":"gpt-5","stream":true,"input":[{"role":"user","content":"one"},{"role":"user","content":"two"}]}`)})
	require.ErrorIs(t, err, shared.ErrResponsesWebSocketReconnectFailed)
	require.Zero(t, prepareCalls)
	require.Equal(t, int32(2), upgrades.Load())
	require.Equal(t, int32(3), messages.Load())
}
