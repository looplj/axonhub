// 职责: 在内存站点通过真实任务和页面入口验证目录统计与摘要读取权限。
// 关联文件:
//   internal/server/plugin/tasks.go
//   internal/server/gql/plugin_call.go
//   internal/server/plugin/hostapi_catalog.go
//   internal/server/biz/model_catalog_preview.go
// 注意事项:
//   所有目录请求由内存 HTTP transport 处理。

package gql

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/pluginstorage"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/plugin"
)

func TestPluginCatalogCheck(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	duringFetch := make(chan func() error, 1)
	var unavailable atomic.Bool
	var hits atomic.Int32
	const document = `{"providers":{"selected":{"models":[{"id":"from-provider"}]},"other":{"models":[{"id":"followed","canonical_model_id":"maker/followed","cost":{"input":5,"output":50,"cache_read":2}}]}}}`
	fx := newPluginEndpointFixture(t, "plugin_catalog_check", &http.Client{Transport: pluginRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://catalog.example.invalid/models" {
			return nil, fmt.Errorf("unexpected fixture URL: %s", r.URL)
		}
		hits.Add(1)
		select {
		case action := <-duringFetch:
			if err := action(); err != nil {
				return nil, err
			}
		default:
		}
		status := http.StatusOK
		if unavailable.Load() {
			status = http.StatusServiceUnavailable
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(document))}, nil
	})})
	followed := objects.CatalogPrice{Provider: "other", ModelID: "followed", Canonical: "maker/followed", Follow: true}
	card := &objects.ModelCard{
		Cost: objects.ModelCardCost{Input: 1, Output: 9, CacheRead: 1},
		CatalogPrices: map[string]objects.CatalogPrice{
			"input": followed, "cacheRead": followed,
		},
	}
	row := fx.db.Model.Create().SetModelID("local-name").SetDeveloper("maker").SetName("Local").SetGroup("").SetIcon("").
		SetModelCard(card).SetSettings(&objects.ModelSettings{}).SaveX(fx.setupCtx)
	const pluginID = "acme.catalog-check"
	const manifest = `{
		"schemaVersion":1,"id":"acme.catalog-check","name":"Catalog check","version":"1.0.0","axonhubVersion":"^1.0.0","pluginApiVersion":1,
		"permissions":["read_channels"],"tasks":{"check":{"schedule":{"cron":"30 8 * * *","timezone":"UTC"}}},
		"ui":{"pages":[{"key":"main","entry":"ui/page.html","title":"Summary"}]}}`
	pack := pluginTestZip(t, map[string]string{
		"manifest.json": manifest, "ui/page.html": "<html></html>",
		"plugin.js": `export function onTask(ctx) {
			ctx.checkModelCatalog({providers:["selected"],ignorePrefix:false});
			if (ctx.kv.get("abort") === "yes") throw new Error("fixture task failed after comparing");
			const retry = ctx.kv.get("retry");
			if (retry) {
			    try { ctx.checkModelCatalog(JSON.parse(retry)); }
			    catch (error) { if (error.code !== "INVALID_ARGUMENT") throw error; }
			}
		}`,
	})
	result, err := fx.manager.Install(fx.setupCtx, pack, nil, nil)
	require.NoError(t, err)
	id := result.Plugin.ID
	run := fmt.Sprintf(`mutation {runPluginTask(id:"gid://axonhub/Plugin/%d",key:"check"){status}}`, id)
	check := func(status string) {
		t.Helper()
		code, payload, raw := fx.postAdmin(fx.ownerToken, run)
		require.Equal(t, http.StatusOK, code)
		require.NotContains(t, payload, "errors", raw)
		require.Equal(t, status, payload["data"].(map[string]any)["runPluginTask"].(map[string]any)["status"])
	}
	const summaryQuery = `query { pluginCatalogCheckSummary { lastSuccessAt available changed } }`
	read := func(token string) map[string]any {
		_, response, raw := fx.postPlugin(token, pluginID, summaryQuery)
		require.NotContains(t, response, "errors", raw)
		value := response["data"].(map[string]any)["pluginCatalogCheckSummary"]
		if value == nil {
			return nil
		}
		return value.(map[string]any)
	}
	require.NoError(t, fx.manager.Enable(fx.setupCtx, id, nil, nil))
	check("failed")
	require.Zero(t, hits.Load())
	_, ungranted, _ := fx.postPlugin(fx.ownerToken, pluginID, summaryQuery)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, ungranted))
	require.NoError(t, fx.manager.Enable(fx.setupCtx, id, []string{"read_channels"}, nil))
	require.Nil(t, read(fx.ownerToken))
	peer, err := plugin.NewManager(plugin.ManagerParams{Client: fx.db, Cache: xcache.Config{Mode: xcache.ModeMemory}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, peer.Stop(context.Background())) })
	duringFetch <- func() error { return peer.Disable(fx.setupCtx, id) }
	check("canceled")
	require.NoError(t, fx.manager.Enable(fx.setupCtx, id, []string{"read_channels"}, nil))
	require.Nil(t, read(fx.ownerToken))
	check("succeeded")
	want := read(fx.ownerToken)
	require.Equal(t, 2.0, want["available"])
	require.Equal(t, 1.0, want["changed"])
	require.NotEmpty(t, want["lastSuccessAt"])
	for _, tc := range []struct {
		name, token string
		allowed     bool
	}{
		{"owner", fx.ownerToken, true},
		{"settings", fx.scopedToken("settings", []string{"read_settings"}), true},
		{"editor", fx.scopedToken("editor", []string{"write_channels"}), true},
		{"reader", fx.readerToken, false},
		{"ordinary", fx.scopedToken("ordinary", nil), false},
		{"plugin manager", fx.scopedToken("manager", []string{"write_plugins"}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, response, raw := fx.postPlugin(tc.token, pluginID, summaryQuery)
			if tc.allowed {
				require.NotContains(t, response, "errors", raw)
				require.Equal(t, want, response["data"].(map[string]any)["pluginCatalogCheckSummary"])
				return
			}
			require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, response))
			require.NotContains(t, raw, want["lastSuccessAt"])
		})
	}
	projectToken := fx.scopedToken("projectonly", nil)
	uid := fx.db.User.Query().Where(user.FirstNameEQ("projectonly")).OnlyIDX(fx.setupCtx)
	projectID := fx.db.Project.Query().OnlyIDX(fx.setupCtx)
	fx.db.UserProject.Create().SetUserID(uid).SetProjectID(projectID).SetScopes([]string{"read_settings", "write_channels"}).SaveX(fx.setupCtx)
	_, projectResult, _ := fx.post("/admin/plugins/"+pluginID+"/graphql", projectToken, summaryQuery, nil, projectID)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, projectResult))
	_, direct, _ := fx.postAdmin(fx.ownerToken, summaryQuery)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, direct))
	require.NoError(t, fx.manager.Enable(fx.setupCtx, fx.other.ID, []string{"read_channels"}, nil))
	_, other, raw := fx.postPlugin(fx.ownerToken, pluginOtherID, summaryQuery)
	require.NotContains(t, other, "errors", raw)
	require.Nil(t, other["data"].(map[string]any)["pluginCatalogCheckSummary"])
	for _, query := range []string{
		`{pluginKV(namespace:"/host",key:"model-catalog-check"){value}}`,
		`{pluginKVList(namespace:"/host"){entries{value}}}`,
		`mutation{setPluginKV(namespace:"/host",key:"model-catalog-check",value:"test"){value}}`,
		`mutation{deletePluginKV(namespace:"/host",key:"model-catalog-check")}`,
	} {
		_, response, raw := fx.postPlugin(fx.ownerToken, pluginID, query)
		require.Contains(t, response, "errors", raw)
		require.NotContains(t, raw, want["lastSuccessAt"])
	}
	_, defaultList, raw := fx.postPlugin(fx.readerToken, pluginID, `{pluginKVList{entries{key value}}}`)
	require.NotContains(t, defaultList, "errors", raw)
	require.Empty(t, defaultList["data"].(map[string]any)["pluginKVList"].(map[string]any)["entries"])
	unavailable.Store(true)
	check("failed")
	require.Equal(t, want, read(fx.ownerToken))
	unavailable.Store(false)
	_, err = fx.manager.KVSet(fx.setupCtx, pluginID, id, "", "abort", "yes", 0)
	require.NoError(t, err)
	before := hits.Load()
	check("failed")
	require.Equal(t, want, read(fx.ownerToken))
	require.Equal(t, before+1, hits.Load())
	_, err = fx.manager.KVDelete(fx.setupCtx, pluginID, id, "", "abort")
	require.NoError(t, err)
	for _, invalid := range []string{`{"ignorePrefix":null}`, `{"providers":[null]}`, `{"providers":null}`} {
		_, err := fx.manager.KVSet(fx.setupCtx, pluginID, id, "", "retry", invalid, 0)
		require.NoError(t, err)
		check("succeeded") // The script caught the error. Its first check must not be published.
		require.Equal(t, want, read(fx.ownerToken))
	}
	_, err = fx.manager.KVDelete(fx.setupCtx, pluginID, id, "", "retry")
	require.NoError(t, err)
	// A peer can revoke the grant before this process reloads the installation.
	duringFetch <- func() error { return peer.Enable(fx.setupCtx, id, nil, nil) }
	check("canceled")
	require.Contains(t, fx.manager.Current().ByID(id).GrantedPermissions, "read_channels")
	_, denied, _ := fx.postPlugin(fx.ownerToken, pluginID, summaryQuery)
	require.Equal(t, "FORBIDDEN", graphqlErrorCode(t, denied))
	before = hits.Load()
	check("failed")
	require.Equal(t, before, hits.Load())
	require.NoError(t, fx.manager.Enable(fx.setupCtx, id, []string{"read_channels"}, nil))
	require.Equal(t, want, read(fx.ownerToken))
	duringFetch <- func() error {
		cancel()
		return nil
	}
	canceled, err := fx.manager.RunTask(cancelCtx, id, "check")
	require.NoError(t, err)
	require.Equal(t, "canceled", canceled.Status)
	require.Equal(t, want, read(fx.ownerToken))
	require.Equal(t, 1, fx.db.Model.Query().CountX(fx.setupCtx))
	require.Equal(t, card, fx.db.Model.GetX(fx.setupCtx, row.ID).ModelCard)
	require.NoError(t, fx.manager.Delete(fx.setupCtx, id))
	require.Zero(t, fx.db.PluginStorage.Query().Where(pluginstorage.InstallationIDEQ(id)).CountX(fx.setupCtx))
}
