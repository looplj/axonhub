// 职责: 验证插件定时任务的 GraphQL、SQL 和导航角标授权与持久化。
// 关联文件:
//
//	internal/server/gql/plugin.resolvers.go
//	internal/server/gql/plugin_endpoint_test.go
//	internal/server/plugin/tasks.go
//	internal/server/plugin/hostapi_graphql.go
//	internal/server/plugin/ui_badge.go
package gql

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/pluginstorage"
	"github.com/looplj/axonhub/internal/objects"
)

func TestPluginTaskGraphqlFullSQLGrant(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_task_graphql")
	pack := zipPluginTaskFixture(t, map[string]string{
		"manifest.json": `{
  "schemaVersion": 1,
  "id": "acme.cron",
  "name": "Cron",
  "version": "1.0.0",
  "axonhubVersion": "^1.0.0",
  "pluginApiVersion": 1,
  "permissions": ["sql"],
  "tasks": {"report": {"schedule": {"cron": "0 2 * * *"}}}
}`,
		"plugin.js": `export function onTask(ctx, run) {
  if (run.key !== "report" || run.trigger !== "manual") throw new Error("unexpected task");
  ctx.sql.exec("CREATE TABLE IF NOT EXISTS reports_test (id INTEGER)");
  ctx.sql.exec("INSERT INTO reports_test (id) VALUES (?)", [42]);
}`,
	})
	installed, err := fx.manager.Install(fx.setupCtx, pack, nil, nil)
	require.NoError(t, err)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, []string{"sql"}, nil))
	require.NoError(t, fx.manager.Enable(fx.setupCtx, fx.pages.ID, nil, nil))

	id := fmt.Sprintf("gid://axonhub/Plugin/%d", installed.Plugin.ID)
	run := fmt.Sprintf(`mutation Manual { ...Run }
fragment Run on Mutation { ... on Mutation { runPluginTask(id:%q, key:"report") { id status } } }`, id)

	_, denied, raw := fx.postAdmin(fx.writerToken, run)
	require.Contains(t, denied, "errors", raw)
	_, denied, raw = fx.postPlugin(fx.ownerToken, pluginPagesID, run)
	require.Contains(t, denied, "errors", raw)

	code, allowed, raw := fx.postAdmin(fx.ownerToken, run)
	require.Equal(t, http.StatusOK, code, raw)
	require.NotContains(t, allowed, "errors", raw)
	require.Contains(t, raw, `"status":"succeeded"`)

	_, history, raw := fx.postAdmin(fx.ownerToken, fmt.Sprintf(`query { pluginTasks(id:%q) { key runs { status } } }`, id))
	require.NotContains(t, history, "errors", raw)
	require.Contains(t, raw, `"key":"report","runs":[{"status":"succeeded"}]`)
}

