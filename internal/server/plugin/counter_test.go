package plugin

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
)

func TestMemoryCounterWindowsAreIndependent(t *testing.T) {
	counters := newMemoryCounters()
	now := time.Unix(1_700_000_000, 0)

	require.Equal(t, int64(1), counters.add(now, "acme.guard", "failover:1:gpt-5", 300))
	require.Equal(t, int64(2), counters.add(now.Add(5*time.Second), "acme.guard", "failover:1:gpt-5", 300))
	require.Equal(t, int64(1), counters.add(now, "acme.guard", "failover:1:gpt-5", 10))
	require.Equal(t, int64(2), counters.get(now.Add(5*time.Second), "acme.guard", "failover:1:gpt-5", 300))
	require.Equal(t, int64(1), counters.get(now, "acme.guard", "failover:1:gpt-5", 10))
	require.Equal(t, int64(0), counters.get(now, "acme.other", "failover:1:gpt-5", 300))
	require.Equal(t, int64(0), counters.get(now.Add(12*time.Second), "acme.guard", "failover:1:gpt-5", 10))
}

func TestWriteLimiterPerMinute(t *testing.T) {
	limiter := newPluginWriteLimiter()
	now := time.Date(2026, 11, 6, 3, 0, 0, 0, time.UTC)
	for range maxWritesPerMinute {
		require.True(t, limiter.allow(now, "acme.guard"))
	}
	require.False(t, limiter.allow(now, "acme.guard"))
	require.True(t, limiter.allow(now, "acme.other"))
	require.True(t, limiter.allow(now.Add(time.Minute), "acme.guard"))
	require.Len(t, limiter.calls["acme.guard"], 1)
}

func BenchmarkMemoryCounterLongWindow(b *testing.B) {
	for _, keys := range []int{1, 1000} {
		b.Run(fmt.Sprint(keys), func(b *testing.B) {
			counters := newMemoryCounters()
			now := time.Unix(1700000000, 0)
			for i := range keys {
				counters.add(now, "bench", fmt.Sprint(i), counterWindowMax)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				counters.get(now, "bench", "0", counterWindowMax)
			}
		})
	}
}

func TestMemoryCounterMatchesBucketWindows(t *testing.T) {
	for _, window := range []int{1, 10, 60, 61, 300, 86400} {
		t.Run(fmt.Sprint(window), func(t *testing.T) {
			counters := newMemoryCounters()
			now := time.Unix(1700000000, 0)
			random := rand.New(rand.NewPCG(1, 2))
			type bucket struct {
				count   int64
				expires time.Time
			}
			reference := map[int64]bucket{}
			size := int64(1)
			if window > 60 {
				size = 10
			}
			for i := range 3000 {
				now = now.Add(time.Duration(random.IntN(window+1)) * time.Millisecond)
				current := now.Unix() / size * size
				for stamp, b := range reference {
					if !b.expires.After(now) {
						delete(reference, stamp)
					}
				}
				add := i%3 != 0
				if add {
					b := reference[current]
					b.count++
					b.expires = now.Add(time.Duration(window+1) * time.Second)
					reference[current] = b
				}
				var want int64
				for stamp, b := range reference {
					if stamp <= current && (current-stamp)/size < int64((window+int(size)-1)/int(size)) {
						want += b.count
					}
				}
				var got int64
				if add {
					got = counters.add(now, "test", "key", window)
				} else {
					got = counters.get(now, "test", "key", window)
				}
				require.Equal(t, want, got, "step %d at %s", i, now)
			}
			require.Zero(t, counters.get(now.Add(25*time.Hour), "test", "key", window))
		})
	}
}

func TestMemoryCounterIdleCleanup(t *testing.T) {
	counters := newMemoryCounters()
	now := time.Unix(1700000000, 0)
	for i := range 1000 {
		counters.add(now, "test", fmt.Sprint(i), 1)
	}
	require.Len(t, counters.series, 1000)
	// An unrelated access eventually reclaims idle series, without a worker.
	require.Zero(t, counters.get(now.Add(time.Minute), "test", "missing", 1))
	require.Empty(t, counters.series)
}

func TestCounterBackendLifecycle(t *testing.T) {
	for _, mode := range []string{xcache.ModeMemory, xcache.ModeRedis, xcache.ModeTwoLevel} {
		t.Run(mode, func(t *testing.T) {
			mr := miniredis.RunT(t)
			manager, err := newManager(ManagerParams{Client: newPluginDB(t), Cache: xcache.Config{Mode: mode, Redis: xredis.Config{Addr: mr.Addr()}}}, managerConfig{})
			require.NoError(t, err)
			defer manager.Stop(context.Background())
			// Only the backend for the cache mode is created, at construction.
			if mode == xcache.ModeMemory {
				require.NotNil(t, manager.memCounters)
				require.Nil(t, manager.redis)
			} else {
				require.NotNil(t, manager.redis)
				require.Nil(t, manager.memCounters)
			}
			now := time.Unix(1700000000, 0)
			manager.kvNow = func() time.Time { return now }
			var wg sync.WaitGroup
			errs := make(chan error, 16)
			for range 16 {
				wg.Go(func() {
					defer func() {
						if p := recover(); p != nil {
							t.Errorf("counter panicked: %v", p)
						}
					}()
					_, err := manager.counterAdd(context.Background(), "test", "key", 300)
					errs <- err
				})
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}
			count, err := manager.counterGet(context.Background(), "test", "key", 300)
			require.NoError(t, err)
			require.Equal(t, int64(16), count)
			require.NoError(t, manager.Stop(context.Background()))
			_, err = manager.counterAdd(context.Background(), "test", "key", 300)
			require.ErrorContains(t, err, "closed")
			require.Nil(t, manager.redis)
			require.Nil(t, manager.memCounters)
		})
	}
}
