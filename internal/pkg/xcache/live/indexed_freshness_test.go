package live

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIndexedCacheSlowLoadCannotRenewSnapshot(t *testing.T) {
	cache := NewIndexedCache(IndexedOptions[string, string]{
		Name: "freshness", TTL: time.Millisecond, RefreshInterval: time.Hour,
		KeyFunc: func(v string) string { return v },
		LoadOneFunc: func(context.Context, string) (string, error) {
			time.Sleep(5 * time.Millisecond)
			return "key", nil
		},
		LoadSinceFunc: func(context.Context, time.Time) ([]string, time.Time, error) {
			time.Sleep(5 * time.Millisecond)
			return []string{"key"}, time.Now(), nil
		},
	})
	defer cache.Stop()
	ctx := context.Background()
	_, err := cache.Get(ctx, "key")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NoError(t, cache.Load(ctx))
	_, found := cache.GetCached("key")
	require.False(t, found)
	require.NoError(t, cache.inner.Load(ctx, true))
	_, found = cache.GetCached("key")
	require.False(t, found)
}
