package cmds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/looplj/axonhub/axon/task"
)

func TestTaskEnableResetsNextRunAt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tasks")
	store, err := task.NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	err = store.Save([]task.Task{
		{
			ID:      "t1",
			Enabled: false,
			Trigger: task.Trigger{Type: task.TriggerTypeInterval, Interval: "1m"},
			Runtime: task.TaskRuntime{NextRunAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)},
		},
	})
	if err != nil {
		t.Fatalf("save task: %v", err)
	}

	if err := store.SetEnabled("t1", true); err != nil {
		t.Fatalf("enable: %v", err)
	}

	tasks, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("unexpected task count: %d", len(tasks))
	}
	if !tasks[0].Enabled {
		t.Fatalf("expected task enabled")
	}
	if tasks[0].Runtime.NextRunAt != "" {
		t.Fatalf("expected next_run_at reset, got %q", tasks[0].Runtime.NextRunAt)
	}
}

func TestTaskListHidesHiddenTasks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tasks")

	store, err := task.NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	err = store.Save([]task.Task{
		{ID: "visible-task", Name: "Visible", Enabled: true, Trigger: task.Trigger{Type: task.TriggerTypeInterval, Interval: "1m"}},
		{ID: "hidden-task", Name: "Hidden", Enabled: true, Hidden: true, Trigger: task.Trigger{Type: task.TriggerTypeInterval, Interval: "1m"}},
	})
	if err != nil {
		t.Fatalf("save task: %v", err)
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "task-list-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer tmpFile.Close()

	cmd := newTaskListCmd(tmpFile, func() *task.Store { return store })

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error = %v", err)
	}

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(data)
	if !strings.Contains(got, "visible-task") {
		t.Fatalf("expected visible task in output: %q", got)
	}

	if strings.Contains(got, "hidden-task") {
		t.Fatalf("hidden task should not appear in output: %q", got)
	}
}

func TestTaskStoreListHidesHiddenTasks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tasks")

	store, err := task.NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	err = store.Save([]task.Task{
		{ID: "visible-task", Name: "Visible", Enabled: true},
		{ID: "hidden-task", Name: "Hidden", Enabled: true, Hidden: true},
	})
	if err != nil {
		t.Fatalf("save task: %v", err)
	}

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}

	if tasks[0].ID != "visible-task" {
		t.Fatalf("tasks[0].ID = %q, want visible-task", tasks[0].ID)
	}
}
