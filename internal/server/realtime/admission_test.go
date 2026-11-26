// 职责: 验证实时并发配额、断线交接、取消、限速与幂等释放。
// 关联文件:
//
//	internal/server/realtime/admission.go

package realtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestAdmissionReconnectHandoffLocalAndShared(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			var client redis.UniversalClient
			if shared {
				s := miniredis.RunT(t)
				client = redis.NewClient(&redis.Options{Addr: s.Addr()})
				t.Cleanup(func() { _ = client.Close() })
			}
			a := NewAdmission(client, Limits{Global: 2, Project: 2, APIKey: 1, Channel: 2, ConnectionsPerMinute: 10}, time.Minute)
			release, err := a.Acquire(t.Context(), 1, 1, 1, "old")
			require.NoError(t, err)
			defer release()
			type result struct {
				release func()
				err     error
			}
			done := make(chan result, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("admission panic: %T", r)
						done <- result{err: fmt.Errorf("admission panic")}
					}
				}()
				f, e := a.AcquireForReconnect(t.Context(), 1, 1, 1, "next")
				done <- result{f, e}
			}()
			require.Eventually(t, func() bool { a.mu.Lock(); defer a.mu.Unlock(); return len(a.waiting) == 1 }, time.Second, time.Millisecond)
			started := time.Now()
			_, err = a.AcquireForReconnect(t.Context(), 1, 1, 1, "third")
			require.ErrorIs(t, err, ErrAdmission)
			require.Less(t, time.Since(started), 100*time.Millisecond, "one key cannot accumulate unbounded waiters")
			release()
			var r result
			select {
			case r = <-done:
			case <-time.After(time.Second):
				t.Fatal("handoff failed")
			}
			require.NoError(t, r.err)
			r.release()
			if shared {
				n, e := client.ZCard(t.Context(), "axonhub:{realtime}:rate:1").Result()
				require.NoError(t, e)
				require.Equal(t, int64(2), n, "only admitted handshakes consume rate slots")
			}
		})
	}
}

func TestAdmissionReconnectCancellationAndRateDenial(t *testing.T) {
	a := NewAdmission(nil, Limits{ConnectionsPerMinute: 1}, time.Minute)
	release, err := a.Acquire(t.Context(), 1, 1, 1)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	_, err = a.AcquireForReconnect(ctx, 1, 1, 1, "canceled")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	a.mu.Lock()
	remaining := len(a.waiting)
	a.mu.Unlock()
	require.Zero(t, remaining)
	release()
	started := time.Now()
	_, err = a.AcquireForReconnect(t.Context(), 1, 1, 1, "rate-denied")
	require.ErrorIs(t, err, ErrAdmission)
	require.Less(t, time.Since(started), 100*time.Millisecond, "rate denials must not enter reconnect grace")
}

func TestAdmissionLocalAndShared(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "redis"}[shared], func(t *testing.T) {
			var client redis.UniversalClient
			if shared {
				s := miniredis.RunT(t)
				client = redis.NewClient(&redis.Options{Addr: s.Addr()})
				t.Cleanup(func() { client.Close() })
			}
			a := NewAdmission(client, Limits{Global: 2, Project: 1, APIKey: 1, Channel: 2, ConnectionsPerMinute: 10}, time.Minute)
			release, err := a.Acquire(context.Background(), 1, 1, 1)
			require.NoError(t, err)
			_, err = a.Acquire(context.Background(), 2, 1, 1)
			require.Error(t, err, "project cap must prevent another key")
			release()
			release()
			next, err := a.Acquire(context.Background(), 2, 1, 1)
			require.NoError(t, err)
			next()
		})
	}
}

func TestAdmissionConnectionRate(t *testing.T) {
	a := NewAdmission(nil, Limits{Global: 4, APIKey: 1, Project: 4, Channel: 4, ConnectionsPerMinute: 1}, time.Minute)
	release, err := a.Acquire(context.Background(), 1, 1, 1)
	require.NoError(t, err)
	release()
	_, err = a.Acquire(context.Background(), 1, 1, 1)
	require.Error(t, err)
}

func TestRealtimeConfigurationBounds(t *testing.T) {
	require.NoError(t, (Config{}).Validate())
	require.NoError(t, (Config{MaxDuration: time.Hour, HandshakeTimeout: time.Minute, IdleTimeout: 10 * time.Minute}).Validate())
	for _, c := range []Config{{MaxDuration: time.Hour + time.Second}, {HandshakeTimeout: time.Minute + time.Second}, {IdleTimeout: -time.Second}, {Limits: Limits{Global: 1025}}} {
		require.Error(t, c.Validate())
	}
}

func TestAccountingOwnershipTracksLocalAndRedisSessions(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "redis"}[shared], func(t *testing.T) {
			var client redis.UniversalClient
			if shared {
				server := miniredis.RunT(t)
				client = redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { client.Close() })
			}
			limiter := NewAdmission(client, Limits{}, time.Minute)
			release, err := limiter.Acquire(context.Background(), 1, 1, 1, "rt-session-owned")
			require.NoError(t, err)
			live, err := limiter.IsActive(context.Background(), "rt-session-owned")
			require.NoError(t, err)
			require.True(t, live)
			release()
			live, err = limiter.IsActive(context.Background(), "rt-session-owned")
			require.NoError(t, err)
			require.False(t, live)
		})
	}
}
