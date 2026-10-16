package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/log"
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
// after acknowledgement it only opens the bounded collection window.
//
//nolint:containedctx // The lifecycle owns the request and execution contexts.
type streamLifecycle struct {
	mu sync.Mutex

	downstream     context.Context
	shutdown       context.Context
	execution      context.Context
	cancel         context.CancelCauseFunc
	deadlineCancel context.CancelFunc

	acknowledged bool
	canceled     bool
	closed       bool
	done         chan struct{}
	finalized    chan struct{}
	stopWatch    chan struct{}
	stopOnce     sync.Once
	tail         *streamTailOwner
}

type streamTailOwner struct {
	source         streams.Stream[*httpclient.StreamEvent]
	readerDone     <-chan struct{}
	readerDoneFunc func() <-chan struct{}
	abort          func()
	onEvent        func(*httpclient.StreamEvent) error
	sourceEnd      func() llm.SourceEndReason
	finalize       func(llm.SourceEndReason) error
	done           chan struct{}
	accountingDone chan struct{}
	startOnce      sync.Once
}

type streamTailResult struct {
	end llm.SourceEndReason
	err error
}

// awaitTailBarrier waits for one quiescence barrier, aborting the source if the
// tail budget expires first so a blocked reader cannot stall finalization.
func awaitTailBarrier(ctx context.Context, abort func(), barrier <-chan struct{}) {
	if barrier == nil {
		return
	}
	select {
	case <-barrier:
	case <-ctx.Done():
		if abort != nil {
			abort()
		}
		<-barrier
	}
}

func (o *streamTailOwner) run(ctx context.Context, cause error) {
	defer close(o.done)
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Warn(ctx, "stream tail owner panicked", log.Any("panic", recovered))
		}
	}()
	if cause != nil {
		if o.abort != nil {
			o.abort()
		}
	}
	// The tail must not touch the raw source until the consumer has both
	// finished its in-flight read and stopped iterating. Awaiting only one of
	// the two barriers lets the tail call Next on the raw source while the
	// consumer is between Next and Current, which can skip or duplicate the
	// final usage event.
	awaitTailBarrier(ctx, o.abort, o.readerDone)
	if o.readerDoneFunc != nil {
		awaitTailBarrier(ctx, o.abort, o.readerDoneFunc())
	}
	result := streamTailResult{end: llm.SourceEndCanceled, err: ctx.Err()}
	if ctx.Err() == nil {
		resultCh := make(chan streamTailResult, 1)
		go func() {
			result := streamTailResult{end: llm.SourceEndError}
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Warn(ctx, "stream tail collection panicked", log.Any("panic", recovered))
					result.err = fmt.Errorf("stream tail collection panic: %v", recovered)
				}
				resultCh <- result
			}()
			result = collectStreamTail(ctx, o.source, o.onEvent)
		}()
		select {
		case result = <-resultCh:
		case <-ctx.Done():
			if o.abort != nil {
				o.abort()
			}
			result = <-resultCh
			result.end = llm.SourceEndCanceled
			result.err = ctx.Err()
		}
	}
	if o.finalize != nil {
		end := result.end
		if result.err != nil && !errors.Is(result.err, io.EOF) {
			end = llm.SourceEndError
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				end = llm.SourceEndCanceled
			}
		}
		if result.err == nil && end == llm.SourceEndCleanEOF && o.sourceEnd != nil {
			if sourceEnd := o.sourceEnd(); sourceEnd == llm.SourceEndSemantic {
				end = sourceEnd
			}
		}
		_ = o.finalize(end)
	}
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
		done:           make(chan struct{}),
		finalized:      make(chan struct{}),
		stopWatch:      make(chan struct{}),
	}
	go owner.watch()

	return owner, withStreamLifecycle(execution, owner)
}

func (s *streamLifecycle) watch() {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Warn(s.execution, "stream lifecycle watcher panicked", log.Any("panic", recovered))
		}
	}()
	select {
	case <-s.downstream.Done():
		s.downstreamEnded(s.downstream.Err())
	case <-s.shutdownDone():
		s.downstreamEnded(context.Cause(s.shutdown))
	case <-s.stopWatch:
	}
	close(s.done)
}

