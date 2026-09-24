package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func newMultiKeyChannel(keys ...string) *Channel {
	return &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{APIKeys: keys},
		},
		cachedEnabledAPIKeys: keys,
	}
}

func TestRandomKeyProvider_StaysInSet(t *testing.T) {
	keys := []string{"k1", "k2", "k3"}
	p := NewRandomKeyProvider(newMultiKeyChannel(keys...))
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		require.Contains(t, keys, p.Get(ctx))
	}
}

func TestRoundRobinKeyProvider_SwitchEveryCall(t *testing.T) {
	keys := []string{"k1", "k2", "k3"}
	p := NewRoundRobinKeyProvider(newMultiKeyChannel(keys...), 1)
	ctx := context.Background()
	// per=1 -> strict rotation k1,k2,k3,k1,...
	require.Equal(t, "k1", p.Get(ctx))
	require.Equal(t, "k2", p.Get(ctx))
	require.Equal(t, "k3", p.Get(ctx))
	require.Equal(t, "k1", p.Get(ctx))
}

func TestRoundRobinKeyProvider_SwitchAfterN(t *testing.T) {
	keys := []string{"k1", "k2"}
	p := NewRoundRobinKeyProvider(newMultiKeyChannel(keys...), 3)
	ctx := context.Background()
	// per=3 -> each key used 3 times before advancing
	want := []string{"k1", "k1", "k1", "k2", "k2", "k2", "k1"}
	for i, w := range want {
		require.Equalf(t, w, p.Get(ctx), "call %d", i)
	}
}

func TestRoundRobinKeyProvider_MinOne(t *testing.T) {
	keys := []string{"k1", "k2"}
	// per < 1 must be treated as 1.
	p := NewRoundRobinKeyProvider(newMultiKeyChannel(keys...), 0)
	ctx := context.Background()
	require.Equal(t, "k1", p.Get(ctx))
	require.Equal(t, "k2", p.Get(ctx))
}

func TestFixedKeyProvider_AlwaysFirst(t *testing.T) {
	keys := []string{"first", "second", "third"}
	p := NewFixedKeyProvider(newMultiKeyChannel(keys...))
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		require.Equal(t, "first", p.Get(ctx))
	}
}

func TestNewMultiKeyProvider_Dispatch(t *testing.T) {
	ch := newMultiKeyChannel("k1", "k2")

	// nil settings -> sticky (default).
	require.IsType(t, &TraceStickyKeyProvider{}, newMultiKeyProvider(ch))

	cases := []struct {
		strategy string
		want     any
	}{
		{"", &TraceStickyKeyProvider{}},
		{objects.APIKeyStrategySticky, &TraceStickyKeyProvider{}},
		{objects.APIKeyStrategyRandom, &RandomKeyProvider{}},
		{objects.APIKeyStrategyRoundRobin, &RoundRobinKeyProvider{}},
		{objects.APIKeyStrategyFixed, &FixedKeyProvider{}},
		{"unknown_value", &TraceStickyKeyProvider{}},
	}
	for _, tc := range cases {
		strategy := tc.strategy
		ch.Settings = &objects.ChannelSettings{APIKeyStrategy: &strategy}
		require.IsTypef(t, tc.want, newMultiKeyProvider(ch), "strategy %q", tc.strategy)
	}
}
