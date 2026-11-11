package gql

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// Keep all endpoint fixtures identical except for the UI contribution kind.
func asActionsOnly(t *testing.T, source []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	require.NoError(t, err)
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, file := range reader.File {
		stream, err := file.Open()
		require.NoError(t, err)
		body, err := io.ReadAll(stream)
		require.NoError(t, err)
		require.NoError(t, stream.Close())
		if file.Name == "manifest.json" {
			var m map[string]any
			require.NoError(t, json.Unmarshal(body, &m))
			ui := m["ui"].(map[string]any)
			var actions []any
			for _, item := range ui["pages"].([]any) {
				page := item.(map[string]any)
				actions = append(actions, map[string]any{"key": page["key"], "title": page["title"], "slot": "app.header.actions", "effect": map[string]any{"type": "dialog", "entry": page["entry"]}})
			}
			m["ui"] = map[string]any{"actions": actions}
			body, err = json.Marshal(m)
			require.NoError(t, err)
		}
		entry, err := writer.Create(file.Name)
		require.NoError(t, err)
		_, err = entry.Write(body)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return out.Bytes()
}

func TestPluginActionsOnlyEndpoints(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_actions_endpoints")
	server := httptest.NewTLSServer(&fetchUpstream{})
	t.Cleanup(server.Close)
	fx.manager.SetOutboundTransport(server.Client().Transport)
	target := server.URL + "/v1/check"
	installed, err := fx.manager.Install(fx.setupCtx, asActionsOnly(t, pluginFetchZip(t, "acme.actions", []any{target})), nil, nil)
	require.NoError(t, err)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, nil, []string{target}))
	code, payload, raw := fx.postPlugin(fx.ownerToken, "acme.actions", `mutation { setPluginKV(key:"hello",value:"world") { value } }`)
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, payload, "errors", raw)
	_, payload, raw = fx.postPlugin(fx.readerToken, "acme.actions", `{ pluginKV(key:"hello") { value } }`)
	require.NotContains(t, payload, "errors", raw)
	require.Contains(t, raw, "world")
	_, payload, raw = fx.postPlugin(fx.readerToken, "acme.actions", `mutation { setPluginKV(key:"hello",value:"bad") { value } }`)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
	_, payload, raw = fx.postPlugin(fx.ownerToken, "acme.actions", `mutation { updateMe(input:{firstName:"Hacked"}) { id } }`)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, payload), raw)
	code, payload, raw = fx.postFetch(fx.ownerToken, "acme.actions", map[string]any{"url": target}, "")
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, "ok", payload["body"])
	code, _, raw = fx.postFetch(fx.ownerToken, "acme.actions", map[string]any{"url": server.URL + "/unauthorized"}, "")
	require.Equal(t, http.StatusForbidden, code, raw)
	require.NoError(t, fx.manager.Disable(fx.setupCtx, installed.Plugin.ID))
	code, _, raw = fx.postPlugin(fx.ownerToken, "acme.actions", `{ __typename }`)
	require.Equal(t, http.StatusNotFound, code, raw)
	code, _, raw = fx.postFetch(fx.ownerToken, "acme.actions", map[string]any{"url": target}, "")
	require.Equal(t, http.StatusNotFound, code, raw)
}

func TestPluginActionsCatalogAndSession(t *testing.T) {
	fx := newPluginUIFixture(t, "plugin_actions_catalog")
	fx.current = fx.owner
	installed, raw := fx.install(t, asActionsOnly(t, pluginUIPagesZip(t)))
	id := fx.mustData(t, installed, raw)["installPlugin"].(map[string]any)["plugin"].(map[string]any)["id"].(string)
	enabled, raw := fx.post(t, enablePluginMutation, map[string]any{"id": id, "grantedPermissions": []string{"read_channels"}})
	fx.mustData(t, enabled, raw)
	fx.waitRuntime(t, id, "active")
	fx.current = fx.member
	listed, raw := fx.post(t, `{ pluginUIEntries { pages { key } actions { pluginID key slot title icon revision effect { type entry } } } }`, nil)
	entries := fx.mustData(t, listed, raw)["pluginUIEntries"].(map[string]any)
	require.Empty(t, entries["pages"])
	require.Len(t, entries["actions"], 2)
	require.Contains(t, raw, `"slot":"app.header.actions"`)
	require.NotContains(t, raw, "read_channels")
	session, raw := fx.post(t, createPluginUISessionMutation, map[string]any{"pluginID": "acme.failover-guard", "entry": "ui/page.html"})
	require.NotEmpty(t, fx.mustData(t, session, raw)["createPluginUISession"].(map[string]any)["token"])
}
