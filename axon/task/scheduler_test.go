package task

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

type testHandler struct {
	done chan struct{}
	err  error
}

func (h *testHandler) HandleTask(ctx context.Context, t Task) error {
	if h.done != nil {
		select {
		case h.done <- struct{}{}:
		default:
		}
	}
	return h.err
}

func newTestStore(t *testing.T, tasks []Task) *Store {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Save(tasks); err != nil {
		t.Fatalf("save store: %v", err)
	}
	return store
}

func TestScheduler_DisablesPastAtTask(t *testing.T) {
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	store := newTestStore(t, []Task{
		{
			ID:      "once",
			Enabled: true,
			Trigger: Trigger{
				Type: TriggerTypeAt,
				At:   now.Add(-time.Minute).Format(time.RFC3339),
			},
		},
	})
	s, err := NewScheduler(slog.Default(), store, &testHandler{}, SchedulerOptions{})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	s.nowFunc = func() time.Time { return now }

	if err := s.tick(context.Background()); err != nil {
		t.Fatalf("tick: %v", err)
	}

	tasks, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("unexpected task count: %d", len(tasks))
	}
	if tasks[0].Enabled {
		t.Fatalf("expected task to be disabled")
	}
	if tasks[0].Runtime.LastStatus != "failed" {
		t.Fatalf("expected failed status, got %q", tasks[0].Runtime.LastStatus)
	}
}

func TestScheduler_UpdatesStatusAfterRun(t *testing.T) {
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	handler := &testHandler{done: make(chan struct{}, 1)}
	store := newTestStore(t, []Task{
		{
			ID:      "i1",
			Enabled: true,
			Trigger: Trigger{Type: TriggerTypeInterval, Interval: "1m"},
			Runtime: TaskRuntime{NextRunAt: now.Format(time.RFC3339)},
		},
	})
	s, err := NewScheduler(slog.Default(), store, handler, SchedulerOptions{})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	s.nowFunc = func() time.Time { return now }

	if err := s.tick(context.Background()); err != nil {
		t.Fatalf("tick: %v", err)
	}

	select {
	case <-handler.done:
	case <-time.After(2 * time.Second):
		t.Fatal("task was not executed")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		tasks, err := store.Load()
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("unexpected task count: %d", len(tasks))
		}
		if tasks[0].Runtime.LastStatus == "success" && !tasks[0].Runtime.Running {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("status not updated, got status=%q running=%v", tasks[0].Runtime.LastStatus, tasks[0].Runtime.Running)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
