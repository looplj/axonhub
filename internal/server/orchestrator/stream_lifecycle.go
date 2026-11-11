package orchestrator

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/server/lifecycle"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

const (
	streamTailTimeout      = time.Second
	streamTailEventLimit   = 256
	streamTailPayloadLimit = 1 << 20
)

type streamLifecycleContextKey struct{}

func withStreamLifecycle(ctx context.Context, owner *streamLifecycle) context.Context {
	return context.WithValue(ctx, streamLifecycleContextKey{}, owner)
}

func streamLifecycleFromContext(ctx context.Context) *streamLifecycle {
	owner, _ := ctx.Value(streamLifecycleContextKey{}).(*streamLifecycle)
	return owner
}

// streamLifecycle separates downstream cancellation from the execution
// context. Before acknowledgement, downstream cancellation aborts execution;
// after acknowledgement it only starts the bounded tail budget, so trailing
// usage can still be collected.
//
//nolint:containedctx // The lifecycle owns the request and execution contexts.
type streamLifecycle struct {
	mu sync.Mutex

	downstream     context.Context
	shutdown       context.Context
	execution      context.Context
	cancel         context.CancelCauseFunc
	deadlineCancel context.CancelFunc
	stopDownstream func() bool
	stopShutdown   func() bool

	acknowledged bool
	canceled     bool
	ended        bool
	closed       bool
	finalized    chan struct{}

	// attempt is the outbound stream of the current upstream attempt. Its Close
	// collects the tail and finalizes accounting.
	attempt *OutboundPersistentStream

	// tail bounds post-acknowledgement collection. When it expires it aborts
	// the attempt's source, releasing a consumer blocked on a silent upstream.
	tail          context.Context
	tailCancel    context.CancelFunc
	stopTailAbort func() bool
}

type streamTailResult struct {
	end llm.SourceEndReason
	err error
}

func newStreamLifecycle(ctx context.Context) (*streamLifecycle, context.Context) {
	base := context.WithoutCancel(ctx)
	var deadlineCancel context.CancelFunc
	if deadline, ok := ctx.Deadline(); ok {
		base, deadlineCancel = context.WithDeadlineCause(base, deadline, context.DeadlineExceeded)
	}
	execution, cancel := context.WithCancelCause(base)

	shutdown := context.Context(nil)
	if manager := lifecycle.FromContext(ctx); manager != nil {
		shutdown = manager.Context()
	}

	owner := &streamLifecycle{
		downstream:     ctx,
		shutdown:       shutdown,
		execution:      execution,
		cancel:         cancel,
		deadlineCancel: deadlineCancel,
		finalized:      make(chan struct{}),
	}
	owner.stopDownstream = context.AfterFunc(ctx, func() { owner.downstreamEnded(ctx.Err()) })
	if shutdown != nil {
		owner.stopShutdown = context.AfterFunc(shutdown, func() { owner.downstreamEnded(context.Cause(shutdown)) })
	}

	return owner, withStreamLifecycle(execution, owner)
}

func (s *streamLifecycle) downstreamEnded(cause error) {
	s.mu.Lock()
	if s.closed || s.canceled || s.ended {
		s.mu.Unlock()
		return
	}
	// Only the first of downstream end and shutdown decides the outcome.
	s.ended = true
	shutdown := s.shutdown != nil && context.Cause(s.shutdown) != nil && errors.Is(cause, context.Cause(s.shutdown))
	if s.acknowledged && cause != nil && !shutdown && !errors.Is(cause, context.DeadlineExceeded) {
		s.tailContextLocked()
		s.mu.Unlock()
		return
	}
	s.canceled = true
	s.cancel(normalizeCancelCause(cause))
	attempt := s.attempt
	s.mu.Unlock()
	if attempt != nil {
		attempt.abortSource()
	}
}

// tailContextLocked starts the shared tail budget on first use.
func (s *streamLifecycle) tailContextLocked() context.Context {
	if s.tail == nil {
		s.tail, s.tailCancel = context.WithTimeout(s.execution, streamTailTimeout)
		if attempt := s.attempt; attempt != nil {
			s.stopTailAbort = context.AfterFunc(s.tail, attempt.abortSource)
		}
	}
	return s.tail
}

// tailContext returns the budget for collecting an attempt's remaining source
// after its consumer stopped reading, and whether delivery was acknowledged.
func (s *streamLifecycle) tailContext() (context.Context, bool, context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acknowledged {
		return s.tailContextLocked(), true, func() {}
	}
	ctx, cancel := context.WithTimeout(s.execution, streamTailTimeout)
	return ctx, false, cancel
}

func (s *streamLifecycle) trackAttempt(attempt *OutboundPersistentStream) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempt = attempt
}

