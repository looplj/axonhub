package gql

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/plugin"
)

const pluginSelection = `
  id
  pluginID
  name
  version
  description
  enabled
  priority
  builtin
  grantedPermissions
  manifestPermissions
  configFields { name type title description format enumValues minimum maximum defaultJSON }
  config
  runtimeStatus
  runtimeError
`

const pluginsQuery = `query Plugins {
  pluginCountersArePerInstance
  pluginRuntimeStatuses { id pluginID status reason }
  plugins {` + pluginSelection + `}
}`

const installPluginMutation = `mutation InstallPlugin($file: Upload!, $grantedPermissions: [String!]) {
  installPlugin(file: $file, grantedPermissions: $grantedPermissions) {
    plugin {` + pluginSelection + `}
    upgradePreview { currentVersion newVersion permissions events hooks pages }
  }
}`

const enablePluginMutation = `mutation EnablePlugin($id: ID!, $grantedPermissions: [String!]!) {
  enablePlugin(id: $id, grantedPermissions: $grantedPermissions) {` + pluginSelection + `}
}`

const disablePluginMutation = `mutation DisablePlugin($id: ID!) {
  disablePlugin(id: $id) {` + pluginSelection + `}
}`

const deletePluginMutation = `mutation DeletePlugin($id: ID!) {
  deletePlugin(id: $id)
}`

const updatePluginSettingsMutation = `mutation UpdatePluginSettings($id: ID!, $priority: Int!, $config: Map!) {
  updatePluginSettings(id: $id, priority: $priority, config: $config) {` + pluginSelection + `}
}`

