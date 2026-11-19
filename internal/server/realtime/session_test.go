// 职责: 验证实时会话确认、并发转换、取消与双向连接清理。
// 关联文件:
//
//	internal/server/realtime/session.go
package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func socketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if e == nil {
			accepted <- c
		}
	}))
	t.Cleanup(srv.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)
	server := <-accepted
	t.Cleanup(func() { client.Close(); server.Close() })
	return client, server
}
func TestSessionBidirectionalAndCancellation(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, provider := socketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- Run(ctx, downstream, upstream, Options{IdleTimeout: time.Second, MaxDuration: time.Minute}, Hooks{})
	}()
	require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`)))
	_, data, err := provider.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(data), "append")
	require.NoError(t, provider.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created"}`)))
	_, data, err = client.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(data), "response.created")
	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("session goroutines did not exit")
	}
	provider.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err = provider.ReadMessage()
	require.Error(t, err)
}
func TestSessionDoesNotForwardRejectedEvent(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, provider := socketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, downstream, upstream, Options{IdleTimeout: time.Second}, Hooks{Client: func([]byte) ([]byte, error) { return nil, ErrInvalidEvent }})
	}()
	require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"type":"invalid"}`)))
	client.SetReadDeadline(time.Now().Add(time.Second))
	_, data, err := client.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(data), "invalid_request_error")
	provider.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	_, _, err = provider.ReadMessage()
	require.Error(t, err)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("did not stop")
	}
}

func TestSessionClosesPeerOnEitherDisconnect(t *testing.T) {
	for _, upstreamDisconnect := range []bool{false, true} {
		t.Run(map[bool]string{false: "downstream", true: "upstream"}[upstreamDisconnect], func(t *testing.T) {
			client, downstream := socketPair(t)
			upstream, provider := socketPair(t)
			done := make(chan error, 1)
			go func() {
				done <- Run(context.Background(), downstream, upstream, Options{IdleTimeout: time.Second}, Hooks{})
			}()
			peer := provider
			if upstreamDisconnect {
				provider.Close()
				peer = client
			} else {
				client.Close()
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("disconnected peer leaked workers")
			}
			peer.SetReadDeadline(time.Now().Add(time.Second))
			_, _, err := peer.ReadMessage()
			require.Error(t, err)
		})
	}
}
func TestSessionEnforcesMessageAndDurationBounds(t *testing.T) {
	t.Run("message", func(t *testing.T) {
		client, downstream := socketPair(t)
		upstream, _ := socketPair(t)
		done := make(chan error, 1)
		go func() { done <- Run(context.Background(), downstream, upstream, Options{}, Hooks{}) }()
		_ = client.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("a", MaxMessageBytes+1)))
		select {
		case err := <-done:
			require.Error(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("oversize message not rejected")
		}
	})
	t.Run("duration", func(t *testing.T) {
		_, downstream := socketPair(t)
		upstream, _ := socketPair(t)
		start := time.Now()
		err := Run(context.Background(), downstream, upstream, Options{MaxDuration: 30 * time.Millisecond}, Hooks{})
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Less(t, time.Since(start), time.Second)
	})
}
func TestSessionSlowClientBoundedBackpressure(t *testing.T) {
	_, downstream := socketPair(t)
	upstream, provider := socketPair(t)
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), downstream, upstream, Options{QueueSize: 1, WriteTimeout: 50 * time.Millisecond, MaxDuration: 2 * time.Second}, Hooks{})
	}()
	payload := []byte(strings.Repeat("x", 200<<10))
	_ = provider.SetWriteDeadline(time.Now().Add(time.Second))
	for i := 0; i < 128; i++ {
		if err := provider.WriteMessage(websocket.TextMessage, payload); err != nil {
			break
		}
	}
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("slow consumer retained session")
	}
}

func TestSessionFastBurstUsesBoundedBackpressureWithoutDropping(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, provider := socketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic: %T", r)
			}
		}()
		done <- Run(ctx, downstream, upstream, Options{QueueSize: 1}, Hooks{})
	}()
	sent := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				sent <- fmt.Errorf("panic: %T", r)
			}
		}()
		for i := 0; i < 64; i++ {
			if err := provider.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"sequence":%d}`, i))); err != nil {
				sent <- err
				return
			}
		}
		sent <- nil
	}()
	for i := 0; i < 64; i++ {
		_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := client.ReadMessage()
		require.NoError(t, err)
		require.JSONEq(t, fmt.Sprintf(`{"sequence":%d}`, i), string(data))
	}
	require.NoError(t, <-sent)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("burst workers leaked")
	}
}

func TestRejectedClientEventPreservesCorrelation(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, _ := socketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic: %T", r)
			}
		}()
		done <- Run(ctx, downstream, upstream, Options{}, Hooks{Client: func([]byte) ([]byte, error) { return nil, ErrInvalidEvent }})
	}()
	require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"event_id":"client-event-1","type":"unsupported"}`)))
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	_, data, err := client.ReadMessage()
	require.NoError(t, err)
	var event struct {
		ID    string `json:"event_id"`
		Error struct {
			ID string `json:"event_id"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(data, &event))
	require.NotEmpty(t, event.ID)
	require.Equal(t, "client-event-1", event.Error.ID)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("workers did not stop")
	}
}
