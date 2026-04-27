package orchestrator

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func makeChannel(id int, rl *objects.ChannelRateLimit) *biz.Channel {
	return &biz.Channel{
		Channel: &ent.Channel{
			ID: id,
			Settings: &objects.ChannelSettings{
				RateLimit: rl,
			},
		},
	}
}

func TestExtractLimiterConfig_Disabled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ch   *biz.Channel
	}{
		{"nil channel", nil},
		{"nil settings", &biz.Channel{Channel: &ent.Channel{ID: 1}}},
		{"nil rate limit", makeChannel(1, nil)},
		{"nil max concurrent", makeChannel(1, &objects.ChannelRateLimit{})},
		{"zero max concurrent", makeChannel(1, &objects.ChannelRateLimit{MaxConcurrent: lo.ToPtr(int64(0))})},
		{"negative max concurrent", makeChannel(1, &objects.ChannelRateLimit{MaxConcurrent: lo.ToPtr(int64(-5))})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := extractLimiterConfig(tc.ch)
			assert.Zero(t, cfg.capacity, "disabled cfg must have zero capacity")
			assert.Zero(t, cfg.queueSize)
			assert.Zero(t, cfg.timeoutMs)
		})
	}
}

func TestExtractLimiterConfig_SoftMode(t *testing.T) {
	t.Parallel()

	ch := makeChannel(1, &objects.ChannelRateLimit{
		MaxConcurrent: lo.ToPtr(int64(5)),
	})
	cfg := extractLimiterConfig(ch)

	assert.Equal(t, 5, cfg.capacity)
	assert.Equal(t, 0, cfg.queueSize, "soft mode")
	assert.Equal(t, int64(0), cfg.timeoutMs)
}

func TestExtractLimiterConfig_HardMode(t *testing.T) {
	t.Parallel()

	ch := makeChannel(1, &objects.ChannelRateLimit{
		MaxConcurrent:  lo.ToPtr(int64(5)),
		QueueSize:      lo.ToPtr(int64(10)),
		QueueTimeoutMs: lo.ToPtr(int64(30000)),
	})
	cfg := extractLimiterConfig(ch)

	assert.Equal(t, 5, cfg.capacity)
	assert.Equal(t, 10, cfg.queueSize)
	assert.Equal(t, int64(30000), cfg.timeoutMs)
}

func TestExtractLimiterConfig_NegativeQueueFieldsTreatedAsUnset(t *testing.T) {
	t.Parallel()

	ch := makeChannel(1, &objects.ChannelRateLimit{
		MaxConcurrent:  lo.ToPtr(int64(5)),
		QueueSize:      lo.ToPtr(int64(-10)),
		QueueTimeoutMs: lo.ToPtr(int64(-100)),
	})
	cfg := extractLimiterConfig(ch)

	// Validation rejects negatives upstream; defensively treat as unset here.
	assert.Equal(t, 5, cfg.capacity)
	assert.Equal(t, 0, cfg.queueSize)
	assert.Equal(t, int64(0), cfg.timeoutMs)
}

func TestLimiterConfig_Equality(t *testing.T) {
	t.Parallel()

	a := limiterConfig{capacity: 5, queueSize: 10, timeoutMs: 30000}
	b := limiterConfig{capacity: 5, queueSize: 10, timeoutMs: 30000}
	c := limiterConfig{capacity: 5, queueSize: 11, timeoutMs: 30000}
	d := limiterConfig{capacity: 5, queueSize: 10, timeoutMs: 0}

	assert.Equal(t, a, b, "identical configs must compare equal")
	assert.NotEqual(t, a, c, "queueSize change must compare unequal")
	assert.NotEqual(t, a, d, "timeoutMs change must compare unequal")
}

func TestChannelLimiterManager_GetOrCreate_Disabled(t *testing.T) {
	t.Parallel()

	m := NewChannelLimiterManager()
	ch := makeChannel(1, nil)

	assert.Nil(t, m.GetOrCreate(ch))
	assert.Nil(t, m.GetOrCreate(makeChannel(1, &objects.ChannelRateLimit{
		MaxConcurrent: lo.ToPtr(int64(0)),
	})))
}

func TestChannelLimiterManager_GetOrCreate_StableInstance(t *testing.T) {
	t.Parallel()

	m := NewChannelLimiterManager()
	ch := makeChannel(1, &objects.ChannelRateLimit{
		MaxConcurrent: lo.ToPtr(int64(5)),
		QueueSize:     lo.ToPtr(int64(10)),
	})

	first := m.GetOrCreate(ch)
	require.NotNil(t, first)

	second := m.GetOrCreate(ch)
	assert.Same(t, first, second, "same config returns same limiter")
}

