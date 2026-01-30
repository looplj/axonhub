package watcher

import (
	"context"
	"sync"
)

type MemoryWatcherOptions struct {
	Buffer int
}

type memoryWatcher[T any] struct {
	mu     sync.Mutex
	nextID uint64
	subs   map[uint64]chan T
	buffer int
}

func NewMemoryWatcher[T any](opts MemoryWatcherOptions) Notifier[T] {
	buffer := opts.Buffer
	if buffer <= 0 {
		buffer = 1
	}

	return &memoryWatcher[T]{
		subs:   make(map[uint64]chan T),
		buffer: buffer,
	}
}

func (w *memoryWatcher[T]) Watch() (<-chan T, func()) {
	w.mu.Lock()
	defer w.mu.Unlock()

	id := w.nextID
	w.nextID++

	ch := make(chan T, w.buffer)
	w.subs[id] = ch

	return ch, func() {
		w.mu.Lock()
		defer w.mu.Unlock()

		sub, ok := w.subs[id]
		if !ok {
			return
		}

		delete(w.subs, id)
		close(sub)
	}
}

func (w *memoryWatcher[T]) Notify(_ context.Context, v T) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, ch := range w.subs {
		select {
		case ch <- v:
		default:
		}
	}

	return nil
}
