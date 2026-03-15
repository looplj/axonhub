package task

import (
	"errors"
	"testing"
)

func TestStore_DeleteRejectsSystemTask(t *testing.T) {
	store := newTestStore(t, []Task{
		{
			ID:      "system-task",
			System:  true,
			Enabled: true,
			Trigger: Trigger{Type: TriggerTypeInterval, Interval: "1m"},
		},
	})

	err := store.Delete("system-task")
	if !errors.Is(err, ErrSystemTaskDelete) {
		t.Fatalf("expected ErrSystemTaskDelete, got %v", err)
	}

	tasks, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("unexpected task count: %d", len(tasks))
	}

	if !tasks[0].System {
		t.Fatalf("expected task to remain system task")
	}
}

func TestStore_MoveToDeletedRejectsSystemTask(t *testing.T) {
	store := newTestStore(t, []Task{
		{
			ID:      "system-task",
			System:  true,
			Enabled: true,
			Trigger: Trigger{Type: TriggerTypeAt},
		},
	})

	err := store.MoveToDeleted("system-task", "completed")
	if !errors.Is(err, ErrSystemTaskDelete) {
		t.Fatalf("expected ErrSystemTaskDelete, got %v", err)
	}

	tasks, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("unexpected task count: %d", len(tasks))
	}

	deleted, err := store.LoadDeleted()
	if err != nil {
		t.Fatalf("load deleted: %v", err)
	}

	if len(deleted) != 0 {
		t.Fatalf("expected no deleted tasks, got %d", len(deleted))
	}
}
