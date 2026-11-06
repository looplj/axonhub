package plugin

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestRedisCounterMatchesMemoryWindows(t *testing.T) {
	for _, window := range []int{1, 10, 60, 61, 300, 86400} {
		t.Run(fmt.Sprint(window), func(t *testing.T) {
			mr := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			defer client.Close()
			memory := newMemoryCounters()
			now := time.Unix(1700000000, 0)
			random := rand.New(rand.NewPCG(1, 2))
			for i := range 200 {
				elapsed := time.Duration(random.IntN(window+1)) * 100 * time.Millisecond
				now = now.Add(elapsed)
				mr.FastForward(elapsed)
				mr.SetTime(now)
				add := i%3 != 0
				var want int64
				if add {
					want = memory.add(now, "test", "key", window)
				} else {
					want = memory.get(now, "test", "key", window)
				}
				got, err := redisCounter(t.Context(), client, "test", "key", window, add)
				require.NoError(t, err)
				require.Equal(t, want, got, "step %d at %s", i, now)
			}
			mr.FastForward(25 * time.Hour)
			require.Empty(t, mr.Keys())
		})
	}
}

func TestRedisCounterConcurrentInstances(t *testing.T) {
	mr := miniredis.RunT(t)
	now := time.Unix(1700000000, 0)
	mr.SetTime(now)
	managers := []*Manager{
		{cacheMode: xcache.ModeRedis, redis: redis.NewClient(&redis.Options{Addr: mr.Addr()}), kvNow: func() time.Time { return now.Add(-time.Hour) }},
		{cacheMode: xcache.ModeRedis, redis: redis.NewClient(&redis.Options{Addr: mr.Addr()}), kvNow: func() time.Time { return now.Add(time.Hour) }},
	}
	for _, manager := range managers {
		defer manager.closeRedis()
	}
	var wg sync.WaitGroup
	values := make([]int64, 100)
	errs := make([]error, len(values))
	for i := range values {
		wg.Go(func() {
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("counter panicked: %v", p)
				}
			}()
			values[i], errs[i] = managers[i%2].counterAdd(context.Background(), "test", "key", 300)
		})
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	slices.Sort(values)
	for i, value := range values {
		require.Equal(t, int64(i+1), value)
	}
	for _, manager := range managers {
		value, err := manager.counterGet(t.Context(), "test", "key", 300)
		require.NoError(t, err)
		require.Equal(t, int64(100), value)
	}
}

func TestRedisCounterReadTTLAndIsolation(t *testing.T) {
	mr := miniredis.RunT(t)
	now := time.Unix(1700000000, 0)
	mr.SetTime(now)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	value, err := redisCounter(t.Context(), client, "test", "key", 86400, false)
	require.NoError(t, err)
	require.Zero(t, value)
	require.Empty(t, mr.Keys())
	for range 20 {
		_, err = redisCounter(t.Context(), client, "test", "key", 86400, true)
		require.NoError(t, err)
	}
	keys := counterRedisKeys("test", "key", 86400)
	require.ElementsMatch(t, keys, mr.Keys())
	// Repeated reads leave both TTLs alone and do not allocate empty buckets.
	mr.FastForward(time.Second)
	mr.SetTime(now.Add(time.Second))
	for range 20 {
		value, err = redisCounter(t.Context(), client, "test", "key", 86400, false)
		require.NoError(t, err)
		require.Equal(t, int64(20), value)
	}
	for _, key := range keys {
		require.Equal(t, 24*time.Hour, mr.TTL(key))
	}
	require.EqualValues(t, 2, client.HLen(t.Context(), keys[0]).Val()) // one bucket + total
	require.EqualValues(t, 1, client.ZCard(t.Context(), keys[1]).Val())
	for _, identity := range []struct {
		plugin, key string
		window      int
	}{{"other", "key", 86400}, {"test", "other", 86400}, {"test", "key", 300}} {
		value, err = redisCounter(t.Context(), client, identity.plugin, identity.key, identity.window, false)
		require.NoError(t, err)
		require.Zero(t, value)
	}
	// Losing one half to Redis eviction must not leave an immortal stale total.
	require.NoError(t, client.Del(t.Context(), keys[1]).Err())
	value, err = redisCounter(t.Context(), client, "test", "key", 86400, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), value)
}

func TestRedisCounterClusterHashTags(t *testing.T) {
	keys := counterRedisKeys("plugin:{one}", "key:{two}", 300)
	tag := func(key string) string {
		start := strings.IndexByte(key, '{')
		end := strings.IndexByte(key, '}')
		require.Greater(t, end, start)
		return key[start+1 : end]
	}
	require.Equal(t, tag(keys[0]), tag(keys[1]))
	require.NotContains(t, tag(keys[0]), "{")
	require.NotEqual(t, keys, counterRedisKeys("plugin", "{one}:key:{two}", 300))
}
