package cmds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/looplj/axonhub/axon/task"

	"github.com/looplj/axonhub/cmd/axonclaw/claw"
	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

func setupHeartbeatTaskStore(t *testing.T) *task.Store {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	runtimeDir, err := conf.RuntimeDir()
	if err != nil {
		t.Fatalf("RuntimeDir() error = %v", err)
	}

	store, err := task.NewStore(filepath.Join(runtimeDir, "tasks"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	return store
}

func TestHeartbeatEnableDisablesTask(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	store := setupHeartbeatTaskStore(t)

	if err := store.Save([]task.Task{{
		ID:      claw.SystemTaskHeartbeatID,
		Name:    claw.SystemTaskNameHeartbeat,
		Type:    string(claw.TaskTypeHeartbeat),
		Enabled: true,
		Hidden:  true,
		Trigger: task.Trigger{Type: task.TriggerTypeInterval, Interval: "30m"},
		Action: map[string]any{
			"active_start":  "08:00",
			"active_end":    "23:00",
			"light_context": false,
			"ack_max_chars": 300,
		},
	}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cmd := newHeartbeatEnableCmd(os.Stdout, false)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	tk, err := store.Get(claw.SystemTaskHeartbeatID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if tk.Enabled {
		t.Fatal("heartbeat task should be disabled")
	}
}

func TestHeartbeatStatusReadsTaskState(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	store := setupHeartbeatTaskStore(t)

	if err := store.Save([]task.Task{{
		ID:      claw.SystemTaskHeartbeatID,
		Name:    claw.SystemTaskNameHeartbeat,
		Type:    string(claw.TaskTypeHeartbeat),
		Enabled: false,
		Hidden:  true,
		Trigger: task.Trigger{Type: task.TriggerTypeInterval, Interval: "30m"},
		Action: map[string]any{
			"active_start":  "08:00",
			"active_end":    "23:00",
			"light_context": false,
			"ack_max_chars": 300,
		},
	}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "heartbeat-status-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer tmpFile.Close()

	cmd := newHeartbeatStatusCmd(tmpFile)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if !strings.Contains(string(data), "Heartbeat: disabled") {
		t.Fatalf("unexpected output: %q", string(data))
	}
}

func TestHeartbeatIntervalUpdatesTaskTrigger(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	store := setupHeartbeatTaskStore(t)

	if err := store.Save([]task.Task{{
		ID:      claw.SystemTaskHeartbeatID,
		Name:    claw.SystemTaskNameHeartbeat,
		Type:    string(claw.TaskTypeHeartbeat),
		Enabled: true,
		Hidden:  true,
		Trigger: task.Trigger{Type: task.TriggerTypeInterval, Interval: "30m"},
		Action:  map[string]any{},
	}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cmd := newHeartbeatIntervalCmd(os.Stdout)
	if err := cmd.RunE(cmd, []string{"1h"}); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	tk, err := store.Get(claw.SystemTaskHeartbeatID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if tk.Trigger.Interval != "1h" {
		t.Fatalf("interval = %#v, want %q", tk.Trigger.Interval, "1h")
	}
}

func TestHeartbeatIntervalReadsTaskTrigger(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	store := setupHeartbeatTaskStore(t)

	if err := store.Save([]task.Task{{
		ID:      claw.SystemTaskHeartbeatID,
		Name:    claw.SystemTaskNameHeartbeat,
		Type:    string(claw.TaskTypeHeartbeat),
		Enabled: true,
		Hidden:  true,
		Trigger: task.Trigger{Type: task.TriggerTypeInterval, Interval: "45m"},
		Action: map[string]any{
			"active_start":  "09:00",
			"active_end":    "21:00",
			"timezone":      "Asia/Shanghai",
			"light_context": true,
			"ack_max_chars": 512,
		},
	}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	bufFile, err := os.CreateTemp(t.TempDir(), "heartbeat-interval-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer bufFile.Close()

	cmd := newHeartbeatIntervalCmd(bufFile)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	data, err := os.ReadFile(bufFile.Name())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if got := strings.TrimSpace(string(data)); got != "45m" {
		t.Fatalf("output = %q, want %q", got, "45m")
	}
}
