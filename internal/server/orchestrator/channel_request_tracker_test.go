package orchestrator

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewChannelRequestTracker(t *testing.T) {
	tracker := NewChannelRequestTracker()
	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.counters)
}

func TestChannelRequestTracker_IncrementRequest(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.IncrementRequest(1)
	tracker.IncrementRequest(1)
	tracker.IncrementRequest(1)

	assert.Equal(t, int64(3), tracker.GetRequestCount(1))
}

func TestChannelRequestTracker_AddTokens(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.AddTokens(1, 100)
	tracker.AddTokens(1, 200)

	assert.Equal(t, int64(300), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_AddTokens_IgnoresNonPositive(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.AddTokens(1, 0)
	tracker.AddTokens(1, -10)

	assert.Equal(t, int64(0), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_GetRequestCount_UnknownChannel(t *testing.T) {
	tracker := NewChannelRequestTracker()
	assert.Equal(t, int64(0), tracker.GetRequestCount(999))
}

func TestChannelRequestTracker_GetTokenCount_UnknownChannel(t *testing.T) {
	tracker := NewChannelRequestTracker()
	assert.Equal(t, int64(0), tracker.GetTokenCount(999))
}

func TestChannelRequestTracker_MultipleChannels(t *testing.T) {
	tracker := NewChannelRequestTracker()

	tracker.IncrementRequest(1)
	tracker.IncrementRequest(1)
	tracker.IncrementRequest(2)
	tracker.AddTokens(1, 100)
	tracker.AddTokens(2, 500)

	assert.Equal(t, int64(2), tracker.GetRequestCount(1))
	assert.Equal(t, int64(1), tracker.GetRequestCount(2))
	assert.Equal(t, int64(100), tracker.GetTokenCount(1))
	assert.Equal(t, int64(500), tracker.GetTokenCount(2))
}

func TestChannelRequestTracker_WindowReset(t *testing.T) {
	tracker := NewChannelRequestTracker()

	// Manually insert a window that belongs to a past minute
	tracker.mu.Lock()
	tracker.counters[1] = &rateLimitWindow{
		requests:    10,
		tokens:      500,
		windowStart: time.Now().Truncate(time.Minute).Add(-time.Minute),
	}
	tracker.mu.Unlock()

	// Reads should return 0 because the window is expired
	assert.Equal(t, int64(0), tracker.GetRequestCount(1))
	assert.Equal(t, int64(0), tracker.GetTokenCount(1))

	// Writes should create a new window
	tracker.IncrementRequest(1)
	assert.Equal(t, int64(1), tracker.GetRequestCount(1))
	assert.Equal(t, int64(0), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_WindowReset_AddTokens(t *testing.T) {
	tracker := NewChannelRequestTracker()

	// Manually insert a window that belongs to a past minute
	tracker.mu.Lock()
	tracker.counters[1] = &rateLimitWindow{
		requests:    10,
		tokens:      500,
		windowStart: time.Now().Truncate(time.Minute).Add(-time.Minute),
	}
	tracker.mu.Unlock()

	tracker.AddTokens(1, 200)
	assert.Equal(t, int64(200), tracker.GetTokenCount(1))
	// Requests should have been reset too since AddTokens creates a new window
	assert.Equal(t, int64(0), tracker.GetRequestCount(1))
}

func TestChannelRequestTracker_Concurrent(t *testing.T) {
	tracker := NewChannelRequestTracker()

	const (
		goroutines      = 100
		opsPerGoroutine = 100
	)

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Concurrent IncrementRequest
	for range goroutines {
		go func() {
			defer wg.Done()

			for range opsPerGoroutine {
				tracker.IncrementRequest(1)
			}
		}()
	}

	// Concurrent AddTokens
	for range goroutines {
		go func() {
			defer wg.Done()

			for range opsPerGoroutine {
				tracker.AddTokens(1, 10)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(goroutines*opsPerGoroutine), tracker.GetRequestCount(1))
	assert.Equal(t, int64(goroutines*opsPerGoroutine*10), tracker.GetTokenCount(1))
}

func TestChannelRequestTracker_ConcurrentReadWrite(t *testing.T) {
	tracker := NewChannelRequestTracker()

	var wg sync.WaitGroup
	wg.Add(3)

	// Writer
	go func() {
		defer wg.Done()

		for range 1000 {
			tracker.IncrementRequest(1)
		}
	}()

	// Reader 1
	go func() {
		defer wg.Done()

		for range 1000 {
			_ = tracker.GetRequestCount(1)
		}
	}()

	// Reader 2
	go func() {
		defer wg.Done()

		for range 1000 {
			_ = tracker.GetTokenCount(1)
		}
	}()

	wg.Wait()

	assert.Equal(t, int64(1000), tracker.GetRequestCount(1))
}
