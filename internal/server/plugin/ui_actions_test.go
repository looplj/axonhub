package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func actionManifest(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	return mutateManifest(t, func(t *testing.T, manifest map[string]any) {
		action := map[string]any{"key": "inspect", "slot": "app.header.actions", "title": "Inspect", "effect": map[string]any{"type": "dialog", "entry": "ui/page.html"}}
		manifest["ui"] = map[string]any{"actions": []any{action}}
		if mutate != nil {
			mutate(manifest)
		}
	})
}

func TestUIActionsManifest(t *testing.T) {
	raw := actionManifest(t, nil)
	manifest, err := ParseManifest(raw, "v1.0.0")
	require.NoError(t, err)
	require.True(t, manifest.UI.HasEntries())
	require.Equal(t, []string{"ui/page.html"}, manifest.UI.Entries())
	require.Empty(t, manifest.UI.Pages)
	encoded, err := json.Marshal(manifest)
	require.NoError(t, err)
	again, err := ParseManifest(encoded, "v1.0.0")
	require.NoError(t, err)
	require.Equal(t, manifest.UI, again.UI)

	cases := []struct {
		name, want string
		mutate     func(map[string]any)
	}{
		{"unknown slot", "slot", func(a map[string]any) { a["slot"] = "channel.actions" }},
		{"unknown icon", "icon", func(a map[string]any) { a["icon"] = "https://example.com/icon.svg" }},
		{"unknown action field", "ui.actions[0].height", func(a map[string]any) { a["height"] = 320 }},
		{"unknown effect field", "ui.actions[0].effect.url", func(a map[string]any) { a["effect"].(map[string]any)["url"] = "/settings" }},
		{"unknown effect", "effect.type", func(a map[string]any) { a["effect"].(map[string]any)["type"] = "execute" }},
		{"missing entry", "effect.entry", func(a map[string]any) { delete(a["effect"].(map[string]any), "entry") }},
		{"traversal", "entry", func(a map[string]any) { a["effect"].(map[string]any)["entry"] = "ui/../plugin.js" }},
		{"invalid key", "key", func(a map[string]any) { a["key"] = "Bad Key" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := actionManifest(t, func(m map[string]any) { tc.mutate(m["ui"].(map[string]any)["actions"].([]any)[0].(map[string]any)) })
			_, err := ParseManifest(raw, "v1.0.0")
			require.ErrorContains(t, err, tc.want)
		})
	}
	for _, value := range []any{nil, "actions", map[string]any{}} {
		_, err := ParseManifest(actionManifest(t, func(m map[string]any) { m["ui"].(map[string]any)["actions"] = value }), "v1.0.0")
		require.ErrorContains(t, err, "ui.actions must be an array")
	}
	_, err = ParseManifest(actionManifest(t, func(m map[string]any) {
		ui := m["ui"].(map[string]any)
		actions := ui["actions"].([]any)
		ui["actions"] = append(actions, actions[0])
	}), "v1.0.0")
	require.ErrorContains(t, err, "duplicated")
}

func TestUIActionsPackageAndUpgrade(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "ui"), 0o755))
	raw := actionManifest(t, nil)
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0o644))
	_, err := PackDirectory(root)
	require.ErrorContains(t, err, "ui action entry")
	_, err = ParsePackage(mustZip(t, map[string]string{"manifest.json": string(raw)}))
	require.ErrorContains(t, err, "entry")
	require.NoError(t, os.WriteFile(filepath.Join(root, "ui/page.html"), []byte("<html>action</html>"), 0o644))
	packed, err := PackDirectory(root)
	require.NoError(t, err)
	pkg, err := ParsePackage(packed)
	require.NoError(t, err)
	require.Len(t, pkg.Manifest.UI.Actions, 1)

	manager := newTestManager(t, newPluginDB(t))
	old := installUI(t, manager, uiZip(t, nil, nil))
	require.NoError(t, manager.Enable(pluginCtx(), old.ID, nil, nil))
	upgraded := actionManifest(t, func(m map[string]any) { m["version"] = "1.1.0" })
	result, err := manager.Install(pluginCtx(), mustZip(t, map[string]string{"manifest.json": string(upgraded), "ui/page.html": "<html>action</html>"}), nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result.Preview)
	require.Equal(t, []string{"inspect"}, result.Preview.Actions)
	require.Equal(t, []string{"inspect"}, normalizePreview(result.Preview).Actions)
	require.Equal(t, "1.0.0", manager.Current().ByPluginID("acme.failover-guard").Version)
}

func TestUIActionsResourcesAndCatalog(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	installed := installUI(t, manager, mustZip(t, map[string]string{"manifest.json": string(actionManifest(t, nil)), "ui/page.html": "<html>action</html>", "ui/other.html": "other"}))
	require.Empty(t, manager.UIEntries().Actions)
	require.NoError(t, manager.Enable(pluginCtx(), installed.ID, nil, nil))
	entries := manager.UIEntries()
	require.Empty(t, entries.Pages)
	require.Len(t, entries.Actions, 1)
	require.Equal(t, "puzzle", entries.Actions[0].Icon)
	require.NotEmpty(t, entries.Actions[0].Revision)
	now := time.Now()
	token, err := manager.IssueUIToken("secret", "acme.failover-guard", "ui/page.html", 7, now)
	require.NoError(t, err)
	other, err := manager.IssueUIToken("secret", "acme.failover-guard", "ui/page.html", 7, now)
	require.NoError(t, err)
	require.NotEqual(t, token, other, "independent instances get different resource tokens even in the same second")
	for path, status := range map[string]int{"/ui/page.html": 200, "/ui/other.html": 403, "/manifest.json": 403} {
		recorder := httptest.NewRecorder()
		ServePluginUI(recorder, httptest.NewRequest(http.MethodGet, "/", nil), PluginUIRequest{Manager: manager, Secret: "secret", Token: token, Path: path})
		require.Equal(t, status, recorder.Code, path)
	}
	require.NoError(t, manager.Disable(pluginCtx(), installed.ID))
	require.Empty(t, manager.UIEntries().Actions)
}

func TestUIActionsExample(t *testing.T) {
	packed, err := PackDirectory("../../../plugins/examples/toolbar-notes")
	require.NoError(t, err)
	pkg, err := ParsePackage(packed)
	require.NoError(t, err)
	require.Empty(t, pkg.Manifest.UI.Pages)
	require.Equal(t, "app.header.actions", pkg.Manifest.UI.Actions[0].Slot)
}
