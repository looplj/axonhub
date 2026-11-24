// 职责: 验证 coder 上游传输的大消息与连接复用取消语义。
// 关联文件:
//
//	llm/transformer/openai/responses/websocket_executor.go
//	llm/transformer/openai/responses/websocket_dial.go
package responses

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestCoderWebSocketPreservesUnlimitedUpstreamMessages(t *testing.T) {
	delta := strings.Repeat("x", 128<<10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if !assertCoderPeer(t, err) {
			return
		}
		defer conn.CloseNow()
		_, _, err = conn.Read(r.Context())
		if !assertCoderPeer(t, err) {
			return
		}
		if !assertCoderPeer(t, wsjson.Write(r.Context(), conn, map[string]string{"type": "response.output_text.delta", "delta": delta})) {
			return
		}
		assertCoderPeer(t, wsjson.Write(r.Context(), conn, map[string]any{
			"type": "response.completed", "response": map[string]any{"id": "large", "status": "completed", "output": []any{}},
		}))
	}))
	defer server.Close()
	executor := NewWebSocketExecutor(nil)
	defer executor.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := executor.DoStream(ctx, &httpclient.Request{URL: server.URL, Body: []byte(`{"model":"test","input":[]}`)})
	require.NoError(t, err)
	defer stream.Close()
	require.True(t, stream.Next())
	require.Contains(t, string(stream.Current().Data), delta)
	require.True(t, stream.Next())
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
}

func TestCoderWebSocketCompletedLeaseSurvivesOldContextCancellation(t *testing.T) {
	accepted := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if !assertCoderPeer(t, err) {
			return
		}
		defer conn.CloseNow()
		accepted <- struct{}{}
		for range 2 {
			_, _, err := conn.Read(r.Context())
			if !assertCoderPeer(t, err) {
				return
			}
			if !assertCoderPeer(t, wsjson.Write(r.Context(), conn, map[string]any{
				"type": "response.completed", "response": map[string]any{"id": "reuse", "status": "completed", "output": []any{}},
			})) {
				return
			}
		}
	}))
	defer server.Close()
	executor := NewWebSocketExecutor(nil)
	defer executor.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = shared.WithSessionID(ctx, "coder-reuse")
	ctx = shared.WithSessionScope(ctx, "coder-test")
	firstCtx, cancelFirst := context.WithCancel(ctx)
	defer cancelFirst()
	request := &httpclient.Request{URL: server.URL, Body: []byte(`{"model":"test","input":[]}`)}
	first, err := executor.DoStream(firstCtx, request)
	require.NoError(t, err)
	require.True(t, first.Next())
	require.False(t, first.Next())
	require.NoError(t, first.Err())
	second, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	defer second.Close()
	cancelFirst()
	require.False(t, first.Next())
	require.True(t, second.Next())
	require.False(t, second.Next())
	require.NoError(t, second.Err())
	require.Len(t, accepted, 1)
	require.NoError(t, first.Err())
}

func TestCoderWebSocketTerminalCancellationEvictsLease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if !assertCoderPeer(t, err) {
			return
		}
		defer conn.CloseNow()
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("canceled=%t", canceled), func(t *testing.T) {
			executor := NewWebSocketExecutor(nil)
			defer executor.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			ctx = shared.WithSessionID(ctx, "terminal-cancel")
			ctx = shared.WithSessionScope(ctx, "coder-test")
			lease, err := executor.acquire(ctx, &httpclient.Request{}, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
			require.NoError(t, err)
			require.NotNil(t, lease.pooled)
			streamCtx, _, stopIdle := httpclient.NewStreamIdleContext(httpclient.WithStreamIdleTimeout(ctx, time.Second))
			stream := &webSocketStream{ctx: streamCtx, lease: lease, done: make(chan struct{}), stopIdle: stopIdle}
			stream.markTerminal()
			if canceled {
				cancel()
			}
			stream.finish(false)
			require.NoError(t, stream.Err(), "completed responses retain success despite cancellation")
			require.Equal(t, canceled, lease.pooled.isClosed(), "cancellation before lease release must retire the socket")
			if canceled {
				require.Empty(t, executor.pool)
			} else {
				require.Len(t, executor.pool, 1, "cleanup cancellation must preserve healthy pooled sockets")
			}
		})
	}
}

func assertCoderPeer(t *testing.T, err error) bool {
	t.Helper()
	if err != nil {
		t.Errorf("websocket peer: %v", err)
		return false
	}
	return true
}
