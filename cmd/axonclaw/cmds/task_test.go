package cmds

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/looplj/axonhub/axon/task"
)

func TestTaskEnableResetsNextRunAt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".axonclaw")
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

	cmd := NewTaskCommand(TaskOptions{Dir: dir})
	cmd.SetArgs([]string{"enable", "t1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
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
