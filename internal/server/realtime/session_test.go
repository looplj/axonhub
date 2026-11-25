// 职责: 验证实时会话确认、并发转换、取消与双向连接清理。
// 关联文件:
//
//	internal/server/realtime/session.go

package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	qwenrt "github.com/looplj/axonhub/llm/realtime"
)

func readWithTimeout(conn *websocket.Conn, timeout time.Duration) (websocket.MessageType, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return conn.Read(ctx)
}

func writeTextWithTimeout(conn *websocket.Conn, timeout time.Duration, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data)
}

func TestSessionSlowConsumerReceivesBackpressureCloseAndFlushes(t *testing.T) {
	release := make(chan struct{})
	var gate *gatedSessionWriteConn
	downstream, client := socketPairWithNetDial(t, func(ctx context.Context, network, address string) (net.Conn, error) {
		var dial net.Dialer
		conn, err := dial.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		gate = newGatedSessionWriteConn(conn, t.Context(), release)
		return gate, nil
	})
	gate.nextWrite.Store(true)
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
		require.NoError(t, writeTextWithTimeout(provider, time.Second, payload))
	}
	select {
	case <-full:
	case <-time.After(time.Second):
		t.Fatal("queue did not fill")
	}
	select {
	case <-gate.entered:
	case <-time.After(time.Second):
		t.Fatal("downstream writer did not start")
	}
	time.Sleep(1200 * time.Millisecond) // unchanged 1s backpressure wait, then resume within close flush grace
	select {
	case err := <-done:
		t.Fatalf("TCP closed before slow consumer resumed and acknowledged Close: %v", err)
	default:
	}
	close(release)
	for {
		_, _, err := readWithTimeout(client, time.Second)
		if err == nil {
			continue
		}
		var closed websocket.CloseError
		require.ErrorAs(t, err, &closed)
		require.Equal(t, websocket.StatusTryAgainLater, closed.Code)
		require.Equal(t, "backpressure", closed.Reason)
		break
	}
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
	_, _, err := readWithTimeout(client, time.Second)
	var closed websocket.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, websocket.StatusNormalClosure, closed.Code)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("normal close did not finish")
	}
}

func TestClientEventRateLimitDefaultAllowsMoreThanLegacy256(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, _ := socketPair(t)
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error { return Run(context.Background(), downstream, upstream, Options{}, Hooks{}) })
	for range 300 {
		require.NoError(t, writeTextWithTimeout(client, time.Second, []byte(`{"type":"ping"}`)))
	}
	select {
	case err := <-done:
		t.Fatalf("default rate limit closed the session before 300 events: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, client.Close(websocket.StatusNormalClosure, ""))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("session did not stop after normal client close")
	}
}

func TestClientEventRateLimitDefaultAllows400EventsPerSecondForTenSeconds(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, _ := socketPair(t)
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error { return Run(context.Background(), downstream, upstream, Options{}, Hooks{}) })
	ticker := time.NewTicker(time.Second / 400)
	defer ticker.Stop()
	for range 4000 {
		<-ticker.C
		require.NoError(t, writeTextWithTimeout(client, time.Second, []byte(`{"type":"ping"}`)))
	}
	select {
	case err := <-done:
		t.Fatalf("default rate limit closed the session during 400 events/second: %v", err)
	default:
	}
	require.NoError(t, client.Close(websocket.StatusNormalClosure, ""))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("session did not stop after normal client close")
	}
}

func TestClientEventRateLimitDefaultClosesAfter1024Events(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, _ := socketPair(t)
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error { return Run(context.Background(), downstream, upstream, Options{}, Hooks{}) })
	for range 1025 {
		if err := writeTextWithTimeout(client, time.Second, []byte(`{"type":"ping"}`)); err != nil {
			break
		}
	}
	_, _, err := readWithTimeout(client, time.Second)
	var closed websocket.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, websocket.StatusPolicyViolation, closed.Code)
	require.Equal(t, "client_rate_limited", closed.Reason)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("default rate-limited session did not stop")
	}
}

