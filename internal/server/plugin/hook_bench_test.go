package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

// BenchmarkHook measures one onRequest that changes a single field on a 100 KiB
// request. Setup, compilation, and the first VM load stay outside the timer.
// The timed call is Manager.Invoke: JSON parse, the script, and JSON return.
func BenchmarkHook(b *testing.B) {
	client := enttest.NewEntClient(b, "sqlite3", "file:hook_bench?mode=memory&cache=shared&_fk=0")
	b.Cleanup(func() {
		require.NoError(b, client.Close())
	})
	manager, err := newManager(ManagerParams{
		Client: client,
		Cache:  xcache.Config{Mode: xcache.ModeMemory},
	}, managerConfig{
		refreshInterval: time.Hour,
		debounce:        time.Hour,
	})
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, manager.Stop(context.Background()))
	})

	var manifest map[string]any
	require.NoError(b, json.Unmarshal([]byte(validManifestJSON), &manifest))
	manifest["id"] = "acme.hook-bench"
	manifest["name"] = "Hook Bench"
	manifestJSON, err := json.Marshal(manifest)
	require.NoError(b, err)
	archive := zipFiles(b, map[string]string{
		"manifest.json": string(manifestJSON),
		"ui/page.html":  "<html></html>",
		"plugin.js": `
export function onRequest(ctx, req) {
  return { ...req, temperature: 0.2 };
}
`,
	})
	installed, err := manager.Install(pluginCtx(), archive, nil, nil)
	require.NoError(b, err)
	require.NoError(b, manager.Enable(pluginCtx(), installed.Plugin.ID, nil, nil))

	request := hookBenchRequest(100 << 10)
	rawRequest := mustJSON(b, request)
	args, err := json.Marshal([]any{json.RawMessage(rawRequest)})
	require.NoError(b, err)
	b.Logf("request JSON %d bytes", len(rawRequest))

	call := InvokeRequest{
		InstallationID: installed.Plugin.ID,
		Export:         "onRequest",
		Args:           args,
		State:          json.RawMessage(`{}`),
		Timeout:        10 * time.Second,
	}
	warm, err := manager.Invoke(context.Background(), call)
	require.NoError(b, err)
	require.Contains(b, string(warm.Value), `"temperature":0.2`)
	require.Contains(b, string(warm.Value), `"model":"bench-model"`)

	b.ReportAllocs()
	b.SetBytes(int64(len(rawRequest)))
	b.ResetTimer()
	for b.Loop() {
		result, err := manager.Invoke(context.Background(), call)
		if err != nil {
			b.Fatal(err)
		}
		if len(result.Value) == 0 {
			b.Fatal("empty hook result")
		}
	}
}

func hookBenchRequest(target int) map[string]any {
	const overhead = len(`{"messages":[{"content":"","role":"user"}],"model":"bench-model","temperature":1}`)
	payload := max(target-overhead, 1)
	return map[string]any{
		"model":       "bench-model",
		"temperature": 1,
		"messages": []any{
			map[string]any{"role": "user", "content": strings.Repeat("a", payload)},
		},
	}
}

func mustJSON(b *testing.B, value any) []byte {
	b.Helper()
	raw, err := json.Marshal(value)
	require.NoError(b, err)
	return raw
}

func zipFiles(b *testing.B, files map[string]string) []byte {
	b.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range files {
		entry, err := writer.Create(name)
		require.NoError(b, err)
		_, err = entry.Write([]byte(content))
		require.NoError(b, err)
	}
	require.NoError(b, writer.Close())
	return buf.Bytes()
}
