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
)

type lifecycleTailStream struct {
	events []*httpclient.StreamEvent
	index  int
}

type blockingLifecycleTailStream struct {
	closed      chan struct{}
	readerDone  chan struct{}
	enteredNext chan struct{}
	enteredOnce sync.Once
	activeNext  atomic.Int32
	concurrent  atomic.Bool
	closeOnce   atomic.Bool
	current     *httpclient.StreamEvent
}

type orderedLifecycleTailStream struct {
	readerDone <-chan struct{}
	sawEarly   atomic.Bool
}

type budgetAbortTailStream struct {
	closeCalled chan struct{}
	nextCalls   atomic.Int32
	closeOnce   sync.Once
}

type blockedTailAfterReaderStream struct {
	closed    chan struct{}
	entered   chan struct{}
	nextDone  chan struct{}
	nextCalls atomic.Int32
	closeOnce sync.Once
}

func (s *blockedTailAfterReaderStream) Next() bool {
	s.nextCalls.Add(1)
	close(s.entered)
	<-s.closed
	close(s.nextDone)
	return false
}

func (s *blockedTailAfterReaderStream) Current() *httpclient.StreamEvent { return nil }
func (s *blockedTailAfterReaderStream) Err() error                       { return context.Canceled }
func (s *blockedTailAfterReaderStream) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

func (s *budgetAbortTailStream) Next() bool {
	s.nextCalls.Add(1)
	return true
}

func (s *budgetAbortTailStream) Current() *httpclient.StreamEvent {
	return &httpclient.StreamEvent{Data: []byte{1}}
}

func (s *budgetAbortTailStream) Err() error { return nil }
func (s *budgetAbortTailStream) Close() error {
	s.closeOnce.Do(func() { close(s.closeCalled) })
	return nil
}

type delayedLifecycleTailStream struct {
	release     chan struct{}
	closeCalled chan struct{}
	readerDone  chan struct{}
	closeOnce   sync.Once
	readOnce    sync.Once
}

func (s *delayedLifecycleTailStream) Next() bool {
	<-s.release
	s.readOnce.Do(func() { close(s.readerDone) })
	return false
}

func (s *delayedLifecycleTailStream) Current() *httpclient.StreamEvent { return nil }
func (s *delayedLifecycleTailStream) Err() error                       { return context.Canceled }
func (s *delayedLifecycleTailStream) Close() error {
	s.closeOnce.Do(func() { close(s.closeCalled) })
	return nil
}

func (s *orderedLifecycleTailStream) Next() bool {
	select {
	case <-s.readerDone:
	default:
		s.sawEarly.Store(true)
	}
	return false
}

func (s *orderedLifecycleTailStream) Current() *httpclient.StreamEvent { return nil }
func (s *orderedLifecycleTailStream) Err() error                       { return nil }
func (s *orderedLifecycleTailStream) Close() error                     { return nil }

func newBlockingLifecycleTailStream() *blockingLifecycleTailStream {
	return &blockingLifecycleTailStream{closed: make(chan struct{}), readerDone: make(chan struct{}), enteredNext: make(chan struct{})}
}

func (s *blockingLifecycleTailStream) Next() bool {
	s.enteredOnce.Do(func() { close(s.enteredNext) })
	if s.activeNext.Add(1) != 1 {
		s.concurrent.Store(true)
	}
	defer s.activeNext.Add(-1)
	<-s.closed
	close(s.readerDone)
	return false
}

func (s *blockingLifecycleTailStream) Current() *httpclient.StreamEvent { return s.current }
func (s *blockingLifecycleTailStream) Err() error                       { return context.Canceled }
func (s *blockingLifecycleTailStream) Close() error {
	if s.closeOnce.CompareAndSwap(false, true) {
		close(s.closed)
	}
	return nil
}

func (s *blockingLifecycleTailStream) concurrentNext() bool { return s.concurrent.Load() }

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
		owner.stopOnce.Do(func() { close(owner.stopWatch) })
		select {
		case <-owner.done:
		case <-time.After(time.Second):
			t.Fatal("lifecycle watcher did not stop")
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

func TestStreamDeliveryTailOwnerBudgetAbortsSourceBeforeFinalizing(t *testing.T) {
	source := &budgetAbortTailStream{closeCalled: make(chan struct{})}
	finalized := make(chan llm.SourceEndReason, 1)
	owner := &streamTailOwner{
		source: source,
		readerDone: func() <-chan struct{} {
			return closedChannel()
		}(),
		abort: func() { _ = source.Close() },
		finalize: func(end llm.SourceEndReason) error {
			finalized <- end
			return nil
		},
		done: make(chan struct{}),
	}
	owner.run(context.Background(), context.Canceled)

	if source.nextCalls.Load() != streamTailEventLimit {
		t.Fatalf("Next calls = %d, want %d", source.nextCalls.Load(), streamTailEventLimit)
	}
	select {
	case <-source.closeCalled:
	default:
		t.Fatal("budget did not abort source")
	}
	if got := <-finalized; got != llm.SourceEndBudget {
		t.Fatalf("finalized source end = %q, want budget", got)
	}
}

func TestStreamDeliveryTailOwnerSourceEndReasons(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		sourceEnd func() llm.SourceEndReason
		want      llm.SourceEndReason
	}{
		{name: "clean eof", want: llm.SourceEndCleanEOF},
		{name: "semantic end", sourceEnd: func() llm.SourceEndReason { return llm.SourceEndSemantic }, want: llm.SourceEndSemantic},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			finalized := make(chan llm.SourceEndReason, 1)
			owner := &streamTailOwner{
				source:     &lifecycleTailStream{},
				readerDone: closedChannel(),
				sourceEnd:  scenario.sourceEnd,
				finalize: func(end llm.SourceEndReason) error {
					finalized <- end
					return nil
				},
				done: make(chan struct{}),
			}
			owner.run(context.Background(), nil)
			if got := <-finalized; got != scenario.want {
				t.Fatalf("finalized source end = %q, want %q", got, scenario.want)
			}
		})
	}
}

