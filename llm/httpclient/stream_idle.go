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

type streamIdleTimeoutKey struct{}

// WithStreamIdleTimeout configures the maximum interval without upstream stream
// bytes, including the wait for response headers. Zero disables the timeout.
func WithStreamIdleTimeout(ctx context.Context, timeout time.Duration) context.Context {
	return context.WithValue(ctx, streamIdleTimeoutKey{}, timeout)
}

// NewStreamIdleContext starts an upstream inactivity watchdog. Call activity
// only when upstream bytes arrive, and stop when the attempt finishes. Custom
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
	var timer *time.Timer
	mu.Lock()
	timer = time.AfterFunc(timeout, func() {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
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
		if !stopped {
			deadline = time.Now().Add(timeout)
			timer.Reset(timeout)
		}
	}
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

type streamActivityReader struct {
	io.ReadCloser
	ctx      context.Context
	activity func()
}

func (r *streamActivityReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.activity()
	}
	if err != nil && context.Cause(r.ctx) != nil {
		err = context.Cause(r.ctx)
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

func (s *idleTimeoutStream) Next() bool {
	if s.finished {
		return false
	}
	if s.Stream.Next() {
		return true
	}
	s.finished = true
	s.err = s.Stream.Err()
	if cause := context.Cause(s.ctx); cause != nil {
		s.err = cause
	}
	s.stop()
	return false
}

func (s *idleTimeoutStream) Err() error {
	if !s.finished {
		return s.Stream.Err()
	}
	return s.err
}

func (s *idleTimeoutStream) Close() error {
	s.stop()
	return s.Stream.Close()
}