// closeAttempt finalizes the current attempt before a retry resets the shared
// persistence state. Not every failure path closes the attempt's stream.
func (s *streamLifecycle) closeAttempt() {
	s.mu.Lock()
	attempt := s.attempt
	s.attempt = nil
	s.mu.Unlock()
	if attempt != nil {
		_ = attempt.Close()
	}
}

func normalizeCancelCause(cause error) error {
	if cause == nil {
		return context.Canceled
	}
	return cause
}

func (s *streamLifecycle) Context() context.Context { return s.execution }

func (s *streamLifecycle) AcknowledgeDelivery() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.acknowledged || s.canceled || s.downstream.Err() != nil {
		return
	}
	// Deliver holds this lock across the terminal write and records successful
	// delivery before releasing it. A direct acknowledgement is valid only
	// while the downstream is still active.
	s.acknowledged = true
}

func (s *streamLifecycle) Deliver(ctx context.Context, write func() (bool, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.canceled {
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.shutdown != nil {
		if err := context.Cause(s.shutdown); err != nil {
			return err
		}
	}
	terminal, err := write()
	if err == nil && terminal {
		s.acknowledged = true
	}
	return err
}

func (s *streamLifecycle) DeliveryAcknowledged() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acknowledged
}

// Close must be called by the consumer after it stopped reading: it finalizes
// the current attempt on the calling goroutine.
func (s *streamLifecycle) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	attempt := s.attempt
	if !s.canceled && (!s.acknowledged || attempt == nil) {
		s.canceled = true
		s.cancel(context.Canceled)
	}
	s.mu.Unlock()

	if attempt != nil {
		_ = attempt.Close()
	}

	s.stopDownstream()
	if s.stopShutdown != nil {
		s.stopShutdown()
	}
	s.mu.Lock()
	if s.stopTailAbort != nil {
		s.stopTailAbort()
	}
	if s.tailCancel != nil {
		s.tailCancel()
	}
	s.mu.Unlock()
	if s.deadlineCancel != nil {
		s.deadlineCancel()
	}
	close(s.finalized)
}

func (s *streamLifecycle) WaitForFinalization() {
	<-s.finalized
}

// collectStreamTail drains one retained source. It is intentionally
// synchronous: callers invoke it only after the source reader has stopped.
// The caller owns the deadline and must close the source to unblock Next.
func collectStreamTail(ctx context.Context, source streams.Stream[*httpclient.StreamEvent], onEvent func(*httpclient.StreamEvent) error) streamTailResult {
	if source == nil {
		return streamTailResult{end: llm.SourceEndCleanEOF}
	}

	events := 0
	bytes := 0
	for events < streamTailEventLimit && bytes < streamTailPayloadLimit {
		select {
		case <-ctx.Done():
			return streamTailResult{end: llm.SourceEndCanceled, err: ctx.Err()}
		default:
		}
		if !source.Next() {
			err := source.Err()
			if err == nil || errors.Is(err, io.EOF) {
				return streamTailResult{end: llm.SourceEndCleanEOF}
			}
			// A source canceled mid-tail is not a clean end: its usage may be a
			// placeholder, so it must not be certified as final accounting.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return streamTailResult{end: llm.SourceEndCanceled, err: err}
			}
			return streamTailResult{end: llm.SourceEndError, err: err}
		}
		event := source.Current()
		if event != nil {
			eventBytes := len(event.Data) + len(event.Type) + len(event.LastEventID)
			if eventBytes >= streamTailPayloadLimit || bytes >= streamTailPayloadLimit-eventBytes {
				return streamTailResult{end: llm.SourceEndBudget, err: io.ErrShortBuffer}
			}
			bytes += eventBytes
		}
		events++
		if onEvent != nil {
			if err := onEvent(event); err != nil {
				return streamTailResult{end: llm.SourceEndError, err: err}
			}
		}
	}
	return streamTailResult{end: llm.SourceEndBudget}
}

// tailSourceEnd classifies how the source ended once its tail was collected.
// A clean EOF after a semantic terminal keeps the semantic end.
func tailSourceEnd(result streamTailResult, observed llm.SourceEndReason) llm.SourceEndReason {
	end := result.end
	if result.err != nil && !errors.Is(result.err, io.EOF) {
		end = llm.SourceEndError
		if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
			end = llm.SourceEndCanceled
		}
	}
	if result.err == nil && end == llm.SourceEndCleanEOF && observed == llm.SourceEndSemantic {
		return observed
	}
	return end
}

var (
	_ streams.DeliveryAcknowledger = (*streamLifecycle)(nil)
	_ streams.DeliveryBoundary     = (*streamLifecycle)(nil)
)
