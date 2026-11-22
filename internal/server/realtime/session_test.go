// 职责: 验证实时会话确认、并发转换、取消与双向连接清理。
// 关联文件:
//
//	internal/server/realtime/session.go

package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	qwenrt "github.com/looplj/axonhub/llm/realtime"
)

func TestSessionSlowConsumerReceivesBackpressureCloseAndFlushes(t *testing.T) {
	client, downstream := socketPair(t)
	require.NoError(t, client.UnderlyingConn().(*net.TCPConn).SetReadBuffer(1024))
	require.NoError(t, downstream.UnderlyingConn().(*net.TCPConn).SetWriteBuffer(1024))
	upstream, provider := socketPair(t)
	full := make(chan struct{})
	var observed atomic.Int32
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("session panic: %T", r)
				done <- fmt.Errorf("session panic")
			}
		}()
		done <- Run(context.Background(), downstream, upstream, Options{QueueSize: 1, WriteTimeout: 4 * time.Second}, Hooks{Server: func(b []byte) ([]byte, error) {
			if observed.Add(1) == 3 {
				close(full)
			}
			return b, nil
		}})
	}()
	// One frame in flight, one queued, the third blocks the production queue.
	payload := []byte(strings.Repeat("x", 64<<10))
	for range 3 {
		require.NoError(t, provider.WriteMessage(websocket.TextMessage, payload))
	}
	select {
	case <-full:
	case <-time.After(time.Second):
		t.Fatal("queue did not fill")
	}
	time.Sleep(1200 * time.Millisecond) // unchanged 1s backpressure wait, then resume within close flush grace
	require.NoError(t, client.UnderlyingConn().(*net.TCPConn).SetReadBuffer(4<<20))
	client.SetCloseHandler(func(int, string) error { return nil }) // real peer deliberately delays Close ACK
	require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Second)))
	for {
		_, _, err := client.ReadMessage()
		if err == nil {
			continue
		}
		var closed *websocket.CloseError
		require.ErrorAs(t, err, &closed)
		require.Equal(t, 1013, closed.Code)
		require.Equal(t, "backpressure", closed.Text)
		break
	}
	select {
	case err := <-done:
		t.Fatalf("TCP closed before peer Close ACK: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	require.NoError(t, client.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1013, ""), time.Now().Add(time.Second)))
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrBackpressure)
	case <-time.After(time.Second):
		t.Fatal("close ACK did not release workers")
	}
}

func TestSessionNormalCloseFrame(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, _ := socketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("session panic: %T", r)
				done <- fmt.Errorf("session panic")
			}
		}()
		done <- Run(ctx, downstream, upstream, Options{}, Hooks{})
	}()
	cancel()
	require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Second)))
	_, _, err := client.ReadMessage()
	var closed *websocket.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, 1000, closed.Code)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("normal close did not finish")
	}
}

func TestSessionInternalErrorCloseFrame(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, provider := socketPair(t)
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("session panic: %T", r)
				done <- fmt.Errorf("session panic")
			}
		}()
		done <- Run(context.Background(), downstream, upstream, Options{}, Hooks{Server: func([]byte) ([]byte, error) { return nil, ErrAccountingUnavailable }})
	}()
	require.NoError(t, provider.WriteMessage(websocket.TextMessage, []byte(`{"type":"error"}`)))
	require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Second)))
	_, _, err := client.ReadMessage()
	var closed *websocket.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, 1011, closed.Code)
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrAccountingUnavailable)
	case <-time.After(time.Second):
		t.Fatal("internal error close did not finish")
	}
}

func socketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	return socketPairWithDialer(t, websocket.DefaultDialer)
}

func socketPairWithDialer(t *testing.T, dialer *websocket.Dialer) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if e == nil {
			accepted <- c
		}
	}))
	t.Cleanup(srv.Close)
	client, _, err := dialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)
	server := <-accepted
	t.Cleanup(func() { client.Close(); server.Close() })
	return client, server
}