func TestPluginTaskGraphQLAndBadges(t *testing.T) {
	const fetchModels = `{ fetchModels(input:{channelType:"openai",baseURL:"https://model-fetch.example.invalid/v1",apiKey:"test-key"}) { models { id } } }`
	var modelFetchCalls atomic.Int32
	fx := newPluginEndpointFixture(t, "plugin_task_host_api", &http.Client{Transport: pluginRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://model-fetch.example.invalid/v1/models" {
			modelFetchCalls.Add(1)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"upstream-model"}]}`))}, nil
		}
		if req.URL.String() != "https://catalog.example.invalid/models" {
			return nil, fmt.Errorf("unexpected catalog URL: %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"providers":{"openai":{"models":[{"id":"catalog-model","cost":{"input":5}}]}}}`))}, nil
	})})
	fx.db.Model.Create().SetName("Local Model").SetModelID("local-model").
		SetDeveloper("openai").SetIcon("").SetGroup("").
		SetSettings(&objects.ModelSettings{}).
		SetModelCard(&objects.ModelCard{
			CatalogAliases: []string{"catalog-alias"},
			Cost:           objects.ModelCardCost{Input: 1},
			CatalogPrices:  map[string]objects.CatalogPrice{"input": {Provider: "openai", ModelID: "catalog-model", Follow: true}},
		}).SaveX(fx.setupCtx)
	pack := zipPluginTaskFixture(t, map[string]string{
		"manifest.json": `{
  "schemaVersion": 1, "id": "acme.host-api", "name": "Host API", "version": "1.0.0",
  "axonhubVersion": "^1.0.0", "pluginApiVersion": 1, "permissions": ["read_channels", "write_channels"],
  "tasks": {"report": {"schedule": {"cron": "0 2 * * *"}}},
  "ui": {
    "pages": [{"key":"status","entry":"ui/page.html","title":"Status"}],
    "actions": [{"key":"status","slot":"app.header.actions","title":"Status","effect":{"type":"dialog","entry":"ui/action.html"}}]
  }
}`,
		"ui/page.html":   "<html><body>page</body></html>",
		"ui/action.html": "<html><body>action</body></html>",
		"plugin.js": fmt.Sprintf(`export function onTask(ctx) {
  const read = ctx.graphql("query Read($first: Int!) { channels(first: $first) { edges { node { id name supportedModels credentials { apiKey } } } } }", {first: 1}, {operationName: "Read"});
  const write = ctx.graphql("mutation Rename($id: ID!, $name: String!) { updateChannel(id: $id, input: { name: $name }) { name } }", {id: %q, name: "updated by plugin"});
  const user = ctx.graphql("{ me { id } }");
  const models = ctx.graphql("{ models(first: 5) { edges { node { modelID catalogAliases catalogPrices { modelID follow value } } } } }");
  const catalog = ctx.graphql("{ modelCatalogOffers(refresh: true) { source offers { modelID rates { base } } } }");
  const fetchModels = ctx.graphql(%q);
  ctx.kv.set("result", JSON.stringify({read, write, user, models, catalog, fetchModels}));
  if (ctx.kv.get("clear") === "yes") {
    ctx.ui.setBadge({type: "action", key: "status"}, null);
    return;
  }
  ctx.ui.setBadge({type: "page", key: "status"}, {count: 2, tone: "warning"});
  ctx.ui.setBadge({type: "action", key: "status"}, {dot: true});
  try {
    ctx.ui.setBadge({type: "action", key: "other"}, {count: 100});
    throw new Error("setBadge accepted an undeclared entry");
  } catch (error) {
    if (error.code !== "NOT_FOUND") throw error;
  }
}`, fmt.Sprintf("gid://axonhub/Channel/%d", fx.channelID), fetchModels),
	})
	installed, err := fx.manager.Install(fx.setupCtx, pack, nil, nil)
	require.NoError(t, err)
	id := installed.Plugin.ID
	manifestID := installed.Plugin.PluginID
	run := fmt.Sprintf(`mutation { runPluginTask(id:"gid://axonhub/Plugin/%d", key:"report") {status} }`, id)

	check := func(grants []string) map[string]json.RawMessage {
		t.Helper()
		require.NoError(t, fx.manager.Enable(fx.setupCtx, id, grants, nil))
		_, response, raw := fx.postAdmin(fx.ownerToken, run)
		require.NotContains(t, response, "errors", raw)
		require.Contains(t, raw, `"status":"succeeded"`)
		value, err := fx.manager.KVGet(fx.setupCtx, manifestID, id, "", "result")
		require.NoError(t, err)
		require.NotNil(t, value)
		var result map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(value.Value), &result))
		return result
	}
	denied := check(nil)
	require.Contains(t, string(denied["read"]), `"errors"`)
	require.Contains(t, string(denied["write"]), `"errors"`)
	require.Contains(t, string(denied["user"]), `"errors"`)
	require.Contains(t, string(denied["catalog"]), `"errors"`)
	require.Equal(t, "seeded channel", fx.db.Channel.GetX(fx.setupCtx, fx.channelID).Name)

	reader := check([]string{"read_channels"})
	require.Contains(t, string(reader["read"]), "seeded channel")
	require.Contains(t, string(reader["read"]), "gpt-4")
	require.NotContains(t, string(reader["read"]), pluginKVSecret)
	require.Contains(t, string(reader["write"]), `"errors"`)
	require.Contains(t, string(reader["models"]), `"catalogAliases":["catalog-alias"]`)
	require.Contains(t, string(reader["models"]), `{"modelID":"catalog-model","follow":true,"value":1}`)
	require.Contains(t, string(reader["catalog"]), `"source":"upstream"`)
	require.Contains(t, string(reader["catalog"]), `"modelID":"catalog-model"`)

	writer := check([]string{"read_channels", "write_channels"})
	require.NotContains(t, string(writer["write"]), `"errors"`)
	require.Contains(t, string(writer["write"]), "updated by plugin")
	require.Contains(t, string(writer["user"]), `"errors"`)
	require.Equal(t, "updated by plugin", fx.db.Channel.GetX(fx.setupCtx, fx.channelID).Name)
	require.Contains(t, string(writer["fetchModels"]), `"code":"FORBIDDEN"`)
	_, pageProbe, _ := fx.postPlugin(fx.writerToken, manifestID, fetchModels)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, pageProbe))
	require.Zero(t, modelFetchCalls.Load())
	_, adminProbe, raw := fx.postAdmin(fx.writerToken, fetchModels)
	require.NotContains(t, adminProbe, "errors", raw)
	require.Contains(t, raw, "upstream-model")
	require.EqualValues(t, 1, modelFetchCalls.Load())

	const badges = `{ pluginUIEntries { pages {key badge {count dot tone}} actions {key badge {count dot tone}} } }`
	_, result, raw := fx.postAdmin(fx.ownerToken, badges)
	require.NotContains(t, result, "errors", raw)
	require.Contains(t, raw, `"badge":{"count":2,"dot":false,"tone":"warning"}`)
	require.Contains(t, raw, `"badge":{"count":null,"dot":true,"tone":"accent"}`)
	require.Equal(t, 2, fx.db.PluginStorage.Query().Where(pluginstorage.InstallationIDEQ(id), pluginstorage.NamespaceNEQ("default")).CountX(fx.setupCtx))
	_, err = fx.manager.KVSet(fx.setupCtx, manifestID, id, "", "clear", "yes", 0)
	require.NoError(t, err)
	check([]string{"read_channels", "write_channels"})
	_, result, raw = fx.postAdmin(fx.ownerToken, badges)
	require.NotContains(t, result, "errors", raw)
	require.Contains(t, raw, `"badge":{"count":2,"dot":false,"tone":"warning"}`)
	require.Contains(t, raw, `"actions":[{"key":"status","badge":null}]`)

	require.NoError(t, fx.manager.Delete(fx.setupCtx, id))
	require.Zero(t, fx.db.PluginStorage.Query().Where(pluginstorage.InstallationIDEQ(id)).CountX(fx.setupCtx))
}

func zipPluginTaskFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, contents := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, contents)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}