func TestClientEventRateLimitUsesOverrideAndSendsPolicyClose(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, _ := socketPair(t)
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error {
		return Run(context.Background(), downstream, upstream, Options{ClientEventsPerSecond: 300}, Hooks{})
	})
	for range 301 {
		if err := writeTextWithTimeout(client, time.Second, []byte(`{"type":"ping"}`)); err != nil {
			break
		}
	}
	_, _, err := readWithTimeout(client, time.Second)
	var closed websocket.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, websocket.StatusPolicyViolation, closed.Code)
	require.Equal(t, "client_rate_limited", closed.Reason)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("rate-limited session did not stop")
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
	require.NoError(t, writeTextWithTimeout(provider, time.Second, []byte(`{"type":"error"}`)))
	_, frame, err := readWithTimeout(client, time.Second)
	require.NoError(t, err)
	require.Contains(t, string(frame), `"code":"internal_error"`)
	_, _, err = readWithTimeout(client, time.Second)
	var closed websocket.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, websocket.StatusInternalError, closed.Code)
	require.Equal(t, "internal_error", closed.Reason)
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrAccountingUnavailable)
	case <-time.After(time.Second):
		t.Fatal("internal error close did not finish")
	}
}

func TestSessionIdleTimeoutClosesWithNormalStatusAndReason(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, _ := socketPair(t)
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error {
		return Run(context.Background(), downstream, upstream, Options{IdleTimeout: 50 * time.Millisecond, PingInterval: time.Minute}, Hooks{})
	})

	_, _, err := readWithTimeout(client, 2*time.Second)
	var closed websocket.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, websocket.StatusNormalClosure, closed.Code)
	require.Equal(t, "idle_timeout", closed.Reason)
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrIdleTimeout)
	case <-time.After(time.Second):
		t.Fatal("idle session did not terminate")
	}
}

func TestSessionClientPingCountsAsIdleActivity(t *testing.T) {
	downstreamActivity := NewActivity()
	client, downstream := socketPairWithActivity(t, downstreamActivity)
	upstream, _ := socketPair(t)
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error {
		return Run(context.Background(), downstream, upstream, Options{
			IdleTimeout:        1100 * time.Millisecond,
			PingInterval:       time.Minute,
			DownstreamActivity: downstreamActivity,
		}, Hooks{})
	})

	// coder/websocket Ping waits for the matching Pong read by a concurrent Reader.
	clientReadErr := make(chan error, 1)
	go func() {
		for {
			_, _, err := client.Read(t.Context())
			if err != nil {
				clientReadErr <- err
				return
			}
		}
	}()

	stopPings := make(chan struct{})
	pingsDone := make(chan struct{})
	go func() {
		defer close(pingsDone)
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("client ping sender panicked: %v", recovered)
			}
		}()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopPings:
				return
			case <-ticker.C:
				pctx, pcancel := context.WithTimeout(t.Context(), time.Second)
				err := client.Ping(pctx)
				pcancel()
				if err != nil {
					return
				}
			}
		}
	}()
	select {
	case err := <-done:
		t.Fatalf("client ping activity did not keep the session alive: %v", err)
	case <-time.After(2200 * time.Millisecond):
	}
	close(stopPings)
	<-pingsDone

	select {
	case err := <-clientReadErr:
		var closed websocket.CloseError
		require.ErrorAs(t, err, &closed)
		require.Equal(t, websocket.StatusNormalClosure, closed.Code)
		require.Equal(t, "idle_timeout", closed.Reason)
	case <-time.After(4 * time.Second):
		t.Fatal("client did not observe idle timeout close")
	}
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrIdleTimeout)
	case <-time.After(time.Second):
		t.Fatal("idle session did not close after pings stopped")
	}
}

