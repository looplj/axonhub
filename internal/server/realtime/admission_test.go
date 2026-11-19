// 职责: 验证实时会话并发配额的原子获取和幂等释放。
// 关联文件:
//
//	internal/server/realtime/admission.go
package realtime

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

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
