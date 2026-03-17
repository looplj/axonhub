package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type stubExecutor struct {
	calledPlugin string
	calledExport string
	calledInput  []byte
	result       []byte
}

func (s *stubExecutor) Call(_ context.Context, plugin LoadedPlugin, export string, input []byte) ([]byte, error) {
	s.calledPlugin = plugin.Manifest.Name
	s.calledExport = export
	s.calledInput = append([]byte(nil), input...)
	return append([]byte(nil), s.result...), nil
}

func TestManagerCallHookDispatchesDeclaredWasmHook(t *testing.T) {
	pluginsDir := t.TempDir()
	manifest := []byte(`{
		"name": "memory-wasm",
		"description": "test plugin",
		"wasm": "memory-plugin.wasm",
		"hooks": {
			"memory.search": "memory_search"
		}
	}`)
	if err := os.WriteFile(filepath.Join(pluginsDir, "memory-plugin.json"), manifest, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "memory-plugin.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result := []byte(`{"results":[{"text":"wasm: release notes"}]}`)
	executor := &stubExecutor{result: result}
	mgr := NewManager(ManagerOptions{Dir: pluginsDir, Executor: executor})

	results, err := mgr.CallHook(context.Background(), "memory.search", map[string]any{"query": "release notes"})
	if err != nil {
		t.Fatalf("CallHook() error = %v", err)
	}

	if executor.calledPlugin != "memory-wasm" || executor.calledExport != "memory_search" {
		t.Fatalf("executor called plugin=%q export=%q", executor.calledPlugin, executor.calledExport)
	}

	var input map[string]any
	if err := json.Unmarshal(executor.calledInput, &input); err != nil {
		t.Fatalf("json.Unmarshal(input) error = %v", err)
	}
	if input["query"] != "release notes" {
		t.Fatalf("executor input = %#v", input)
	}

	if len(results) != 1 || string(results[0]) != string(result) {
		t.Fatalf("CallHook() results = %#v", results)
	}
}