func TestStreamDeliveryTailOwnerAbortsBlockedNextBeforeFinalizing(t *testing.T) {
	for _, name := range []string{"deadline", "shutdown"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			source := &blockedTailAfterReaderStream{
				closed: make(chan struct{}), entered: make(chan struct{}), nextDone: make(chan struct{}),
			}
			finalized := make(chan llm.SourceEndReason, 1)
			owner := &streamTailOwner{
				source:     source,
				readerDone: closedChannel(),
				abort:      func() { _ = source.Close() },
				finalize: func(end llm.SourceEndReason) error {
					finalized <- end
					return nil
				},
				done: make(chan struct{}),
			}
			go owner.run(ctx, nil)
			<-source.entered
			cancel()
			select {
			case <-source.closed:
			case <-time.After(time.Second):
				t.Fatal("blocked Next was not aborted")
			}
			select {
			case <-finalized:
			case <-time.After(time.Second):
				t.Fatal("tail owner did not finalize")
			}
			select {
			case <-source.nextDone:
			default:
				t.Fatal("finalized before blocked Next returned")
			}
			if got := source.nextCalls.Load(); got != 1 {
				t.Fatalf("Next calls = %d, want 1", got)
			}
		})
	}
}

func closedChannel() <-chan struct{} {
	channel := make(chan struct{})
	close(channel)
	return channel
}

func TestStreamDeliveryLifecycleTailOwner(t *testing.T) {
	downstream, cancel := context.WithCancel(context.Background())
	owner, execution := newStreamLifecycle(downstream)
	source := newBlockingLifecycleTailStream()
	finalized := make(chan llm.SourceEndReason, 1)
	owner.registerTailOwner(&streamTailOwner{
		source:     source,
		readerDone: closedChannel(),
		abort:      func() { _ = source.Close() },
		finalize: func(end llm.SourceEndReason) error {
			finalized <- end
			return nil
		},
		done: make(chan struct{}),
	})
	owner.AcknowledgeDelivery()
	cancel()
	owner.Close()

	select {
	case <-finalized:
	case <-time.After(time.Second):
		t.Fatal("tail owner did not finalize")
	}
	select {
	case <-execution.Done():
		t.Fatal("client cancellation canceled acknowledged execution")
	default:
	}
	if source.concurrentNext() {
		t.Fatal("tail owner called Next concurrently")
	}
}

