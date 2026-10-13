package orchestrator

import (
	"context"
	"sync"

	"github.com/looplj/axonhub/llm/httpclient"
)

type rawStreamQueue struct {
	backlog   *rawStreamBacklog
	ch        chan *httpclient.StreamEvent
	heldMu    sync.Mutex
	held      []*httpclient.StreamEvent
	heldSizes []int
}

func newRawStreamQueue(backlog *rawStreamBacklog) *rawStreamQueue {
	return &rawStreamQueue{
		backlog: backlog,
		ch:      make(chan *httpclient.StreamEvent, 64),
	}
}

func (q *rawStreamQueue) attach() []*httpclient.StreamEvent {
	q.backlog.mu.Lock()
	q.backlog.attached = true
	q.heldMu.Lock()
	q.held = q.backlog.events
	q.heldSizes = make([]int, len(q.held))
	for i, event := range q.held {
		q.heldSizes[i] = rawStreamEventSize(event)
	}
	q.backlog.events = nil
	held := q.held
	q.heldMu.Unlock()
	q.backlog.mu.Unlock()

	return held
}

func (q *rawStreamQueue) enqueue(ctx context.Context, event *httpclient.StreamEvent) error {
	size := rawStreamEventSize(event)
	if err := q.backlog.reserve(ctx, size); err != nil {
		return err
	}

	select {
	case q.ch <- event:
		return nil
	case <-ctx.Done():
		q.backlog.release(size)
		return ctx.Err()
	}
}

func (q *rawStreamQueue) next(ctx context.Context) (*httpclient.StreamEvent, bool) {
	q.heldMu.Lock()
	if len(q.held) > 0 {
		event := q.held[0]
		q.held = q.held[1:]
		size := q.heldSizes[0]
		q.heldSizes = q.heldSizes[1:]
		q.heldMu.Unlock()
		q.backlog.release(size)
		return event, true
	}
	q.heldMu.Unlock()

	select {
	case event, ok := <-q.ch:
		if !ok {
			return nil, false
		}
		q.backlog.release(rawStreamEventSize(event))
		return event, true
	case <-ctx.Done():
		return nil, false
	}
}

func (q *rawStreamQueue) nextBuffered() (*httpclient.StreamEvent, bool) {
	q.heldMu.Lock()
	if len(q.held) > 0 {
		event := q.held[0]
		q.held = q.held[1:]
		size := q.heldSizes[0]
		q.heldSizes = q.heldSizes[1:]
		q.heldMu.Unlock()
		q.backlog.release(size)
		return event, true
	}
	q.heldMu.Unlock()

	select {
	case event, ok := <-q.ch:
		if !ok {
			return nil, false
		}
		q.backlog.release(rawStreamEventSize(event))
		return event, true
	default:
		return nil, false
	}
}

func rawStreamEventSize(event *httpclient.StreamEvent) int {
	if event == nil {
		return 0
	}

	return len(event.Data) + len(event.Type) + len(event.LastEventID)
}
