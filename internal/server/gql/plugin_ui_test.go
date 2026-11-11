package gql

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/plugin"
)

func TestPluginUIEntries(t *testing.T) {
	fx := newPluginUIFixture(t, "plugin_ui_entries")

	fx.current = nil
	denied, raw := fx.post(t, pluginUIEntriesQuery, nil)
	require.Contains(t, fx.errorText(t, denied), "authentication required", raw)

	fx.current = fx.member
	listed, raw := fx.post(t, pluginUIEntriesQuery, nil)
	data := fx.mustData(t, listed, raw)
	entries := data["pluginUIEntries"].(map[string]any)
	require.Empty(t, entries["pages"])
	require.NotContains(t, raw, "read_channels")

	fx.current = fx.owner
	installed, raw := fx.install(t, pluginUIPagesZip(t))
	created := fx.mustData(t, installed, raw)["installPlugin"].(map[string]any)["plugin"].(map[string]any)
	require.Equal(t, false, created["enabled"])
	pluginRowID := created["id"].(string)

	fx.current = fx.member
	listed, raw = fx.post(t, pluginUIEntriesQuery, nil)
	entries = fx.mustData(t, listed, raw)["pluginUIEntries"].(map[string]any)
	require.Empty(t, entries["pages"], raw)

	fx.current = fx.owner
	enabled, raw := fx.post(t, enablePluginMutation, map[string]any{
		"id":                 pluginRowID,
		"grantedPermissions": []string{"read_channels"},
	})
	require.Equal(t, true, fx.mustData(t, enabled, raw)["enablePlugin"].(map[string]any)["enabled"], raw)
	fx.waitRuntime(t, pluginRowID, "active")

	fx.current = fx.member
	listed, raw = fx.post(t, pluginUIEntriesQuery, nil)
	pages := fx.mustData(t, listed, raw)["pluginUIEntries"].(map[string]any)["pages"].([]any)
	require.Len(t, pages, 2, raw)
	require.NotContains(t, raw, "read_channels")
	require.NotContains(t, raw, "s3cret-value")
	require.NotContains(t, raw, "PK")

	byKey := map[string]any{}
	for _, item := range pages {
		page := item.(map[string]any)
		require.Equal(t, "acme.failover-guard", page["pluginID"])
		require.NotEmpty(t, page["entry"])
		byKey[page["key"].(string)] = page["title"]
	}
	require.Equal(t, "Overview", byKey["main"])
	stats, ok := byKey["stats"].(map[string]any)
	require.True(t, ok, "object title must stay a JSON object, got %#v", byKey["stats"])
	require.Equal(t, "Stats", stats["en"])
	require.Equal(t, "统计", stats["zh-CN"])
	require.Contains(t, raw, `"title":"Overview"`)
	require.Contains(t, raw, `"title":{"en":"Stats","zh-CN":"统计"}`)
}