func TestSessionUpstreamPongsCannotKeepSilentClientAlive(t *testing.T) {
	_, downstream := socketPair(t)
	upstream, provider := socketPair(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error {
		return Run(ctx, downstream, upstream, Options{IdleTimeout: 350 * time.Millisecond, PingInterval: 100 * time.Millisecond}, Hooks{})
	})

	// Read provider-side pings so coder/websocket answers them with pongs. Leave the
	// client socket unread until shutdown, as with a silently lost connection.
	providerDone := make(chan error, 1)
	go runSessionForTest(t, providerDone, func() error {
		_, _, err := provider.Read(ctx)
		return err
	})
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrIdleTimeout)
	case <-time.After(3 * time.Second):
		t.Fatal("upstream pongs kept a silent client session alive")
	}
}

func TestSafeCloseCauseSanitizesAndBoundsUnknownErrors(t *testing.T) {
	encoded := strings.Repeat("QUJD", 80)
	message := safeCloseCause(errors.New("provider failure\nresponse=" + encoded))
	require.Contains(t, message, "provider failure")
	require.NotContains(t, message, "\n")
	require.NotContains(t, message, encoded)
	require.LessOrEqual(t, len([]rune(message)), 200)
}

func socketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	return socketPairWithOptions(t, nil, nil)
}

func socketPairWithActivity(t *testing.T, serverActivity *Activity) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	return socketPairWithOptions(t, &websocket.AcceptOptions{
		OnPingReceived: func(context.Context, []byte) bool {
			serverActivity.Touch()
			return true
		},
		OnPongReceived: func(context.Context, []byte) {
			serverActivity.Touch()
		},
	}, nil)
}

func socketPairWithNetDial(t *testing.T, dialContext func(context.Context, string, string) (net.Conn, error)) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	transport := &http.Transport{DialContext: dialContext}
	t.Cleanup(transport.CloseIdleConnections)
	return socketPairWithOptions(t, nil, &websocket.DialOptions{
		HTTPClient: &http.Client{Transport: transport},
	})
}

