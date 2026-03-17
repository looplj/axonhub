package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Manifest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Wasm        string            `json:"wasm"`
	Hooks       map[string]string `json:"hooks"`
}

type LoadedPlugin struct {
	Manifest     Manifest
	ManifestPath string
	WasmPath     string
}

func (p LoadedPlugin) ExportForHook(hook string) (string, bool) {
	export, ok := p.Manifest.Hooks[hook]
	return strings.TrimSpace(export), ok && strings.TrimSpace(export) != ""
}

func LoadFromDir(dir string) ([]LoadedPlugin, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read plugins directory: %w", err)
	}

	plugins := make([]LoadedPlugin, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		manifestPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read plugin manifest %q: %w", manifestPath, err)
		}

		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("decode plugin manifest %q: %w", manifestPath, err)
		}

		if strings.TrimSpace(manifest.Name) == "" {
			return nil, fmt.Errorf("plugin manifest %q is missing name", manifestPath)
		}
		if strings.TrimSpace(manifest.Wasm) == "" {
			return nil, fmt.Errorf("plugin manifest %q is missing wasm path", manifestPath)
		}
		if len(manifest.Hooks) == 0 {
			return nil, fmt.Errorf("plugin manifest %q does not declare any hooks", manifestPath)
		}

		wasmPath := manifest.Wasm
		if !filepath.IsAbs(wasmPath) {
			wasmPath = filepath.Join(filepath.Dir(manifestPath), wasmPath)
		}

		plugins = append(plugins, LoadedPlugin{
			Manifest:     manifest,
			ManifestPath: manifestPath,
			WasmPath:     wasmPath,
		})
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Manifest.Name < plugins[j].Manifest.Name
	})

	return plugins, nil
}