func TestChannelLimiterManager_GetOrCreate_RebuildsOnConfigChange(t *testing.T) {
	t.Parallel()

	m := NewChannelLimiterManager()

	chA := makeChannel(1, &objects.ChannelRateLimit{
		MaxConcurrent: lo.ToPtr(int64(5)),
		QueueSize:     lo.ToPtr(int64(10)),
	})
	chB := makeChannel(1, &objects.ChannelRateLimit{
		MaxConcurrent: lo.ToPtr(int64(5)),
		QueueSize:     lo.ToPtr(int64(20)), // changed
	})
	chC := makeChannel(1, &objects.ChannelRateLimit{
		MaxConcurrent:  lo.ToPtr(int64(5)),
		QueueSize:      lo.ToPtr(int64(20)),
		QueueTimeoutMs: lo.ToPtr(int64(500)), // newly set
	})

	first := m.GetOrCreate(chA)
	second := m.GetOrCreate(chB)
	third := m.GetOrCreate(chC)

	assert.NotSame(t, first, second, "queueSize change rebuilds limiter")
	assert.NotSame(t, second, third, "timeout change rebuilds limiter")
}

func TestChannelLimiterManager_Forget(t *testing.T) {
	t.Parallel()

	m := NewChannelLimiterManager()
	ch := makeChannel(1, &objects.ChannelRateLimit{
		MaxConcurrent: lo.ToPtr(int64(5)),
	})

	first := m.GetOrCreate(ch)
	require.NotNil(t, first)

	m.Forget(1)

	second := m.GetOrCreate(ch)
	require.NotNil(t, second)
	assert.NotSame(t, first, second, "Forget then GetOrCreate yields a fresh instance")

	// Forgetting an unknown channel must be a no-op.
	m.Forget(9999)
}

func TestChannelLimiterManager_Stats(t *testing.T) {
	t.Parallel()

	m := NewChannelLimiterManager()
	ch := makeChannel(7, &objects.ChannelRateLimit{
		MaxConcurrent: lo.ToPtr(int64(3)),
		QueueSize:     lo.ToPtr(int64(2)),
	})

	// Unknown channel: ok=false.
	inFlight, waiting, ok := m.Stats(7)
	assert.Equal(t, 0, inFlight)
	assert.Equal(t, 0, waiting)
	assert.False(t, ok)

	lim := m.GetOrCreate(ch)
	require.NoError(t, lim.Acquire(t.Context()))
	require.NoError(t, lim.Acquire(t.Context()))

	inFlight, waiting, ok = m.Stats(7)
	assert.Equal(t, 2, inFlight)
	assert.Equal(t, 0, waiting)
	assert.True(t, ok)

	lim.Release()
	lim.Release()
}

func TestChannelLimiterManager_GetOrCreate_DropsStaleEntryWhenDisabled(t *testing.T) {
	t.Parallel()

	m := NewChannelLimiterManager()

	// First, create a limiter for the channel.
	enabled := makeChannel(42, &objects.ChannelRateLimit{
		MaxConcurrent: lo.ToPtr(int64(5)),
	})
	require.NotNil(t, m.GetOrCreate(enabled))

	_, _, ok := m.Stats(42)
	require.True(t, ok, "stats reports channel as tracked while limiter exists")

	// Same channel ID, but rate limit cleared (user disabled MaxConcurrent).
	disabled := makeChannel(42, nil)
	assert.Nil(t, m.GetOrCreate(disabled))

	_, _, ok = m.Stats(42)
	assert.False(t, ok, "stale entry must be dropped so Stats reports unlimited")

	// Snapshot should also no longer report the channel.
	for _, snap := range m.Snapshot() {
		assert.NotEqual(t, 42, snap.ChannelID, "Snapshot must not include dropped channel")
	}
}

func TestChannelLimiterManager_GetOrCreate_DistinctChannels(t *testing.T) {
	t.Parallel()

	m := NewChannelLimiterManager()
	c1 := makeChannel(1, &objects.ChannelRateLimit{MaxConcurrent: lo.ToPtr(int64(2))})
	c2 := makeChannel(2, &objects.ChannelRateLimit{MaxConcurrent: lo.ToPtr(int64(2))})

	assert.NotSame(t, m.GetOrCreate(c1), m.GetOrCreate(c2),
		"different channels get different limiters even with identical config")
}