func (s *streamLifecycle) shutdownDone() <-chan struct{} {
	if s.shutdown == nil {
		return nil
	}
	return s.shutdown.Done()
}

func (s *streamLifecycle) downstreamEnded(cause error) {
	s.mu.Lock()
	if s.closed || s.canceled {
		s.mu.Unlock()
		return
	}
	shutdown := s.shutdown != nil && context.Cause(s.shutdown) != nil && errors.Is(cause, context.Cause(s.shutdown))
	if s.acknowledged && cause != nil && !shutdown && !errors.Is(cause, context.DeadlineExceeded) {
		tail := s.tail
		s.mu.Unlock()
		if tail != nil {
			s.startTail(tail, nil)
		}
		return
	}
	s.canceled = true
	s.cancel(normalizeCancelCause(cause))
	tail := s.tail
	s.mu.Unlock()
	if tail != nil {
		s.startTail(tail, cause)
	}
}

func (s *streamLifecycle) registerTailOwner(owner *streamTailOwner) {
	if owner == nil {
		return
	}
	s.mu.Lock()
	s.tail = owner
	acknowledged := s.acknowledged
	s.mu.Unlock()
	if acknowledged {
		s.startTail(owner, nil)
	}
}

func (s *streamLifecycle) startTail(owner *streamTailOwner, cause error) {
	owner.startOnce.Do(func() {
		if owner.accountingDone == nil {
			owner.accountingDone = make(chan struct{})
		}
		tailCtx, cancel := context.WithTimeout(s.execution, streamTailTimeout)
		go func() {
			defer cancel()
			defer close(owner.accountingDone)
			owner.run(tailCtx, cause)
		}()
	})
}

func (s *streamLifecycle) waitForAccounting() {
	s.mu.Lock()
	tail := s.tail
	s.mu.Unlock()
	if tail == nil {
		return
	}

	s.startTail(tail, context.Canceled)
	<-tail.accountingDone
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

func (s *streamLifecycle) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	acknowledged := s.acknowledged
	tail := s.tail
	if !s.canceled && (!acknowledged || tail == nil) {
		s.canceled = true
		s.cancel(context.Canceled)
	}
	s.mu.Unlock()
	if tail != nil && acknowledged {
		s.startTail(tail, nil)
		<-tail.done
	} else if tail != nil {
		if tail.abort != nil {
			tail.abort()
		}
		awaitTailBarrier(context.Background(), nil, tail.readerDone)
		if tail.readerDoneFunc != nil {
			awaitTailBarrier(context.Background(), nil, tail.readerDoneFunc())
		}
		s.startTail(tail, context.Canceled)
		<-tail.done
	}
	s.stopOnce.Do(func() { close(s.stopWatch) })
	<-s.done
	if s.deadlineCancel != nil {
		s.deadlineCancel()
	}
	close(s.finalized)
}

func (s *streamLifecycle) WaitForFinalization() {
	<-s.finalized
}

// collectTail drains one retained source. It is intentionally synchronous:
// callers invoke it only after the source reader has acknowledged ownership.
func collectStreamTail(ctx context.Context, source streams.Stream[*httpclient.StreamEvent], onEvent func(*httpclient.StreamEvent) error) streamTailResult {
	if source == nil {
		return streamTailResult{end: llm.SourceEndCleanEOF}
	}
	tailCtx, cancel := context.WithTimeout(ctx, streamTailTimeout)
	defer cancel()

	events := 0
	bytes := 0
	for events < streamTailEventLimit && bytes < streamTailPayloadLimit {
		select {
		case <-tailCtx.Done():
			return streamTailResult{end: llm.SourceEndCanceled, err: tailCtx.Err()}
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

var (
	_ streams.DeliveryAcknowledger = (*streamLifecycle)(nil)
	_ streams.DeliveryBoundary     = (*streamLifecycle)(nil)
)
