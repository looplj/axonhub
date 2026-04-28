package orchestrator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelLimiter_SoftMode_NeverBlocks(t *testing.T) {
	t.Parallel()

	lim := NewChannelLimiter(5, 0, 0) // capacity 5, soft mode

	const n = 100

	for range n {
		require.NoError(t, lim.Acquire(t.Context()))
	}

	inFlight, waiting := lim.Stats()
	assert.Equal(t, n, inFlight, "soft mode counts every Acquire")
	assert.Equal(t, 0, waiting, "soft mode never queues")

	for range n {
		lim.Release()
	}

	inFlight, waiting = lim.Stats()
	assert.Equal(t, 0, inFlight)
	assert.Equal(t, 0, waiting)
}

func TestChannelLimiter_HardMode_AcquireUpToCapacity(t *testing.T) {
	t.Parallel()

	lim := NewChannelLimiter(3, 5, 0)

	for range 3 {
		require.NoError(t, lim.Acquire(t.Context()))
	}

	inFlight, waiting := lim.Stats()
	assert.Equal(t, 3, inFlight)
	assert.Equal(t, 0, waiting)
}

func TestChannelLimiter_HardMode_QueueFull(t *testing.T) {
	t.Parallel()

	lim := NewChannelLimiter(1, 2, 0)

	// Use a cancellable context so we can unblock the queued waiters at the end.
	waitCtx, cancelWait := context.WithCancel(t.Context())
	defer cancelWait()

	// 1 in-flight + 2 queued = at capacity ceiling
	require.NoError(t, lim.Acquire(t.Context()))
	defer lim.Release()

	enqueued := launchAcquire(t, lim, waitCtx, 2)

	require.Eventually(t, func() bool {
		_, waiting := lim.Stats()
		return waiting == 2
	}, time.Second, 5*time.Millisecond)

	// Next Acquire must hit the queue full path immediately.
	err := lim.Acquire(t.Context())
	assert.ErrorIs(t, err, ErrChannelQueueFull)

	// Unblock the two waiters (they should observe ctx cancellation) and drain results.
	cancelWait()
	drainGrants(t, lim, enqueued)
}

func TestChannelLimiter_HardMode_QueueTimeout(t *testing.T) {
	t.Parallel()

	lim := NewChannelLimiter(1, 5, 50) // 50ms timeout
	ctx := t.Context()

	require.NoError(t, lim.Acquire(ctx))

	start := time.Now()
	err := lim.Acquire(ctx)
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, ErrChannelQueueTimeout)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)

	// Slot must not have leaked.
	lim.Release()

	inFlight, waiting := lim.Stats()
	assert.Equal(t, 0, inFlight)
	assert.Equal(t, 0, waiting)
}

func TestChannelLimiter_HardMode_NoTimeoutHonoursContext(t *testing.T) {
	t.Parallel()

	lim := NewChannelLimiter(1, 5, 0) // no per-channel timeout
	ctx, cancel := context.WithCancel(t.Context())

	require.NoError(t, lim.Acquire(ctx))

	errCh := make(chan error, 1)

	go func() {
		errCh <- lim.Acquire(ctx)
	}()

	require.Eventually(t, func() bool {
		_, waiting := lim.Stats()
		return waiting == 1
	}, time.Second, 5*time.Millisecond)

	cancel()

	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, context.Canceled)
		assert.NotErrorIs(t, err, ErrChannelQueueTimeout, "ctx cancellation must not be reported as queue timeout")

	case <-time.After(time.Second):
		t.Fatal("Acquire did not return after ctx cancel")
	}

	lim.Release()
	inFlight, waiting := lim.Stats()
	assert.Equal(t, 0, inFlight)
	assert.Equal(t, 0, waiting)
}

func TestChannelLimiter_HardMode_FIFOFairness(t *testing.T) {
	t.Parallel()

	const waiters = 20

	lim := NewChannelLimiter(1, waiters, 0)
	ctx := t.Context()

	require.NoError(t, lim.Acquire(ctx))

	gotOrder := make([]int, 0, waiters)

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	// Launch waiters one at a time, blocking until each one is enqueued, so the
	// FIFO order is deterministic regardless of goroutine scheduling.
	for i := range waiters {
		wg.Go(func() {
			require.NoError(t, lim.Acquire(ctx))

			mu.Lock()
			gotOrder = append(gotOrder, i)
			mu.Unlock()

			lim.Release()
		})

		require.Eventually(t, func() bool {
			_, waiting := lim.Stats()
			return waiting == i+1
		}, time.Second, time.Millisecond, "waiter %d should enqueue", i)
	}

	// Release the initial slot — FIFO chain drains in insertion order.
	lim.Release()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, gotOrder, waiters)
	for i, idx := range gotOrder {
		assert.Equal(t, i, idx, "waiters should drain in FIFO order")
	}
}