func socketPairWithOptions(t *testing.T, acceptOpts *websocket.AcceptOptions, dialOpts *websocket.DialOptions) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := websocket.Accept(w, r, acceptOpts)
		if e == nil {
			c.SetReadLimit(MaxMessageBytes)
			accepted <- c
		}
	}))
	t.Cleanup(srv.Close)
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(srv.URL, "http"), dialOpts)
	require.NoError(t, err)
	client.SetReadLimit(MaxMessageBytes)
	server := <-accepted
	t.Cleanup(func() { _ = client.CloseNow(); _ = server.CloseNow() })
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
	require.NoError(t, writeTextWithTimeout(client, time.Second, []byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`)))
	_, data, err := readWithTimeout(provider, time.Second)
	require.NoError(t, err)
	require.Contains(t, string(data), "append")
	require.NoError(t, writeTextWithTimeout(provider, time.Second, []byte(`{"type":"response.created"}`)))
	_, data, err = readWithTimeout(client, time.Second)
	require.NoError(t, err)
	require.Contains(t, string(data), "response.created")
	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("session goroutines did not exit")
	}
	_, _, err = readWithTimeout(provider, time.Second)
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
	require.NoError(t, writeTextWithTimeout(client, time.Second, []byte(`{"type":"invalid"}`)))
	_, data, err := readWithTimeout(client, time.Second)
	require.NoError(t, err)
	require.Contains(t, string(data), "invalid_request_error")
	_, _, err = readWithTimeout(provider, 20*time.Millisecond)
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
				_ = provider.CloseNow()
				peer = client
			} else {
				_ = client.CloseNow()
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("disconnected peer leaked workers")
			}
			_, frame, err := readWithTimeout(peer, time.Second)
			if upstreamDisconnect {
				require.NoError(t, err)
				require.Contains(t, string(frame), `"code":"upstream_unavailable"`)
				_, _, err = readWithTimeout(peer, time.Second)
				var closed websocket.CloseError
				require.ErrorAs(t, err, &closed)
				require.Equal(t, websocket.StatusTryAgainLater, closed.Code)
			}
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
		_ = writeTextWithTimeout(client, time.Second, []byte(strings.Repeat("a", MaxMessageBytes+1)))
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
	release := make(chan struct{})
	var gate *gatedSessionWriteConn
	downstream, _ := socketPairWithNetDial(t, func(ctx context.Context, network, address string) (net.Conn, error) {
		var dial net.Dialer
		conn, err := dial.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		gate = newGatedSessionWriteConn(conn, t.Context(), release)
		return gate, nil
	})
	gate.nextWrite.Store(true)
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
	payload := []byte(strings.Repeat("x", 64<<10))
	started := time.Now()
	for range 4 {
		if err := writeTextWithTimeout(provider, 8*time.Second, payload); err != nil {
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
	require.NoError(t, writeTextWithTimeout(client, time.Second, []byte(`{"event_id":"client-event-1","type":"unsupported"}`)))
	_, data, err := readWithTimeout(client, time.Second)
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
	closed    chan struct{}
	closeOnce sync.Once
}

func newGatedSessionWriteConn(conn net.Conn, ctx context.Context, release <-chan struct{}) *gatedSessionWriteConn {
	return &gatedSessionWriteConn{
		Conn:    conn,
		entered: make(chan struct{}),
		release: release,
		ctx:     ctx,
		closed:  make(chan struct{}),
	}
}

func (c *gatedSessionWriteConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func (c *gatedSessionWriteConn) Write(data []byte) (int, error) {
	if c.nextWrite.Swap(false) {
		close(c.entered)
		select {
		case <-c.release:
		case <-c.closed:
			return 0, net.ErrClosed
		case <-c.ctx.Done():
			return 0, c.ctx.Err()
		}
	}
	return c.Conn.Write(data)
}

func TestSessionOrderlyUpstreamCloseDeliversQueuedTail(t *testing.T) {
	release := make(chan struct{})
	var gate *gatedSessionWriteConn
	downstream, client := socketPairWithNetDial(t, func(ctx context.Context, network, address string) (net.Conn, error) {
		var dial net.Dialer
		conn, err := dial.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		gate = newGatedSessionWriteConn(conn, t.Context(), release)
		return gate, nil
	})
	gate.nextWrite.Store(true)
	upstream, provider := socketPair(t)
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error {
		return Run(t.Context(), downstream, upstream, Options{QueueSize: 4}, Hooks{})
	})

	frames := []string{`{"type":"response.output_audio.delta"}`, `{"type":"response.output_audio.done"}`, `{"type":"response.done"}`}
	for _, frame := range frames {
		require.NoError(t, writeTextWithTimeout(provider, time.Second, []byte(frame)))
	}
	require.NoError(t, provider.Close(websocket.StatusNormalClosure, ""))

	select {
	case <-gate.entered:
	case <-time.After(time.Second):
		t.Fatal("downstream writer did not start")
	}
	close(release)
	for i, expected := range frames {
		kind, data, readErr := readWithTimeout(client, 2*time.Second)
		require.NoError(t, readErr, "frame %d", i)
		require.Equal(t, websocket.MessageText, kind)
		require.True(t, expected == string(data), "frame %d differed", i)
	}
	_, _, err := readWithTimeout(client, 2*time.Second)
	require.Equal(t, websocket.StatusNormalClosure, websocket.CloseStatus(err), "%v", err)
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
						kind := gjson.GetBytes(data, "type").String()
						if kind == "response.cancel" {
							return data, nil
						}
						if kind == "session.update" {
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
			for i := 1; i <= tc.count; i++ {
				require.NoError(t, writeTextWithTimeout(client, 5*time.Second, fmt.Appendf(nil, `{"type":"session.update","event_id":"u%d"}`, i)))
			}
			require.NoError(t, writeTextWithTimeout(client, 5*time.Second, []byte(`{"type":"response.create","event_id":"blocked"}`)))
			for i := tc.accepted; tc.accepted > 0 && i < tc.count; i++ {
				_, rejected, err := readWithTimeout(client, 5*time.Second)
				require.NoError(t, err)
				require.Equal(t, "session_update_queue_full", gjson.GetBytes(rejected, "error.code").String())
			}
			_, blocked, err := readWithTimeout(client, 5*time.Second)
			require.NoError(t, err)
			require.Equal(t, "blocked", gjson.GetBytes(blocked, "error.event_id").String())
			require.Equal(t, ErrUpdatePending.Error(), gjson.GetBytes(blocked, "error.message").String())
			if !tc.cancelPending {
				for i := 1; i <= tc.accepted; i++ {
					_, data, err := readWithTimeout(provider, 5*time.Second)
					require.NoError(t, err)
					id := fmt.Sprintf("u%d", i)
					require.Equal(t, id, gjson.GetBytes(data, "event_id").String())
					kind := "session.updated"
					if tc.rejectFirst && i == 1 {
						kind = "error"
					}
					require.NoError(t, writeTextWithTimeout(provider, 5*time.Second, fmt.Appendf(nil, `{"type":"%s","event_id":"%s"}`, kind, id)))
					_, ack, err := readWithTimeout(client, 5*time.Second)
					require.NoError(t, err)
					require.Equal(t, kind, gjson.GetBytes(ack, "type").String())
					require.Equal(t, id, gjson.GetBytes(ack, "event_id").String())
				}
			} else {
				require.NoError(t, writeTextWithTimeout(client, 5*time.Second, []byte(`{"type":"response.cancel"}`)))
				_, firstUpdate, err := readWithTimeout(provider, 5*time.Second)
				require.NoError(t, err)
				require.Equal(t, "session.update", gjson.GetBytes(firstUpdate, "type").String())
				_, cancelFrame, err := readWithTimeout(provider, 5*time.Second)
				require.NoError(t, err)
				require.Equal(t, "response.cancel", gjson.GetBytes(cancelFrame, "type").String())
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

func TestSessionRejectsDuplicateJSONKeysBeforeClientAdmission(t *testing.T) {
	client, downstream := socketPair(t)
	upstream, provider := socketPair(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var admitted atomic.Int32
	done := make(chan error, 1)
	go runSessionForTest(t, done, func() error {
		return Run(ctx, downstream, upstream, Options{}, Hooks{Client: func(data []byte) ([]byte, error) {
			admitted.Add(1)
			return data, nil
		}})
	})

	require.NoError(t, writeTextWithTimeout(client, time.Second, []byte(`{"type":"conversation.item.create","type":"input_audio_buffer.append","audio":"AAA="}`)))
	_, rejected, err := readWithTimeout(client, time.Second)
	require.NoError(t, err)
	require.Equal(t, "unsupported_event", gjson.GetBytes(rejected, "error.code").String())
	require.Zero(t, admitted.Load(), "ambiguous event envelopes must be rejected before routing and accounting")

	valid := []byte(`{"type":"input_audio_buffer.append","audio":"AAA="}`)
	require.NoError(t, writeTextWithTimeout(client, time.Second, valid))
	_, forwarded, err := readWithTimeout(provider, time.Second)
	require.NoError(t, err)
	require.JSONEq(t, string(valid), string(forwarded))
	require.EqualValues(t, 1, admitted.Load())

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("session did not stop after duplicate-envelope rejection")
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

func TestFlushDownstreamCloseRecoversFromPanic(t *testing.T) {
	readerDone := make(chan struct{})
	started := time.Now()
	flushDownstreamClose(t.Context(), nil, int(websocket.StatusNormalClosure), "test", func() {}, readerDone)
	require.Less(t, time.Since(started), 200*time.Millisecond, "recovered close panic must release done immediately")
}