func TestPluginUISession(t *testing.T) {
	fx := newPluginUIFixture(t, "plugin_ui_session")
	const secret = "s3cret-value"

	fx.current = fx.owner
	installed, raw := fx.install(t, pluginUIPagesZip(t))
	pluginRowID := fx.mustData(t, installed, raw)["installPlugin"].(map[string]any)["plugin"].(map[string]any)["id"].(string)

	fx.current = fx.member
	denied, raw := fx.post(t, createPluginUISessionMutation, map[string]any{
		"pluginID": "acme.failover-guard",
		"entry":    "ui/page.html",
	})
	require.Contains(t, fx.errorText(t, denied), "not active", raw)
	require.NotContains(t, raw, secret)

	fx.current = fx.owner
	enabled, raw := fx.post(t, enablePluginMutation, map[string]any{
		"id":                 pluginRowID,
		"grantedPermissions": []string{"read_channels"},
	})
	require.Equal(t, true, fx.mustData(t, enabled, raw)["enablePlugin"].(map[string]any)["enabled"], raw)
	fx.waitRuntime(t, pluginRowID, "active")

	fx.current = fx.writer
	updated, raw := fx.post(t, updatePluginSettingsMutation, map[string]any{
		"id":       pluginRowID,
		"priority": 4,
		"config":   map[string]any{"token": secret, "threshold": 3},
	})
	require.Equal(t, plugin.SecretMask, fx.mustData(t, updated, raw)["updatePluginSettings"].(map[string]any)["config"].(map[string]any)["token"])
	fx.waitConfigMask(t, secret)

	sessionFor := func(user *ent.User, entry string) (map[string]any, string) {
		t.Helper()
		fx.current = user
		result, body := fx.post(t, createPluginUISessionMutation, map[string]any{
			"pluginID": "acme.failover-guard",
			"entry":    entry,
		})
		data := fx.mustData(t, result, body)
		return data["createPluginUISession"].(map[string]any), body
	}

	memberSession, raw := sessionFor(fx.member, "ui/page.html")
	require.NotEmpty(t, memberSession["token"])
	require.Equal(t, false, memberSession["canWriteKV"])
	require.Equal(t, plugin.SecretMask, memberSession["config"].(map[string]any)["token"])
	require.NotContains(t, raw, secret)

	writerSession, raw := sessionFor(fx.writer, "ui/page.html")
	require.Equal(t, true, writerSession["canWriteKV"])
	require.Equal(t, plugin.SecretMask, writerSession["config"].(map[string]any)["token"])
	require.NotContains(t, raw, secret)

	ownerSession, raw := sessionFor(fx.owner, "ui/stats.html")
	require.Equal(t, true, ownerSession["canWriteKV"])
	require.NotEqual(t, memberSession["token"], ownerSession["token"])
	require.NotContains(t, raw, secret)

	fx.current = fx.owner
	denied, raw = fx.post(t, createPluginUISessionMutation, map[string]any{
		"pluginID": "acme.failover-guard",
		"entry":    "ui/missing.html",
	})
	require.Contains(t, fx.errorText(t, denied), "not declared", raw)

	denied, raw = fx.post(t, createPluginUISessionMutation, map[string]any{
		"pluginID": "acme.missing",
		"entry":    "ui/page.html",
	})
	require.Contains(t, fx.errorText(t, denied), "not active", raw)

	fx.current = fx.owner
	disabled, raw := fx.post(t, disablePluginMutation, map[string]any{"id": pluginRowID})
	require.Equal(t, false, fx.mustData(t, disabled, raw)["disablePlugin"].(map[string]any)["enabled"])
	fx.waitRuntime(t, pluginRowID, "disabled")

	fx.current = fx.owner
	denied, raw = fx.post(t, createPluginUISessionMutation, map[string]any{
		"pluginID": "acme.failover-guard",
		"entry":    "ui/page.html",
	})
	require.Contains(t, fx.errorText(t, denied), "not active", raw)

	fx.current = fx.member
	listed, raw := fx.post(t, pluginUIEntriesQuery, nil)
	require.Empty(t, fx.mustData(t, listed, raw)["pluginUIEntries"].(map[string]any)["pages"])
}

const pluginUIEntriesQuery = `query PluginUIEntries {
  pluginUIEntries { pages { pluginID key entry title } }
}`

const createPluginUISessionMutation = `mutation CreatePluginUISession($pluginID: String!, $entry: String!) {
  createPluginUISession(pluginID: $pluginID, entry: $entry) { token config canWriteKV }
}`

type pluginUIFixture struct {
	current *ent.User
	owner   *ent.User
	writer  *ent.User
	member  *ent.User
	post    func(t *testing.T, query string, variables map[string]any) (map[string]any, string)
	install func(t *testing.T, content []byte) (map[string]any, string)
}

func (fx pluginUIFixture) mustData(t *testing.T, result map[string]any, raw string) map[string]any {
	t.Helper()
	require.NotContains(t, result, "errors", raw)
	data, ok := result["data"].(map[string]any)
	require.True(t, ok, raw)
	return data
}