func runSessionForTest(t *testing.T, done chan<- error, run func() error) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Errorf("session panic: %T", recovered)
			done <- fmt.Errorf("session panic")
		}
	}()
	done <- run()
}

func TestSessionBidirectionalAndCancellation(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, provider := socketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go runSessionForTest(t, finished, func() error {
		return Run(ctx, downstream, upstream, Options{IdleTimeout: time.Second, MaxDuration: time.Minute}, Hooks{})
	})
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
	go runSessionForTest(t, done, func() error {
		return Run(ctx, downstream, upstream, Options{IdleTimeout: time.Second}, Hooks{Client: func([]byte) ([]byte, error) { return nil, ErrInvalidEvent }})
	})
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
			go runSessionForTest(t, done, func() error {
				return Run(context.Background(), downstream, upstream, Options{IdleTimeout: time.Second}, Hooks{})
			})
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
		go runSessionForTest(t, done, func() error { return Run(context.Background(), downstream, upstream, Options{}, Hooks{}) })
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

func TestSendQueueFullBlocksThenReturnsBackpressure(t *testing.T) {
	const capacity = 3
	const wait = 80 * time.Millisecond
	q := newSendQueue(capacity, wait)
	// The first capacity frames succeed without a consumer; only the next frame
	// may time out. No session deadline or input-rate limiter participates.
	for i := range capacity {
		require.NoError(t, q.enqueue(context.Background(), []byte{byte(i)}))
	}
	started := time.Now()
	err := q.enqueue(context.Background(), []byte{capacity})
	require.ErrorIs(t, err, ErrBackpressure)
	require.GreaterOrEqual(t, time.Since(started), wait*3/4, "full queue must actually block the producer")
	for i := range capacity {
		require.Equal(t, []byte{byte(i)}, <-q.frames, "timeout must preserve accepted frames")
	}
}

func TestSendQueueBlockedProducerResumesWithoutDropping(t *testing.T) {
	q := newSendQueue(1, time.Second)
	require.NoError(t, q.enqueue(context.Background(), []byte("first")))
	type result struct {
		err     error
		elapsed time.Duration
	}
	sent := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("session test worker panic: %T", r)
				sent <- result{err: fmt.Errorf("panic: %T", r)}
			}
		}()
		started := time.Now()
		err := q.enqueue(context.Background(), []byte("second"))
		sent <- result{err: err, elapsed: time.Since(started)}
	}()
	select {
	case outcome := <-sent:
		t.Fatalf("producer did not wait for queue space: %+v", outcome)
	case <-time.After(80 * time.Millisecond):
	}
	require.Equal(t, []byte("first"), <-q.frames)
	select {
	case outcome := <-sent:
		require.NoError(t, outcome.err)
		require.GreaterOrEqual(t, outcome.elapsed, 60*time.Millisecond)
	case <-time.After(time.Second):
		t.Fatal("producer did not resume after consumer released space")
	}
	select {
	case frame := <-q.frames:
		require.Equal(t, []byte("second"), frame, "accepted frame must not be silently discarded")
	case <-time.After(time.Second):
		t.Fatal("resumed producer silently dropped the pending frame")
	}
}

