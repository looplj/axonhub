package orchestrator

import "sync"

// BackgroundWorker manages the lifecycle of a background goroutine with
// safe Start/Stop semantics. It prevents double-start, supports
// stop-before-start (making subsequent starts no-ops), and ensures the
// stop channel is closed exactly once.
type BackgroundWorker struct {
	mu           sync.Mutex
	stopCh       chan struct{}
	stoppedEarly bool
	everStarted  bool
}

// Start begins the background goroutine. The work function receives the stop
// channel; it should select on it to detect shutdown.
// Safe to call multiple times — only the first call launches the goroutine.
// If Stop was called before Start, subsequent Start calls are no-ops.
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
	w.mu.Unlock()

	go work(stopCh)
}

// Stop signals the background goroutine to shut down by closing the stop channel.
// Safe to call multiple times. If called before Start has ever been called,
// subsequent Start calls become no-ops.
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
}