func TestStreamDeliveryRetryWaitsForPendingAccounting(t *testing.T) {
	downstream := t.Context()
	lifecycleOwner, _ := newStreamLifecycle(downstream)
	releaseAccounting := make(chan struct{})
	accountingStarted := make(chan struct{})
	tail := &streamTailOwner{
		source:     &lifecycleTailStream{},
		readerDone: closedChannel(),
		finalize: func(llm.SourceEndReason) error {
			close(accountingStarted)
			<-releaseAccounting
			return nil
		},
		done: make(chan struct{}),
	}
	lifecycleOwner.registerTailOwner(tail)
	lifecycleOwner.startTail(tail, nil)
	<-accountingStarted

	retryReady := make(chan struct{})
	go func() {
		lifecycleOwner.waitForAccounting()
		close(retryReady)
	}()

	select {
	case <-retryReady:
		t.Fatal("retry proceeded before pending accounting completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseAccounting)
	select {
	case <-retryReady:
	case <-time.After(time.Second):
		t.Fatal("retry did not wait for accounting completion")
	}

	lifecycleOwner.Close()
}

func TestStreamDeliveryUnacknowledgedCloseWaitsForReaderBeforeFinalizing(t *testing.T) {
	// Given: an unacknowledged lifecycle owns a source whose reader is still active.
	downstream := t.Context()
	owner, _ := newStreamLifecycle(downstream)
	source := &delayedLifecycleTailStream{
		release:     make(chan struct{}),
		closeCalled: make(chan struct{}),
		readerDone:  make(chan struct{}),
	}
	finalized := make(chan struct{})
	closeDone := make(chan struct{})
	go source.Next()
	owner.registerTailOwner(&streamTailOwner{
		source:     source,
		readerDone: source.readerDone,
		abort:      func() { _ = source.Close() },
		finalize:   func(llm.SourceEndReason) error { close(finalized); return nil },
		done:       make(chan struct{}),
	})

	// When
	go func() {
		owner.Close()
		close(closeDone)
	}()

	// Then: finalization is ordered after the source reader has quiesced.
	select {
	case <-source.closeCalled:
	case <-finalized:
		t.Fatal("lifecycle finalized before the reader quiesced")
	}
	select {
	case <-finalized:
		t.Fatal("lifecycle finalized before the reader quiesced")
	default:
	}
	close(source.release)
	<-closeDone
	select {
	case <-finalized:
	default:
		t.Fatal("lifecycle did not finalize after the reader quiesced")
	}
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

func TestPassThroughDetachKeepsRawOwner(t *testing.T) {
	var detached atomic.Bool
	var canceled atomic.Bool
	stream := &passThroughChannelStream{
		ch:     make(chan *httpclient.StreamEvent),
		detach: func() { detached.Store(true) },
		cancel: func() { canceled.Store(true) },
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close pass-through stream: %v", err)
	}
	if !detached.Load() {
		t.Fatal("client branch did not detach")
	}
	if canceled.Load() {
		t.Fatal("client branch canceled the raw owner")
	}
}

func TestStreamDeliveryTailOwnerWaitsForReader(t *testing.T) {
	readerDone := make(chan struct{})
	source := &orderedLifecycleTailStream{readerDone: readerDone}
	owner := &streamTailOwner{
		source:     source,
		readerDone: readerDone,
		abort:      func() { close(readerDone) },
		finalize:   func(llm.SourceEndReason) error { return nil },
		done:       make(chan struct{}),
	}
	owner.run(context.Background(), context.Canceled)
	if source.sawEarly.Load() {
		t.Fatal("tail reader started before consumer quiescence")
	}
}

// handoffProbeTailStream records whether the tail entered the raw source, so a
// handoff test can prove the consumer barrier — not just the in-flight reader —
// gates the tail's first read.
type handoffProbeTailStream struct {
	entered     chan struct{}
	enteredOnce sync.Once
	release     chan struct{}
	activeNext  atomic.Int32
	concurrent  atomic.Bool
}

func (s *handoffProbeTailStream) Next() bool {
	if s.activeNext.Add(1) != 1 {
		s.concurrent.Store(true)
	}
	defer s.activeNext.Add(-1)
	s.enteredOnce.Do(func() { close(s.entered) })
	<-s.release
	return false
}

func (s *handoffProbeTailStream) Current() *httpclient.StreamEvent { return nil }
func (s *handoffProbeTailStream) Err() error                       { return context.Canceled }
func (s *handoffProbeTailStream) Close() error                     { return nil }
func (s *handoffProbeTailStream) concurrentNext() bool             { return s.concurrent.Load() }

// TestStreamDeliveryTailOwnerWaitsForConsumerQuiescence locks the handoff
// contract: the tail owner must await both the consumer barrier and the
// in-flight reader. Awaiting only the in-flight reader lets the tail call Next
// on the raw source while the consumer sits between Next and Current, which can
// skip or duplicate the trailing usage event that accounting depends on.
func TestStreamDeliveryTailOwnerWaitsForConsumerQuiescence(t *testing.T) {
	consumerDone := make(chan struct{})
	activeReader := make(chan struct{})
	close(activeReader) // the in-flight read has already returned
	source := &handoffProbeTailStream{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	finalized := make(chan llm.SourceEndReason, 1)
	owner := &streamTailOwner{
		source:         source,
		readerDone:     consumerDone,
		readerDoneFunc: func() <-chan struct{} { return activeReader },
		abort:          func() { _ = source.Close() },
		finalize: func(end llm.SourceEndReason) error {
			finalized <- end
			return nil
		},
		done: make(chan struct{}),
	}

	tailCtx, cancelTail := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelTail()
	go owner.run(tailCtx, nil)

	// Then: the consumer still owns the source, so the tail must not read it.
	select {
	case <-source.entered:
		t.Fatalf("tail read the raw source before the consumer quiesced")
	case <-time.After(100 * time.Millisecond):
	}
	if source.concurrentNext() {
		t.Fatal("tail read the raw source concurrently with the consumer")
	}

	// When: the consumer finishes iterating.
	close(consumerDone)
	select {
	case <-source.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("tail did not read the source after the consumer quiesced")
	}
	close(source.release)
	select {
	case <-finalized:
	case <-time.After(2 * time.Second):
		t.Fatal("tail did not finalize")
	}
	if source.concurrentNext() {
		t.Fatal("tail read the raw source concurrently with the consumer")
	}
}
