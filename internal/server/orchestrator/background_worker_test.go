package orchestrator

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackgroundWorker_DoubleStart_Noop(t *testing.T) {
	var calls atomic.Int32
	w := &BackgroundWorker{}
	w.Start(func(stopCh <-chan struct{}) {
		calls.Add(1)
		<-stopCh
	})
	w.Start(func(stopCh <-chan struct{}) {
		calls.Add(1)
		<-stopCh
	})
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(1), calls.Load())
	w.Stop()
}

func TestBackgroundWorker_StopBeforeStart_PermanentNoop(t *testing.T) {
	w := &BackgroundWorker{}
	w.Stop() // stop before start

	var started atomic.Bool
	w.Start(func(stopCh <-chan struct{}) {
		started.Store(true)
		<-stopCh
	})
	time.Sleep(50 * time.Millisecond)
	require.False(t, started.Load(), "Start should be permanent no-op after Stop")
}

func TestBackgroundWorker_StartAfterStop_PermanentNoop(t *testing.T) {
	w := &BackgroundWorker{}
	w.Start(func(stopCh <-chan struct{}) {
		<-stopCh
	})
	w.Stop()

	var started atomic.Bool
	w.Start(func(stopCh <-chan struct{}) {
		started.Store(true)
		<-stopCh
	})
	time.Sleep(50 * time.Millisecond)
	require.False(t, started.Load(), "Start should be permanent no-op after Stop")
}

func TestBackgroundWorker_ConcurrentStartStop(t *testing.T) {
	w := &BackgroundWorker{}

	// Start a worker
	w.Start(func(stopCh <-chan struct{}) {
		<-stopCh
	})

	// Concurrently start and stop
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Stop()
	}()

	// Try to start again concurrently - should be no-op
	w.Start(func(stopCh <-chan struct{}) {
		t.Error("second Start should be no-op")
	})

	<-done
}

func TestBackgroundWorker_StopBlocksUntilGoroutineExits(t *testing.T) {
	var exited atomic.Bool
	w := &BackgroundWorker{}
	w.Start(func(stopCh <-chan struct{}) {
		<-stopCh
		exited.Store(true)
	})
	w.Stop()
	require.True(t, exited.Load(), "Stop should block until goroutine exits")
}