func TestSessionSlowClientBoundedBackpressure(t *testing.T) {
	client, downstream := socketPair(t)
	// Prevent the loopback receive buffer from absorbing the whole burst while
	// this client deliberately does not consume frames.
	require.NoError(t, client.UnderlyingConn().(*net.TCPConn).SetReadBuffer(1024))
	upstream, provider := socketPair(t)
	var observed atomic.Int32
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("session test worker panic: %T", r)
				done <- fmt.Errorf("panic: %T", r)
			}
		}()
		done <- Run(context.Background(), downstream, upstream,
			Options{QueueSize: 1, WriteTimeout: 4 * time.Second, MaxDuration: 30 * time.Second, IdleTimeout: 30 * time.Second},
			Hooks{Server: func(data []byte) ([]byte, error) { observed.Add(1); return data, nil }})
	}()
	// Upstream messages bypass the client-only input rate limiter. Session and
	// socket deadlines are deliberately longer than the queue's one-second wait.
	payload := []byte(strings.Repeat("x", 200<<10))
	started := time.Now()
	require.NoError(t, provider.SetWriteDeadline(started.Add(8*time.Second)))
	for range 128 {
		if err := provider.WriteMessage(websocket.TextMessage, payload); err != nil {
			break
		}
	}
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrBackpressure)
		require.Greater(t, observed.Load(), int32(1), "must inject more frames than the queue capacity")
		require.GreaterOrEqual(t, time.Since(started), 750*time.Millisecond)
	case <-time.After(10 * time.Second):
		t.Fatal("full send queue did not terminate the stalled session")
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
				t.Errorf("session test worker panic: %T", r)
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

type gatedSessionWriteConn struct {
	net.Conn

	nextWrite atomic.Bool
	entered   chan struct{}
	release   <-chan struct{}
	ctx       context.Context
}

func (c *gatedSessionWriteConn) Write(data []byte) (int, error) {
	if c.nextWrite.Swap(false) {
		close(c.entered)
		select {
		case <-c.release:
		case <-c.ctx.Done():
			return 0, c.ctx.Err()
		}
	}
	return c.Conn.Write(data)
}

func TestSessionOrderlyUpstreamCloseDeliversQueuedTail(t *testing.T) {
	release := make(chan struct{})
	var gate *gatedSessionWriteConn
	dialer := websocket.Dialer{NetDialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		var dial net.Dialer
		conn, err := dial.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		gate = &gatedSessionWriteConn{Conn: conn, ctx: t.Context(), entered: make(chan struct{}), release: release}
		return gate, nil
	}}
	downstream, client := socketPairWithDialer(t, &dialer)
	gate.nextWrite.Store(true)
	upstream, provider := socketPair(t)
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error {
		return Run(t.Context(), downstream, upstream, Options{QueueSize: 4}, Hooks{})
	})

	frames := []string{`{"type":"response.output_audio.delta"}`, `{"type":"response.output_audio.done"}`, `{"type":"response.done"}`}
	for _, frame := range frames {
		require.NoError(t, provider.WriteMessage(websocket.TextMessage, []byte(frame)))
	}
	require.NoError(t, provider.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second)))
	require.NoError(t, provider.SetReadDeadline(time.Now().Add(time.Second)))
	_, _, err := provider.ReadMessage()
	require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure), "%v", err)

	select {
	case <-gate.entered:
	case <-time.After(time.Second):
		t.Fatal("downstream writer did not start")
	}
	close(release)
	require.NoError(t, client.SetReadDeadline(time.Now().Add(2*time.Second)))
	for i, expected := range frames {
		kind, data, readErr := client.ReadMessage()
		require.NoError(t, readErr, "frame %d", i)
		require.Equal(t, websocket.TextMessage, kind)
		require.True(t, expected == string(data), "frame %d differed", i)
	}
	_, _, err = client.ReadMessage()
	require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure), "%v", err)
	select {
	case result := <-done:
		require.False(t, IsAbnormal(result), "%v", result)
	case <-time.After(time.Second):
		t.Fatal("session did not close after draining")
	}
}

func TestDrainUpstreamTailTimesOutWithoutWriter(t *testing.T) {
	frames := make(chan []byte, 1)
	written := make(chan struct{})
	require.ErrorIs(t, drainUpstreamTail(t.Context(), frames, written, 20*time.Millisecond), ErrBackpressure)
	require.Len(t, frames, 1, "the timeout must cover waiting for the writer after the marker is queued")
}