func TestChannelLimiter_NoSlotLeakOnTimeout(t *testing.T) {
	t.Parallel()

	const (
		capacity  = 2
		queueSize = 10
		waiters   = 30
	)

	lim := NewChannelLimiter(capacity, queueSize, 30) // 30ms timeout

	// Saturate capacity so subsequent Acquire calls must queue.
	for range capacity {
		require.NoError(t, lim.Acquire(t.Context()))
	}

	var (
		wg         sync.WaitGroup
		timeouts   atomic.Int64
		queueFulls atomic.Int64
		acquired   atomic.Int64
		ctxCancels atomic.Int64
		other      atomic.Int64
	)

	for range waiters {
		wg.Go(func() {
			err := lim.Acquire(t.Context())
			switch {
			case err == nil:
				acquired.Add(1)
				lim.Release()
			case errors.Is(err, ErrChannelQueueTimeout):
				timeouts.Add(1)
			case errors.Is(err, ErrChannelQueueFull):
				queueFulls.Add(1)
			case errors.Is(err, context.Canceled):
				ctxCancels.Add(1)
			default:
				other.Add(1)
			}
		})
	}

	wg.Wait()

	// Drain the originally-held slots.
	for range capacity {
		lim.Release()
	}

	inFlight, waiting := lim.Stats()
	assert.Equal(t, 0, inFlight, "no slot leak: inFlight must be zero (timeouts=%d, full=%d, acquired=%d)", timeouts.Load(), queueFulls.Load(), acquired.Load())
	assert.Equal(t, 0, waiting, "no waiter leak")
	assert.Zero(t, ctxCancels.Load(), "ctx never cancelled in this test")
	assert.Zero(t, other.Load(), "no unexpected error class")
}

func TestChannelLimiter_ReleaseOnEmptyIsNoop(t *testing.T) {
	t.Parallel()

	lim := NewChannelLimiter(2, 2, 0)
	// Release without prior Acquire must not panic or go negative.
	lim.Release()
	lim.Release()

	inFlight, waiting := lim.Stats()
	assert.Equal(t, 0, inFlight)
	assert.Equal(t, 0, waiting)
}

func TestChannelLimiter_HardMode_ReleaseTransfersToWaiter(t *testing.T) {
	t.Parallel()

	lim := NewChannelLimiter(1, 2, 0)
	ctx := t.Context()

	require.NoError(t, lim.Acquire(ctx))

	// One waiter queued.
	got := make(chan struct{})

	go func() {
		require.NoError(t, lim.Acquire(ctx))
		close(got)
	}()

	require.Eventually(t, func() bool {
		_, waiting := lim.Stats()
		return waiting == 1
	}, time.Second, 5*time.Millisecond)

	lim.Release()

	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("waiter was not granted slot via Release")
	}

	// inFlight should still be 1 — the slot was transferred, not freed.
	inFlight, waiting := lim.Stats()
	assert.Equal(t, 1, inFlight)
	assert.Equal(t, 0, waiting)

	lim.Release()
	inFlight, _ = lim.Stats()
	assert.Equal(t, 0, inFlight)
}

func TestChannelLimiter_HardMode_AlreadyCancelledCtx(t *testing.T) {
	t.Parallel()

	lim := NewChannelLimiter(1, 5, 0)
	ctx, cancel := context.WithCancel(t.Context())

	// Saturate capacity so the next Acquire must queue.
	require.NoError(t, lim.Acquire(ctx))

	cancel()

	err := lim.Acquire(ctx)
	assert.ErrorIs(t, err, context.Canceled)

	lim.Release()
	inFlight, waiting := lim.Stats()
	assert.Equal(t, 0, inFlight)
	assert.Equal(t, 0, waiting)
}

// launchAcquire fires n background Acquire calls. The returned channel emits exactly
// n results, one per goroutine, then closes — drainGrants releases any granted slots.
func launchAcquire(t *testing.T, lim *ChannelLimiter, ctx context.Context, n int) <-chan error {
	t.Helper()

	out := make(chan error, n)

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			out <- lim.Acquire(ctx)
		})
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// drainGrants reads every result from launchAcquire and releases any slot that
// was granted, so the limiter ends the test in a clean state.
func drainGrants(t *testing.T, lim *ChannelLimiter, ch <-chan error) {
	t.Helper()

	for err := range ch {
		if err == nil {
			lim.Release()
		}
	}
}
