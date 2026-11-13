package httpclient

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/looplj/axonhub/llm/streams"
)

// ErrStreamIdleTimeout indicates that an upstream stream stopped sending bytes.
var ErrStreamIdleTimeout = errors.New("upstream stream idle timeout")

type (
	streamIdleTimeoutKey struct{}
	streamIdleWaitingKey struct{}
)

// WithStreamIdleTimeout configures the maximum interval without upstream stream
// bytes while waiting on upstream, including response headers. Zero disables it.
func WithStreamIdleTimeout(ctx context.Context, timeout time.Duration) context.Context {
	return context.WithValue(ctx, streamIdleTimeoutKey{}, timeout)
}

// NewStreamIdleContext starts an upstream inactivity watchdog. Call activity
// only when upstream bytes arrive, and stop when the attempt finishes. Use
// SetStreamIdleWaiting to exclude time spent waiting for the consumer. Custom
// streaming transports use this to share the HTTP transport's timeout policy.
func NewStreamIdleContext(ctx context.Context) (streamCtx context.Context, activity func(), stop func()) {
	timeout, _ := ctx.Value(streamIdleTimeoutKey{}).(time.Duration)
	if timeout <= 0 {
		return ctx, func() {}, func() {}
	}

	streamCtx, cancel := context.WithCancelCause(ctx)
	var mu sync.Mutex
	deadline := time.Now().Add(timeout)
	stopped := false
	waiting := true
	var timer *time.Timer
	mu.Lock()
	timer = time.AfterFunc(timeout, func() {
		mu.Lock()
		defer mu.Unlock()
		if stopped || !waiting {
			return
		}
		// A callback already queued before Reset must honor the latest bytes.
		if remaining := time.Until(deadline); remaining > 0 {
			timer.Reset(remaining)
			return
		}
		stopped = true
		cancel(ErrStreamIdleTimeout)
	})
	mu.Unlock()
	activity = func() {
		mu.Lock()
		defer mu.Unlock()
		if !stopped && waiting {
			deadline = time.Now().Add(timeout)
			timer.Reset(timeout)
		}
	}
	streamCtx = context.WithValue(streamCtx, streamIdleWaitingKey{}, func(active bool) {
		mu.Lock()
		defer mu.Unlock()
		if stopped || waiting == active {
			return
		}
		waiting = active
		if active {
			deadline = time.Now().Add(timeout)
			timer.Reset(timeout)
		} else {
			timer.Stop()
		}
	})
	disarm := func() {
		mu.Lock()
		stopped = true
		timer.Stop()
		mu.Unlock()
	}
	stopOnCancel := context.AfterFunc(streamCtx, disarm)
	stop = func() {
		stopOnCancel()
		disarm()
		cancel(context.Canceled)
	}
	return streamCtx, activity, stop
}

// SetStreamIdleWaiting arms the watchdog only while the transport is waiting
// for upstream data. Resuming grants a full idle interval to the next read.
func SetStreamIdleWaiting(ctx context.Context, waiting bool) {
	if set, ok := ctx.Value(streamIdleWaitingKey{}).(func(bool)); ok {
		set(waiting)
	}
}

// StreamContextError preserves the upstream idle sentinel while retaining the
// standard cancellation errors used by callers to classify parent cancellation.
func StreamContextError(ctx context.Context) error {
	if errors.Is(context.Cause(ctx), ErrStreamIdleTimeout) {
		return ErrStreamIdleTimeout
	}
	return ctx.Err()
}

type streamActivityReader struct {
	io.ReadCloser
	ctx      context.Context
	activity func()
}

// Read counts only time blocked on upstream and refreshes activity for any bytes,
// even when the same read also returns an error.
func (r *streamActivityReader) Read(p []byte) (int, error) {
	SetStreamIdleWaiting(r.ctx, true)
	defer SetStreamIdleWaiting(r.ctx, false)
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.activity()
	}
	if err != nil && StreamContextError(r.ctx) != nil {
		err = StreamContextError(r.ctx)
	}
	return n, err
}

type idleTimeoutStream struct {
	streams.Stream[*StreamEvent]
	ctx      context.Context
	stop     func()
	err      error
	finished bool
}

// Next captures the terminal cause before stopping the watchdog, so cleanup's
// cancellation cannot replace a successful end or an upstream idle timeout.
func (s *idleTimeoutStream) Next() bool {
	if s.finished {
		return false
	}
	if s.Stream.Next() {
		return true
	}
	s.finished = true
	s.err = s.Stream.Err()
	if cause := StreamContextError(s.ctx); cause != nil {
		s.err = cause
	}
	s.stop()
	return false
}

// Err exposes the wrapped error until iteration ends, then returns the cause
// captured before watchdog cleanup.
func (s *idleTimeoutStream) Err() error {
	if !s.finished {
		return s.Stream.Err()
	}
	return s.err
}

// Close stops the watchdog before closing the underlying stream.
func (s *idleTimeoutStream) Close() error {
	s.stop()
	return s.Stream.Close()
}