func TestPluginGraphQL(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:plugin_graphql?mode=memory&cache=shared&_fk=0")
	t.Cleanup(func() { _ = db.Close() })
	db.Driver().(*entsql.Driver).DB().SetMaxOpenConns(4)

	manager, err := plugin.NewManager(plugin.ManagerParams{
		Client: db,
		Cache:  xcache.Config{Mode: xcache.ModeMemory},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, manager.Stop(context.Background()))
	})

	owner := &ent.User{ID: 1, IsOwner: true}
	reader := &ent.User{ID: 2, Scopes: []string{"read_plugins"}}
	writer := &ent.User{ID: 3, Scopes: []string{"write_plugins"}}
	neither := &ent.User{ID: 4, Scopes: []string{"read_settings"}}
	current := owner

	handler := NewGraphqlHandlers(Dependencies{Ent: db, PluginManager: manager})
	server := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := ent.NewContext(r.Context(), db)
		ctx = contexts.WithUser(ctx, current)
		handler.Graphql.ServeHTTP(w, r.WithContext(ctx))
	})

	post := func(query string, variables map[string]any) (map[string]any, string) {
		t.Helper()
		payload := map[string]any{"query": query}
		if variables != nil {
			payload["variables"] = variables
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/admin/graphql", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var result map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result), recorder.Body.String())
		return result, recorder.Body.String()
	}
	mustData := func(result map[string]any, raw string) map[string]any {
		t.Helper()
		require.NotContains(t, result, "errors", raw)
		data, ok := result["data"].(map[string]any)
		require.True(t, ok, raw)
		return data
	}
	errorText := func(result map[string]any) string {
		t.Helper()
		items, _ := result["errors"].([]any)
		require.NotEmpty(t, items)
		first, _ := items[0].(map[string]any)
		message, _ := first["message"].(string)
		return message
	}
	install := func(content []byte, granted any) (map[string]any, string) {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		operations, err := json.Marshal(map[string]any{
			"query": installPluginMutation,
			"variables": map[string]any{
				"file":               nil,
				"grantedPermissions": granted,
			},
		})
		require.NoError(t, err)
		require.NoError(t, writer.WriteField("operations", string(operations)))
		require.NoError(t, writer.WriteField("map", `{"0":["variables.file"]}`))
		part, err := writer.CreateFormFile("0", "guard.axplugin")
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		req := httptest.NewRequest(http.MethodPost, "/admin/graphql", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var result map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result), recorder.Body.String())
		return result, recorder.Body.String()
	}
	stored := func() *ent.Plugin {
		t.Helper()
		ctx := authz.WithSystemBypass(context.Background(), "plugin-graphql-test")
		row, err := db.Plugin.Query().Where(entplugin.PluginIDEQ("acme.failover-guard")).Only(ctx)
		require.NoError(t, err)
		return row
	}

	current = neither
	denied, _ := post(pluginsQuery, nil)
	require.Contains(t, errorText(denied), "read_plugins")

	current = reader
	listed, raw := post(pluginsQuery, nil)
	data := mustData(listed, raw)
	require.Equal(t, true, data["pluginCountersArePerInstance"])
	require.Empty(t, data["plugins"])

	fakeID := "gid://axonhub/Plugin/1"
	current = reader
	denied, _ = post(enablePluginMutation, map[string]any{"id": fakeID, "grantedPermissions": []string{}})
	require.Contains(t, errorText(denied), "owner access required")
	denied, _ = post(disablePluginMutation, map[string]any{"id": fakeID})
	require.Contains(t, errorText(denied), "owner access required")
	denied, _ = post(deletePluginMutation, map[string]any{"id": fakeID})
	require.Contains(t, errorText(denied), "owner access required")
	denied, _ = install(uiPluginZip(t, "1.0.0", []string{"read_channels"}), nil)
	require.Contains(t, errorText(denied), "owner access required")

	current = writer
	denied, _ = install(uiPluginZip(t, "1.0.0", []string{"read_channels"}), nil)
	require.Contains(t, errorText(denied), "owner access required")
	denied, _ = post(enablePluginMutation, map[string]any{"id": fakeID, "grantedPermissions": []string{"read_channels"}})
	require.Contains(t, errorText(denied), "owner access required")
	denied, _ = post(updatePluginSettingsMutation, map[string]any{
		"id":       fakeID,
		"priority": 3,
		"config":   map[string]any{},
	})
	require.NotContains(t, errorText(denied), "owner access required")
	require.NotContains(t, errorText(denied), "read_plugins")

	firstZip := uiPluginZip(t, "1.0.0", []string{"read_channels"})
	current = owner
	installed, raw := install(firstZip, nil)
	installData := mustData(installed, raw)["installPlugin"].(map[string]any)
	require.Nil(t, installData["upgradePreview"])
	created := installData["plugin"].(map[string]any)
	require.Equal(t, "acme.failover-guard", created["pluginID"])
	require.Equal(t, "1.0.0", created["version"])
	require.Equal(t, false, created["enabled"])
	require.Equal(t, false, created["builtin"])
	require.Equal(t, "disabled", created["runtimeStatus"])
	require.NotContains(t, raw, "PK")
	pluginID := created["id"].(string)

	current = reader
	listed, raw = post(pluginsQuery, nil)
	data = mustData(listed, raw)
	plugins := data["plugins"].([]any)
	require.Len(t, plugins, 1)
	require.Equal(t, "acme.failover-guard", plugins[0].(map[string]any)["pluginID"])

	current = reader
	denied, _ = install(uiPluginZip(t, "9.9.9", []string{"read_channels"}), nil)
	require.Contains(t, errorText(denied), "owner access required")
	denied, _ = post(enablePluginMutation, map[string]any{"id": pluginID, "grantedPermissions": []string{"read_channels"}})
	require.Contains(t, errorText(denied), "owner access required")
	denied, _ = post(disablePluginMutation, map[string]any{"id": pluginID})
	require.Contains(t, errorText(denied), "owner access required")
	denied, _ = post(deletePluginMutation, map[string]any{"id": pluginID})
	require.Contains(t, errorText(denied), "owner access required")
	require.Equal(t, "1.0.0", stored().Version)

	current = writer
	denied, _ = install(uiPluginZip(t, "9.9.9", []string{"read_channels"}), nil)
	require.Contains(t, errorText(denied), "owner access required")
	denied, _ = post(enablePluginMutation, map[string]any{"id": pluginID, "grantedPermissions": []string{"read_channels"}})
	require.Contains(t, errorText(denied), "owner access required")

	const secret = "s3cret-value"
	updated, raw := post(updatePluginSettingsMutation, map[string]any{
		"id":       pluginID,
		"priority": 12,
		"config":   map[string]any{"token": secret, "threshold": 4},
	})
	updatedPlugin := mustData(updated, raw)["updatePluginSettings"].(map[string]any)
	require.Equal(t, plugin.SecretMask, updatedPlugin["config"].(map[string]any)["token"])
	require.NotContains(t, raw, secret)
	require.Equal(t, float64(12), updatedPlugin["priority"])

	current = reader
	listed, raw = post(pluginsQuery, nil)
	visible := mustData(listed, raw)["plugins"].([]any)[0].(map[string]any)
	require.Equal(t, plugin.SecretMask, visible["config"].(map[string]any)["token"])
	require.NotContains(t, raw, secret)

	current = writer
	updated, raw = post(updatePluginSettingsMutation, map[string]any{
		"id":       pluginID,
		"priority": 15,
		"config":   map[string]any{"token": plugin.SecretMask, "threshold": 6},
	})
	updatedPlugin = mustData(updated, raw)["updatePluginSettings"].(map[string]any)
	require.Equal(t, plugin.SecretMask, updatedPlugin["config"].(map[string]any)["token"])
	require.NotContains(t, raw, secret)
	row := stored()
	require.Equal(t, secret, row.Config["token"])
	require.Equal(t, 15, row.Priority)

	current = owner
	enabled, raw := post(enablePluginMutation, map[string]any{
		"id":                 pluginID,
		"grantedPermissions": []string{"read_channels"},
	})
	enabledPlugin := mustData(enabled, raw)["enablePlugin"].(map[string]any)
	require.Equal(t, true, enabledPlugin["enabled"])
	require.Equal(t, []any{"read_channels"}, enabledPlugin["grantedPermissions"])

	nextZip := uiPluginZip(t, "1.1.0", []string{"read_channels", "write_channels"})
	previewed, raw := install(nextZip, nil)
	previewData := mustData(previewed, raw)["installPlugin"].(map[string]any)
	preview := previewData["upgradePreview"].(map[string]any)
	require.Equal(t, "1.0.0", preview["currentVersion"])
	require.Equal(t, "1.1.0", preview["newVersion"])
	require.Equal(t, []any{"write_channels"}, preview["permissions"])
	require.Empty(t, preview["events"])
	require.Empty(t, preview["hooks"])
	require.Empty(t, preview["pages"])
	require.Equal(t, "1.0.0", previewData["plugin"].(map[string]any)["version"])
	require.Contains(t, previewData["plugin"].(map[string]any)["manifestPermissions"], "write_channels")
	require.Equal(t, "1.0.0", stored().Version)
	require.NotContains(t, stored().Manifest.Permissions, "write_channels")

	confirmed, raw := install(nextZip, []string{"read_channels", "write_channels"})
	confirmedData := mustData(confirmed, raw)["installPlugin"].(map[string]any)
	require.Nil(t, confirmedData["upgradePreview"])
	require.Equal(t, "1.1.0", confirmedData["plugin"].(map[string]any)["version"])
	require.Equal(t, true, confirmedData["plugin"].(map[string]any)["enabled"])
	require.Equal(t, "1.1.0", stored().Version)
	require.Equal(t, secret, stored().Config["token"])
}

func uiPluginZip(t *testing.T, version string, permissions []string) []byte {
	t.Helper()
	encoded, err := json.Marshal(permissions)
	require.NoError(t, err)
	manifest := fmt.Sprintf(`{
	  "schemaVersion": 1,
	  "id": "acme.failover-guard",
	  "name": "Failover Guard",
	  "version": %q,
	  "description": "UI only",
	  "axonhubVersion": "^1.0.0",
	  "pluginApiVersion": 1,
	  "permissions": %s,
	  "configSchema": {
	    "type": "object",
	    "properties": {
	      "threshold": {"type": "integer", "title": "Threshold", "minimum": 1, "default": 10},
	      "token": {"type": "string", "title": "Token", "format": "secret"}
	    }
	  },
	  "ui": {"pages": [{"key": "main", "entry": "ui/page.html", "title": "Overview"}]}
	}`, version, encoded)

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	write := func(name, content string) {
		t.Helper()
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, content)
		require.NoError(t, err)
	}
	write("manifest.json", manifest)
	write("ui/page.html", "<html>ui</html>")
	require.NoError(t, writer.Close())
	return buf.Bytes()
}
