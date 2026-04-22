package orchestrator

import "sync"

// BackgroundWorker manages a background goroutine with safe Start/Stop semantics.
// It prevents double-start, supports stop-before-start (subsequent starts become
// no-ops), and ensures Stop blocks until the goroutine exits.
type BackgroundWorker struct {
	mu           sync.Mutex
	stopCh       chan struct{}
	stoppedEarly bool
	everStarted  bool
	wg           sync.WaitGroup
}

// Start launches the background goroutine. Only the first call launches;
// subsequent calls are no-ops. If Stop was called before Start, subsequent
// Start calls are also no-ops.
func (w *BackgroundWorker) Start(work func(stopCh <-chan struct{})) {
	w.mu.Lock()
	if w.stopCh != nil {
		w.mu.Unlock()
		return
	}

	if w.stoppedEarly && !w.everStarted {
		w.mu.Unlock()
		return
	}

	w.stoppedEarly = false
	w.everStarted = true
	w.stopCh = make(chan struct{})
	stopCh := w.stopCh
	w.wg.Add(1)
	w.mu.Unlock()
	go func() {
		defer w.wg.Done()
		work(stopCh)
	}()
}

// Stop signals the goroutine to shut down by closing the stop channel,
// then blocks until the goroutine has exited. Safe to call multiple times.
func (w *BackgroundWorker) Stop() {
	w.mu.Lock()
	if w.stopCh == nil {
		if !w.everStarted {
			w.stoppedEarly = true
		}
		w.mu.Unlock()
		return
	}

	ch := w.stopCh
	w.stopCh = nil
	w.mu.Unlock()
	close(ch)
	w.wg.Wait()
}
