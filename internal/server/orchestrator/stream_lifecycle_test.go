package orchestrator

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/looplj/axonhub/internal/server/lifecycle"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type lifecycleTailStream struct {
	events []*httpclient.StreamEvent
	index  int
}

func (s *lifecycleTailStream) Next() bool {
	if s.index == len(s.events) {
		return false
	}
	s.index++
	return true
}

func (s *lifecycleTailStream) Current() *httpclient.StreamEvent { return s.events[s.index-1] }
func (s *lifecycleTailStream) Err() error                       { return nil }
func (s *lifecycleTailStream) Close() error                     { return nil }

type sourceCloseProbeStream struct {
	streams.Stream[*httpclient.StreamEvent]

	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
	err     error
	once    sync.Once
}

func (s *sourceCloseProbeStream) Close() error {
	s.calls.Add(1)
	s.once.Do(func() { close(s.entered) })
	<-s.release
	return s.err
}

func TestStreamDeliverySourceCloseSerializesAbortAndFinalization(t *testing.T) {
	closeErr := errors.New("source close failed")
	source := &sourceCloseProbeStream{
		entered: make(chan struct{}), release: make(chan struct{}), err: closeErr,
	}
	attempt := &OutboundPersistentStream{stream: source}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(source.release) }) }
	defer release()

	aborted := make(chan struct{})
	context.AfterFunc(ctx, func() {
		attempt.abortSource()
		close(aborted)
	})
	cancel()
	<-source.entered

	const callers = 8
	results := make(chan error, callers)
	for range callers {
		context.AfterFunc(ctx, func() { results <- attempt.closeSource() })
	}
	select {
	case <-aborted:
		t.Fatal("abort returned before source cleanup completed")
	case <-results:
		t.Fatal("close returned before source cleanup completed")
	case <-time.After(20 * time.Millisecond):
	}

	release()
	<-aborted
	for range callers {
		if err := <-results; !errors.Is(err, closeErr) {
			t.Fatalf("close error = %v, want %v", err, closeErr)
		}
	}
	if got := source.calls.Load(); got != 1 {
		t.Fatalf("source Close calls = %d, want 1", got)
	}
}

