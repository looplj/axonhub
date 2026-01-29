package watcher

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestMemoryWatcher_BroadcastAndUnsubscribe(t *testing.T) {
	w := NewMemoryWatcher[int](MemoryWatcherOptions{Buffer: 1})

	ch1, stop1 := w.Watch()

	require.NoError(t, w.Notify(context.Background(), 42))

	select {
	case v := <-ch1:
		require.Equal(t, 42, v)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ch1")
	}

	stop1()

	select {
	case _, ok := <-ch1:
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ch1 close")
	}
}

func TestRedisWatcher_BroadcastAcrossInstances(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	w1, err := NewRedisWatcher[struct{}](client, RedisWatcherOptions{Channel: "xcache:test", Buffer: 1})
	require.NoError(t, err)
	w2, err := NewRedisWatcher[struct{}](client, RedisWatcherOptions{Channel: "xcache:test", Buffer: 1})
	require.NoError(t, err)

	ch1, stop1 := w1.Watch()
	ch2, stop2 := w2.Watch()

	defer stop1()
	defer stop2()

	require.NoError(t, w1.Notify(context.Background(), struct{}{}))

	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ch1")
	}

	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ch2")
	}
}