func TestSessionQueuesConfigurationUpdates(t *testing.T) {
	for _, tc := range []struct {
		name            string
		count, accepted int
		rejectFirst     bool
		cancelPending   bool
	}{
		{name: "two", count: 2, accepted: 2},
		{name: "three", count: 3, accepted: 3},
		{name: "overflow", count: 6, accepted: 5},
		{name: "provider rejects first", count: 3, accepted: 3, rejectFirst: true},
		{name: "cancel while waiting", count: 3, cancelPending: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, downstream := socketPair(t)
			upstream, provider := socketPair(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			state := NewState()
			require.NoError(t, state.Connecting())
			require.NoError(t, state.ObserveServer("session.created"))
			state.ConfirmDefaultConfiguration()
			done := make(chan error, 1)
			go runSessionForTest(t, done, func() error {
				return Run(ctx, downstream, upstream, Options{}, Hooks{
					Client: func(data []byte) ([]byte, error) {
						if gjson.GetBytes(data, "type").String() == "session.update" {
							return data, state.Update()
						}
						if !state.Ready() {
							return nil, ErrUpdatePending
						}
						return data, nil
					},
					Server: func(data []byte) ([]byte, error) {
						kind := gjson.GetBytes(data, "type").String()
						if kind == "error" {
							state.ConfigurationRejected()
						}
						return data, state.ObserveServer(kind)
					},
				})
			})
			require.NoError(t, client.SetReadDeadline(time.Now().Add(5*time.Second)))
			require.NoError(t, provider.SetReadDeadline(time.Now().Add(5*time.Second)))
			for i := 1; i <= tc.count; i++ {
				require.NoError(t, client.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"type":"session.update","event_id":"u%d"}`, i)))
			}
			require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","event_id":"blocked"}`)))
			for i := tc.accepted; tc.accepted > 0 && i < tc.count; i++ {
				_, rejected, err := client.ReadMessage()
				require.NoError(t, err)
				require.Equal(t, "session_update_queue_full", gjson.GetBytes(rejected, "error.code").String())
			}
			_, blocked, err := client.ReadMessage()
			require.NoError(t, err)
			require.Equal(t, "blocked", gjson.GetBytes(blocked, "error.event_id").String())
			require.Equal(t, ErrUpdatePending.Error(), gjson.GetBytes(blocked, "error.message").String())
			if !tc.cancelPending {
				for i := 1; i <= tc.accepted; i++ {
					_, data, err := provider.ReadMessage()
					require.NoError(t, err)
					id := fmt.Sprintf("u%d", i)
					require.Equal(t, id, gjson.GetBytes(data, "event_id").String())
					kind := "session.updated"
					if tc.rejectFirst && i == 1 {
						kind = "error"
					}
					require.NoError(t, provider.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"type":"%s","event_id":"%s"}`, kind, id)))
					_, ack, err := client.ReadMessage()
					require.NoError(t, err)
					require.Equal(t, kind, gjson.GetBytes(ack, "type").String())
					require.Equal(t, id, gjson.GetBytes(ack, "event_id").String())
				}
			}
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("session workers did not exit")
			}
		})
	}
}

func TestClientErrorExposesOnlySafeDetails(t *testing.T) {
	adapter, err := qwenrt.NewAdapter("qwen3.8-omni-flash-realtime", true)
	require.NoError(t, err)
	frame := []byte(`{"type":"session.update","event_id":"update-1","session":{"audio":{"input":{"unknown_field":"private-value"}}}}`)
	_, fieldErr := adapter.Client(frame)
	require.ErrorIs(t, fieldErr, qwenrt.ErrUnsupportedField)
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"unsupported field", fmt.Errorf("private wrapper: %w", fieldErr), `unsupported realtime field "audio.input.unknown_field"`},
		{"pending", ErrUpdatePending, ErrUpdatePending.Error()},
		{"unknown", fmt.Errorf("private-value"), "Unsupported or invalid realtime event; see the gateway Realtime capability documentation."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := clientError(frame, tc.err)
			require.Equal(t, tc.want, gjson.GetBytes(wire, "error.message").String())
			require.Equal(t, "update-1", gjson.GetBytes(wire, "error.event_id").String())
		})
	}
}
