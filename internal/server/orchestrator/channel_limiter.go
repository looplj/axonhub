package orchestrator

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"
)

// Sentinel errors returned by ChannelLimiter.Acquire.
var (
	// ErrChannelQueueFull is returned when the wait queue is full at acquire time.
	ErrChannelQueueFull = errors.New("channel concurrency queue full")
	// ErrChannelQueueTimeout is returned when the per-channel queue timeout elapses
	// while a request is still waiting for a slot.
	ErrChannelQueueTimeout = errors.New("channel concurrency queue wait timeout")
)

// ChannelLimiter provides per-channel admission control with two modes:
//
//   - Soft mode (queueSize == 0): Acquire only counts in-flight requests; it never
//     blocks or rejects. The load balancer uses Stats() to down-rank busy channels.
//   - Hard mode (queueSize > 0): Acquire enforces a strict capacity. Excess requests
//     wait in a FIFO queue of queueSize entries; further arrivals get
//     ErrChannelQueueFull. Each waiter additionally honours an optional timeout
//     (ErrChannelQueueTimeout) and the caller's context.
//
// Caller must ensure capacity > 0.
type ChannelLimiter struct {
	capacity  int
	queueSize int
	timeout   time.Duration

	mu       sync.Mutex
	inFlight int
	waiters  *list.List // FIFO of *slotReq; only used in hard mode.
}

// slotReq is a single waiter in the FIFO queue. Release closes ch to transfer
// slot ownership directly to that waiter.
type slotReq struct {
	ch chan struct{}
}

// NewChannelLimiter creates a limiter. timeoutMs == 0 means "no per-channel timeout"
// and the caller's context becomes the only deadline.
func NewChannelLimiter(capacity, queueSize int, timeoutMs int64) *ChannelLimiter {
	return &ChannelLimiter{
		capacity:  capacity,
		queueSize: queueSize,
		timeout:   time.Duration(timeoutMs) * time.Millisecond,
		waiters:   list.New(),
	}
}

// Acquire requests a slot. Returning nil means a slot was granted and Release
// MUST be called exactly once to give it back. Any non-nil error means no slot
// was acquired and Release must NOT be called.
//
// Possible non-nil errors:
//   - ErrChannelQueueFull when the queue has no remaining capacity at entry time
//   - ErrChannelQueueTimeout when the per-channel timeout elapses while waiting
//   - ctx.Err() when the caller's context is cancelled (overrides the timeout)
func (l *ChannelLimiter) Acquire(ctx context.Context) error {
	l.mu.Lock()

	// Soft mode: no blocking, just bookkeeping.
	if l.queueSize == 0 {
		l.inFlight++
		l.mu.Unlock()

		return nil
	}

	// Hard mode: try to take a slot directly.
	if l.inFlight < l.capacity {
		l.inFlight++
		l.mu.Unlock()

		return nil
	}

	// Capacity exhausted; check whether the queue has room.
	if l.waiters.Len() >= l.queueSize {
		l.mu.Unlock()

		return ErrChannelQueueFull
	}

	req := &slotReq{ch: make(chan struct{})}
	elem := l.waiters.PushBack(req)
	l.mu.Unlock()

	waitCtx := ctx

	var cancel context.CancelFunc
	if l.timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, l.timeout)
		defer cancel()
	}

	select {
	case <-req.ch:
		// Release transferred slot ownership directly; inFlight already accounts for us.
		return nil

	case <-waitCtx.Done():
		// Resolve the race between cancellation and a concurrent grant.
		l.mu.Lock()
		select {
		case <-req.ch:
			// We were granted between Done firing and us locking — own the slot,
			// then hand it back so the next waiter (or inFlight) is correct.
			l.mu.Unlock()
			l.Release()

		default:
			l.waiters.Remove(elem)
			l.mu.Unlock()
		}

		// Distinguish per-channel queue timeout from caller-side cancellation.
		if l.timeout > 0 && errors.Is(waitCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			return ErrChannelQueueTimeout
		}

		return ctx.Err()
	}
}

// Release returns one slot. In hard mode it transfers the slot directly to the
// head waiter (FIFO fairness); when no waiter is queued it decrements inFlight.
// Guards against decrementing below zero so an unmatched Release is a no-op
// rather than a panic.
func (l *ChannelLimiter) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.queueSize > 0 {
		if e := l.waiters.Front(); e != nil {
			l.waiters.Remove(e)
			//nolint:forcetypeassert // PushBack only ever stores *slotReq.
			close(e.Value.(*slotReq).ch)

			return
		}
	}

	if l.inFlight > 0 {
		l.inFlight--
	}
}

// Stats returns the current in-flight and waiting counts. Used by the load
// balancer scoring layer; in soft mode waiting is always zero.
func (l *ChannelLimiter) Stats() (inFlight, waiting int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.inFlight, l.waiters.Len()
}
