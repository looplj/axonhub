package api

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func newResponsesWebSocketConvertedDeliveryStream(t *testing.T) *responsesWebSocketDeliveryStream {
	t.Helper()
	normalized := streams.SliceStream([]*llm.Response{
		{Object: "chat.completion.chunk", ID: "resp_delivery", Model: "gpt-test", Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}}}, Usage: &llm.Usage{}},
		{Object: "chat.completion.chunk", ID: "resp_delivery", Model: "gpt-test", Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{}, FinishReason: lo.ToPtr("stop")}}, Usage: &llm.Usage{}},
		{Object: "chat.completion.chunk", ID: "resp_delivery", Model: "gpt-test", Usage: &llm.Usage{PromptTokens: 12, CompletionTokens: 34, TotalTokens: 46}},
	})
	converted, err := responses.NewInboundTransformer().TransformStream(t.Context(), normalized)
	require.NoError(t, err)
	source := newResponsesWebSocketDeliveryStream("response.completed")
	source.Stream = converted
	return source
}

func TestStreamDeliveryWebSocketConvertedTerminalWriteFailure(t *testing.T) {
	// Given: the real Responses converter with placeholder zeros and a final 12/34 usage chunk.
	source := newResponsesWebSocketConvertedDeliveryStream(t)
	terminalAttempted := false
	transport := &responsesWebSocketDeliveryWriter{write: func(data []byte) error {
		require.False(t, source.DeliveryAcknowledged())
		if gjson.GetBytes(data, "type").String() == "response.completed" {
			terminalAttempted = true
			require.Equal(t, int64(12), gjson.GetBytes(data, "response.usage.input_tokens").Int())
			require.Equal(t, int64(34), gjson.GetBytes(data, "response.usage.output_tokens").Int())
			return io.ErrClosedPipe
		}
		return nil
	}}
	// When: converted output reaches its queued terminal but the terminal write fails.
	err := writeResponsesWebSocketResult(t.Context(), &responsesWebSocketWriter{conn: transport}, orchestrator.ChatCompletionResult{ChatCompletionStream: source}, nil, "lane-converted")
	// Then: a fully generated terminal with authoritative usage cannot prove delivery.
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.True(t, terminalAttempted)
	require.False(t, source.DeliveryAcknowledged())
}

func TestStreamDeliveryWebSocketConvertedLoopback(t *testing.T) {
	// Given: the real converter, API adapter, priming decorator, and loopback WebSocket.
	source := newResponsesWebSocketConvertedDeliveryStream(t)
	process := func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		adapted, _ := newResponsesStreamAdapter(ctx, source, nil)
		return orchestrator.ChatCompletionResult{ChatCompletionStream: primeCodexTurnStateHeader(nil, adapted)}, nil
	}
	server := newResponsesWebSocketTestServer(t, process, nil)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	// When: the client reads the converted terminal then disconnects after acknowledgement.
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"lane-converted","model":"gpt-test","input":"hello"}`)))
	var terminal []byte
	for {
		_, data, err := conn.ReadMessage()
		require.NoError(t, err)
		if gjson.GetBytes(data, "type").String() == "response.completed" {
			terminal = data
			break
		}
	}
	select {
	case <-source.closedC:
	case <-time.After(5 * time.Second):
		t.Fatal("converted turn did not finish")
	}
	require.NoError(t, conn.Close())
	// Then: final usage ordering and the original observer survive the actual wrapper path.
	require.True(t, source.DeliveryAcknowledged())
	require.Equal(t, "lane-converted", gjson.GetBytes(terminal, "stream_id").String())
	require.Equal(t, int64(12), gjson.GetBytes(terminal, "response.usage.input_tokens").Int())
	require.Equal(t, int64(34), gjson.GetBytes(terminal, "response.usage.output_tokens").Int())
	t.Log("real converter -> Responses adapter -> primed stream -> socket terminal (12/34) -> acknowledgement -> disconnect")
}
