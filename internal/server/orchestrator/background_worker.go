package orchestrator

import "sync"

// BackgroundWorker manages a background goroutine with safe Start/Stop semantics.
// It prevents double-start, supports stop-before-start (permanent no-op after Stop),
// and ensures Stop blocks until the goroutine exits without deadlock.
type BackgroundWorker struct {
	mu      sync.Mutex
	stopCh  chan struct{}
	stopped bool // permanently disabled after Stop called
	running bool // true when a goroutine is active
	wg      sync.WaitGroup
}

// Start launches the background goroutine. Only the first call launches a goroutine;
// subsequent calls are no-ops. After Stop is called, all future Start calls are
// permanent no-ops.
func (w *BackgroundWorker) Start(work func(stopCh <-chan struct{})) {
	w.mu.Lock()
	if w.running || w.stopped {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	stopCh := w.stopCh
	w.wg.Add(1)
	w.mu.Unlock()
	go func() {
		defer w.wg.Done()
		work(stopCh)
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()
}

// Stop signals the goroutine to shut down and blocks until it exits.
// After Stop is called, all future Start calls are permanent no-ops.
func (w *BackgroundWorker) Stop() {
	w.mu.Lock()
	w.stopped = true
	if w.stopCh == nil {
		w.mu.Unlock()
		return
	}
	ch := w.stopCh
	w.stopCh = nil
	w.mu.Unlock()
	close(ch)
	w.wg.Wait()
}
