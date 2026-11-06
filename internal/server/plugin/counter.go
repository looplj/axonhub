package plugin

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/looplj/axonhub/internal/pkg/xcache"
)

const (
	onEventExport       = "onEvent"
	onEventTimeout      = 10 * time.Second
	maxWritesPerCall    = 10
	maxWritesPerMinute  = 60
	counterWindowMin    = 1
	counterWindowMax    = 24 * 60 * 60
	counterWideAfter    = 60
	counterWideBucket   = 10
	pluginCounterPrefix = "axonhub:plugin:cnt:"
	pluginActionReason  = "plugin action"
	channelListDefault  = 100
	channelListMax      = 100
)

// memoryCounters stores only occupied buckets and maintains their running total.
// No worker is needed: accessed series expire incrementally, and idle series
// are swept at most once per minute while counters are in use.
type memoryCounters struct {
	mu        sync.Mutex
	series    map[string]*memorySeries
	nextSweep time.Time
}

type memorySeries struct {
	buckets []memoryBucket
	head    int
	total   int64
	expires time.Time
}

type memoryBucket struct {
	unix    int64
	count   int64
	expires time.Time
}

func newMemoryCounters() *memoryCounters {
	return &memoryCounters{series: map[string]*memorySeries{}}
}

func (c *memoryCounters) add(now time.Time, pluginID, key string, window int) int64 {
	return c.addWithClock(func() time.Time { return now }, pluginID, key, window)
}

func (c *memoryCounters) addWithClock(clock func() time.Time, pluginID, key string, window int) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Sample the clock after locking so concurrent callers append in time order.
	now := clock()
	c.sweepLocked(now)
	name := counterSeries(pluginID, key, window)
	series := c.series[name]
	if series == nil {
		series = &memorySeries{}
		c.series[name] = series
	}
	series.expire(now, window)
	current := alignedBucket(now, counterBucketSize(window))
	n := len(series.buckets)
	if n == 0 || series.buckets[n-1].unix != current {
		series.buckets = append(series.buckets, memoryBucket{unix: current})
	}
	bucket := &series.buckets[len(series.buckets)-1]
	bucket.count++
	bucket.expires = now.Add(time.Duration(window+1) * time.Second)
	series.expires = bucket.expires
	series.total++
	return series.total
}

func (c *memoryCounters) get(now time.Time, pluginID, key string, window int) int64 {
	return c.getWithClock(func() time.Time { return now }, pluginID, key, window)
}

func (c *memoryCounters) getWithClock(clock func() time.Time, pluginID, key string, window int) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := clock()
	c.sweepLocked(now)
	name := counterSeries(pluginID, key, window)
	series := c.series[name]
	if series == nil {
		return 0
	}
	series.expire(now, window)
	if series.total == 0 {
		delete(c.series, name)
	}
	return series.total
}

func (c *memoryCounters) sweepLocked(now time.Time) {
	if now.Before(c.nextSweep) {
		return
	}
	for name, series := range c.series {
		if !series.expires.After(now) {
			delete(c.series, name)
		}
	}
	c.nextSweep = now.Add(time.Minute)
}

func (s *memorySeries) expire(now time.Time, window int) {
	size := counterBucketSize(window)
	count := max((window+size-1)/size, 1)
	cutoff := alignedBucket(now, size) - int64((count-1)*size)
	for s.head < len(s.buckets) {
		bucket := s.buckets[s.head]
		if bucket.unix >= cutoff && bucket.expires.After(now) {
			break
		}
		s.total -= bucket.count
		s.head++
	}
	// Compact only after consuming at least half the slice, keeping work amortized
	// and memory proportional to the occupied window rather than process uptime.
	if s.head > 0 && s.head >= len(s.buckets)-s.head {
		s.buckets = append([]memoryBucket(nil), s.buckets[s.head:]...)
		s.head = 0
	}
}

func counterBucketSize(window int) int {
	if window > counterWideAfter {
		return counterWideBucket
	}
	return 1
}

func alignedBucket(now time.Time, size int) int64 {
	unix := now.Unix()
	if size <= 1 {
		return unix
	}
	return unix - unix%int64(size)
}

func counterSeries(pluginID, key string, window int) string {
	return pluginCounterPrefix +
		url.QueryEscape(pluginID) + ":" +
		url.QueryEscape(key) + ":" +
		strconv.Itoa(window)
}

func (m *Manager) countersUseRedis() bool {
	if m == nil {
		return false
	}
	return m.cacheMode == xcache.ModeRedis || m.cacheMode == xcache.ModeTwoLevel
}

func (m *Manager) counterBackend() (*memoryCounters, redis.UniversalClient, error) {
	m.counterMu.Lock()
	defer m.counterMu.Unlock()
	if m.countersClosed {
		return nil, nil, errors.New("plugin counters are closed")
	}
	if m.countersUseRedis() {
		if m.redis == nil {
			return nil, nil, errors.New("plugin counter redis is not configured")
		}
		return nil, m.redis, nil
	}
	if m.memCounters == nil {
		return nil, nil, errors.New("plugin counter is not configured")
	}
	return m.memCounters, nil, nil
}

func (m *Manager) counterAdd(ctx context.Context, pluginID, key string, window int) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	memory, client, err := m.counterBackend()
	if err != nil {
		return 0, err
	}
	if client == nil {
		return memory.addWithClock(m.now, pluginID, key, window), nil
	}
	return redisCounter(ctx, client, pluginID, key, window, true)
}

func (m *Manager) counterGet(ctx context.Context, pluginID, key string, window int) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	memory, client, err := m.counterBackend()
	if err != nil {
		return 0, err
	}
	if client == nil {
		return memory.getWithClock(m.now, pluginID, key, window), nil
	}
	return redisCounter(ctx, client, pluginID, key, window, false)
}

func (m *Manager) closeRedis() {
	if m == nil {
		return
	}
	m.counterMu.Lock()
	m.countersClosed = true
	client := m.redis
	m.redis = nil
	m.memCounters = nil
	m.counterMu.Unlock()
	if client != nil {
		_ = client.Close()
	}
}

// pluginWriteLimiter is the per-plugin sliding window of host write calls.
type pluginWriteLimiter struct {
	mu    sync.Mutex
	calls map[string][]time.Time
}

func newPluginWriteLimiter() *pluginWriteLimiter {
	return &pluginWriteLimiter{calls: map[string][]time.Time{}}
}

func (l *pluginWriteLimiter) allow(now time.Time, pluginID string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-time.Minute)
	recent := l.calls[pluginID]
	kept := recent[:0]
	for _, at := range recent {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) >= maxWritesPerMinute {
		l.calls[pluginID] = kept
		return false
	}
	l.calls[pluginID] = append(kept, now)
	return true
}

func (m *Manager) allowWrite(pluginID string) bool {
	if m == nil || m.writes == nil {
		return false
	}
	return m.writes.allow(m.now(), pluginID)
}
