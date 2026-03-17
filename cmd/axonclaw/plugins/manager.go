package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

type Manager struct {
	dir      string
	executor Executor
	logger   *slog.Logger
}

type ManagerOptions struct {
	Dir      string
	Executor Executor
	Logger   *slog.Logger
}

func NewManager(opts ManagerOptions) *Manager {
	executor := opts.Executor
	if executor == nil {
		executor = NewWasmExecutor(opts.Logger)
	}

	return &Manager{
		dir:      opts.Dir,
		executor: executor,
		logger:   opts.Logger,
	}
}

func (m *Manager) List() ([]LoadedPlugin, error) {
	return LoadFromDir(m.dir)
}

func (m *Manager) CallHook(ctx context.Context, hook string, input any) ([]json.RawMessage, error) {
	plugins, err := m.List()
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal hook %q input: %w", hook, err)
	}

	results := make([]json.RawMessage, 0, len(plugins))
	for _, plugin := range plugins {
		export, ok := plugin.ExportForHook(hook)
		if !ok {
			continue
		}

		result, err := m.executor.Call(ctx, plugin, export, payload)
		if err != nil {
			return nil, err
		}
		if len(result) == 0 {
			continue
		}

		var failure struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(result, &failure); err == nil && failure.Error != "" {
			return nil, fmt.Errorf("plugin %q hook %q failed: %s", plugin.Manifest.Name, hook, failure.Error)
		}

		results = append(results, append(json.RawMessage(nil), result...))
	}

	return results, nil
}
