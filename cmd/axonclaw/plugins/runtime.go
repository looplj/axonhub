package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Executor interface {
	Call(ctx context.Context, plugin LoadedPlugin, export string, input []byte) ([]byte, error)
}

type WasmExecutor struct {
	logger *slog.Logger
	cache  sync.Map
}

type wasmCallState struct {
	input  []byte
	output []byte
	err    error
}

func NewWasmExecutor(logger *slog.Logger) *WasmExecutor {
	return &WasmExecutor{logger: logger}
}

func (e *WasmExecutor) Call(ctx context.Context, plugin LoadedPlugin, export string, input []byte) ([]byte, error) {
	wasmBytes, err := e.loadWasm(plugin.WasmPath)
	if err != nil {
		return nil, err
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	state := &wasmCallState{input: input}
	host := runtime.NewHostModuleBuilder("env")
	host.NewFunctionBuilder().WithFunc(func() uint32 {
		return uint32(len(state.input))
	}).Export("host_input_len")
	host.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, ptr uint32, size uint32) {
		if state.err != nil {
			return
		}
		if int(size) > len(state.input) {
			size = uint32(len(state.input))
		}
		if size == 0 {
			return
		}
		if !mod.Memory().Write(ptr, state.input[:size]) {
			state.err = fmt.Errorf("plugin %q failed to read host input", plugin.Manifest.Name)
		}
	}).Export("host_input_copy")
	host.NewFunctionBuilder().WithFunc(func() {
		state.output = state.output[:0]
	}).Export("host_output_reset")
	host.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, ptr uint32, size uint32) {
		if state.err != nil || size == 0 {
			return
		}
		data, ok := mod.Memory().Read(ptr, size)
		if !ok {
			state.err = fmt.Errorf("plugin %q wrote invalid output buffer", plugin.Manifest.Name)
			return
		}
		state.output = append(state.output, data...)
	}).Export("host_output_append")
	host.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, level uint32, ptr uint32, size uint32) {
		if e.logger == nil || size == 0 {
			return
		}
		data, ok := mod.Memory().Read(ptr, size)
		if !ok {
			return
		}
		msg := string(data)
		switch level {
		case 0:
			e.logger.Debug("wasm plugin", "plugin", plugin.Manifest.Name, "message", msg)
		case 1:
			e.logger.Info("wasm plugin", "plugin", plugin.Manifest.Name, "message", msg)
		case 2:
			e.logger.Warn("wasm plugin", "plugin", plugin.Manifest.Name, "message", msg)
		default:
			e.logger.Error("wasm plugin", "plugin", plugin.Manifest.Name, "message", msg)
		}
	}).Export("host_log")

	if _, err := host.Instantiate(ctx); err != nil {
		return nil, fmt.Errorf("instantiate host module for plugin %q: %w", plugin.Manifest.Name, err)
	}
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return nil, fmt.Errorf("instantiate WASI for plugin %q: %w", plugin.Manifest.Name, err)
	}

	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm plugin %q: %w", plugin.Manifest.Name, err)
	}

	guest, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("instantiate wasm plugin %q: %w", plugin.Manifest.Name, err)
	}
	defer guest.Close(ctx)

	fn := guest.ExportedFunction(export)
	if fn == nil {
		return nil, fmt.Errorf("plugin %q does not export %q", plugin.Manifest.Name, export)
	}

	if _, err := fn.Call(ctx); err != nil {
		return nil, fmt.Errorf("execute plugin %q hook %q: %w", plugin.Manifest.Name, export, err)
	}
	if state.err != nil {
		return nil, state.err
	}

	return append([]byte(nil), state.output...), nil
}

func (e *WasmExecutor) loadWasm(path string) ([]byte, error) {
	if cached, ok := e.cache.Load(path); ok {
		return cached.([]byte), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wasm plugin %q: %w", path, err)
	}

	e.cache.Store(path, data)

	return data, nil
}