func (fx *pluginUIFixture) waitRuntime(t *testing.T, id, status string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	prev := fx.current
	fx.current = fx.owner
	var last string
	for time.Now().Before(deadline) {
		result, raw := fx.post(t, `query { plugins { id runtimeStatus } }`, nil)
		last = raw
		data, _ := result["data"].(map[string]any)
		rows, _ := data["plugins"].([]any)
		for _, item := range rows {
			row, _ := item.(map[string]any)
			if row["id"] == id && row["runtimeStatus"] == status {
				fx.current = prev
				return
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	fx.current = prev
	t.Fatalf("runtime status %s not reached: %s", status, last)
}

func (fx *pluginUIFixture) waitConfigMask(t *testing.T, secret string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	prev := fx.current
	fx.current = fx.owner
	var last string
	for time.Now().Before(deadline) {
		result, raw := fx.post(t, createPluginUISessionMutation, map[string]any{
			"pluginID": "acme.failover-guard",
			"entry":    "ui/page.html",
		})
		last = raw
		data, _ := result["data"].(map[string]any)
		session, _ := data["createPluginUISession"].(map[string]any)
		config, _ := session["config"].(map[string]any)
		if config["token"] == plugin.SecretMask && !strings.Contains(raw, secret) {
			fx.current = prev
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	fx.current = prev
	t.Fatalf("masked config not visible: %s", last)
}

func (fx pluginUIFixture) errorText(t *testing.T, result map[string]any) string {
	t.Helper()
	items, _ := result["errors"].([]any)
	require.NotEmpty(t, items)
	first, _ := items[0].(map[string]any)
	message, _ := first["message"].(string)
	return message
}

func newPluginUIFixture(t *testing.T, name string) *pluginUIFixture {
	t.Helper()
	db := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&cache=shared&_fk=0")
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

	setupCtx := authz.WithSystemBypass(ent.NewContext(context.Background(), db), "plugin-ui-test")
	systemSvc := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         db,
	})
	require.NoError(t, systemSvc.SetSecretKey(setupCtx, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))

	handler := NewGraphqlHandlers(Dependencies{
		Ent:           db,
		SystemService: systemSvc,
		PluginManager: manager,
	})

	fx := &pluginUIFixture{
		owner:  &ent.User{ID: 1, IsOwner: true},
		writer: &ent.User{ID: 2, Scopes: []string{"write_plugins"}},
		member: &ent.User{ID: 3, Scopes: []string{"read_settings"}},
	}
	serve := func(t *testing.T, req *http.Request) (map[string]any, string) {
		t.Helper()
		ctx := ent.NewContext(req.Context(), db)
		if fx.current != nil {
			ctx = contexts.WithUser(authz.NewUserContext(ctx, fx.current.ID), fx.current)
		}
		recorder := httptest.NewRecorder()
		handler.Graphql.ServeHTTP(recorder, req.WithContext(ctx))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var result map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result), recorder.Body.String())
		return result, recorder.Body.String()
	}
	fx.post = func(t *testing.T, query string, variables map[string]any) (map[string]any, string) {
		t.Helper()
		payload := map[string]any{"query": query}
		if variables != nil {
			payload["variables"] = variables
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/admin/graphql", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		return serve(t, req)
	}
	fx.install = func(t *testing.T, content []byte) (map[string]any, string) {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		operations, err := json.Marshal(map[string]any{
			"query": installPluginMutation,
			"variables": map[string]any{
				"file":               nil,
				"grantedPermissions": nil,
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
		return serve(t, req)
	}
	return fx
}

func pluginUIPagesZip(t *testing.T) []byte {
	t.Helper()
	manifest := `{
	  "schemaVersion": 1,
	  "id": "acme.failover-guard",
	  "name": "Failover Guard",
	  "version": "1.0.0",
	  "description": "UI only",
	  "axonhubVersion": "^1.0.0",
	  "pluginApiVersion": 1,
	  "permissions": ["read_channels"],
	  "configSchema": {
	    "type": "object",
	    "properties": {
	      "threshold": {"type": "integer", "title": "Threshold", "minimum": 1, "default": 10},
	      "token": {"type": "string", "title": "Token", "format": "secret"}
	    }
	  },
	  "ui": {"pages": [
	    {"key": "main", "entry": "ui/page.html", "title": "Overview"},
	    {"key": "stats", "entry": "ui/stats.html", "title": {"en": "Stats", "zh-CN": "统计"}}
	  ]}
	}`
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
	write("ui/stats.html", "<html>stats</html>")
	require.NoError(t, writer.Close())
	return buf.Bytes()
}