func TestStreamDeliveryCancelRace(t *testing.T) {
	t.Run("cancel before acknowledgement cannot promote", func(t *testing.T) {
		downstream, cancel := context.WithCancel(context.Background())
		owner, execution := newStreamLifecycle(downstream)
		cancel()
		select {
		case <-execution.Done():
		case <-time.After(time.Second):
			t.Fatal("execution context was not canceled")
		}
		err := owner.Deliver(execution, func() (bool, error) {
			t.Fatal("terminal write ran after downstream cancellation")
			return true, nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Deliver error = %v, want context.Canceled", err)
		}
		owner.AcknowledgeDelivery()

		if owner.DeliveryAcknowledged() {
			t.Fatal("cancel-before-ack was promoted")
		}
		select {
		case <-execution.Done():
		case <-time.After(time.Second):
			t.Fatal("execution context was not canceled")
		}
		owner.Close()
	})

	t.Run("successful terminal write remains acknowledged after cancellation", func(t *testing.T) {
		downstream, cancel := context.WithCancel(context.Background())
		owner, _ := newStreamLifecycle(downstream)
		if !owner.stopDownstream() {
			t.Fatal("downstream watcher already fired")
		}
		cancel()
		owner.AcknowledgeDelivery()

		if owner.DeliveryAcknowledged() {
			t.Fatal("cancelled downstream was acknowledged without a delivery attempt")
		}
		owner.Close()
	})

	t.Run("acknowledgement before later cancellation remains latched", func(t *testing.T) {
		downstream, cancel := context.WithCancel(context.Background())
		owner, execution := newStreamLifecycle(downstream)
		err := owner.Deliver(execution, func() (bool, error) { return true, nil })
		if err != nil {
			t.Fatalf("Deliver error = %v", err)
		}
		cancel()

		if !owner.DeliveryAcknowledged() {
			t.Fatal("acknowledgement was demoted by later cancellation")
		}
		select {
		case <-execution.Done():
			t.Fatal("execution canceled before lifecycle close")
		default:
		}
		owner.Close()
		select {
		case <-execution.Done():
		case <-time.After(time.Second):
			t.Fatal("execution context was not released on close")
		}
	})
}

func TestStreamDeliveryTail(t *testing.T) {
	t.Run("retains order and stops at event budget", func(t *testing.T) {
		events := make([]*httpclient.StreamEvent, streamTailEventLimit+1)
		for i := range events {
			events[i] = &httpclient.StreamEvent{Data: []byte{byte(i)}}
		}
		source := &lifecycleTailStream{events: events}
		seen := 0
		result := collectStreamTail(context.Background(), source, func(event *httpclient.StreamEvent) error {
			if event == nil {
				return errors.New("nil tail event")
			}
			seen++
			return nil
		})

		if result.err != nil {
			t.Fatalf("collect tail: %v", result.err)
		}
		if seen != streamTailEventLimit {
			t.Fatalf("seen %d events, want %d", seen, streamTailEventLimit)
		}
		if source.index != streamTailEventLimit {
			t.Fatalf("read %d events, want %d", source.index, streamTailEventLimit)
		}
	})

	t.Run("event budget reports budget instead of semantic end", func(t *testing.T) {
		events := make([]*httpclient.StreamEvent, streamTailEventLimit+1)
		for i := range events {
			events[i] = &httpclient.StreamEvent{Data: []byte{byte(i)}}
		}
		result := collectStreamTail(context.Background(), &lifecycleTailStream{events: events}, nil)
		if result.end != llm.SourceEndBudget {
			t.Fatalf("tail end = %q, want budget", result.end)
		}
	})

	t.Run("stops before exceeding payload budget", func(t *testing.T) {
		event := &httpclient.StreamEvent{Data: make([]byte, streamTailPayloadLimit)}
		source := &lifecycleTailStream{events: []*httpclient.StreamEvent{event}}
		result := collectStreamTail(context.Background(), source, nil)

		if !errors.Is(result.err, io.ErrShortBuffer) {
			t.Fatalf("error = %v, want payload budget error", result.err)
		}
		if result.end != llm.SourceEndBudget {
			t.Fatalf("tail end = %q, want budget", result.end)
		}
	})
}

func TestStreamDeliveryLifecycleShutdownCancelsAcknowledgedExecution(t *testing.T) {
	manager := lifecycle.New()
	downstream := t.Context()
	ctx := manager.Attach(downstream)
	owner, execution := newStreamLifecycle(ctx)
	owner.AcknowledgeDelivery()
	manager.Shutdown(errors.New("test shutdown"))

	select {
	case <-execution.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel acknowledged execution")
	}
	owner.Close()
}

type closeProbeStream struct {
	streams.Stream[*httpclient.StreamEvent]

	closed atomic.Bool
}

func (s *closeProbeStream) Close() error {
	s.closed.Store(true)
	return nil
}

func TestPassThroughCloseKeepsRawSourceForTail(t *testing.T) {
	transformed := &closeProbeStream{Stream: streams.SliceStream[*httpclient.StreamEvent](nil)}
	raw := &closeProbeStream{Stream: streams.SliceStream[*httpclient.StreamEvent](nil)}
	stream := &passThroughStream{transformed: transformed, recorder: &rawEventRecorder{source: raw}}
	if err := stream.Close(); err != nil {
		t.Fatalf("close pass-through stream: %v", err)
	}
	if !transformed.closed.Load() {
		t.Fatal("client branch did not close the transformed stream")
	}
	if raw.closed.Load() {
		t.Fatal("client branch closed the raw source before tail accounting")
	}
}

// Both entry points must release a reader even when the source ignores context
// cancellation. A watchdog makes removing the abort callback fail, not hang.
func TestStreamDeliverySilentTail(t *testing.T) {
	for _, disconnect := range []bool{false, true} {
		name := "close"
		if disconnect {
			name = "disconnect_before_close"
		}
		t.Run(name, func(t *testing.T) {
			downstream, cancel := context.WithCancel(t.Context())
			defer cancel()
			owner, ctx := newStreamLifecycle(downstream)
			source := newBlockingStream()
			attempt := &OutboundPersistentStream{ctx: ctx, stream: source, lifecycle: owner}
			owner.trackAttempt(attempt)
			owner.AcknowledgeDelivery()
			defer func() { owner.trackAttempt(nil); owner.Close() }()

			var watchdogFired atomic.Bool
			watchdog := time.AfterFunc(3*streamTailTimeout, func() {
				watchdogFired.Store(true)
				attempt.abortSource()
			})
			defer watchdog.Stop()
			if disconnect {
				cancel()
				if attempt.Next() {
					t.Fatal("silent source yielded an event")
				}
				if ctx.Err() != nil {
					t.Fatal("disconnect canceled acknowledged execution")
				}
			}
			attempt.collectTail()
			if watchdogFired.Load() {
				t.Fatal("tail budget failed to release blocked Next")
			}
			if attempt.sourceEnd != llm.SourceEndCanceled {
				t.Fatalf("source end = %q, want canceled", attempt.sourceEnd)
			}
		})
	}
}
