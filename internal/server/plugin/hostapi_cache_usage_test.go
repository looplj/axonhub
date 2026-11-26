// 职责: 验证 onTask 的缓存指标调用、授权撤销和能力隔离。
// 关联文件:
//
//	internal/server/plugin/hostapi_cache_usage.go
//	internal/server/plugin/sql_task_test.go
package plugin

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestPluginTaskCacheUsageGrant(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	project := client.Project.Create().SetName("cache-usage").SaveX(ctx)
	key := client.APIKey.Create().SetProjectID(project.ID).SetName("cache-usage").SetKey("not-in-results").SaveX(ctx)
	req := client.Request.Create().SetProjectID(project.ID).SetAPIKeyID(key.ID).SetModelID("test").
		SetRequestBody([]byte(`{"secret":"not-in-results"}`)).SetStatus(request.StatusCompleted).SaveX(ctx)
	client.UsageLog.Create().SetProjectID(project.ID).SetAPIKeyID(key.ID).SetRequestID(req.ID).SetModelID("test").
		SetPromptTokens(100).SetPromptCachedTokens(25).SetCacheReadStatus(string(llm.CacheReadReported)).SaveX(ctx)
	source := fmt.Sprintf(`
export function outsideTask(ctx) { return typeof ctx.usage }

export function onTask(ctx, run) {
  const result = {};
  try { ctx.sql.query('SELECT 1') } catch (e) { result.sql = e.code }
  try {
    ctx.usage.cacheStats({projectId: %d, allKeys: true, statement: 'SELECT 1'});
  } catch (e) { result.invalid = e.code }
  try {
    const page = ctx.usage.cacheStats({projectId: %d, allKeys: true});
    result.items = page.items;
  } catch (e) { result.cache = e.code }
  if (run) ctx.kv.set('cache_result', JSON.stringify(result));
  return result;
}`, project.ID, project.ID)
	install := func(id string) int {
		t.Helper()
		return installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
			manifest["id"] = id
			manifest["permissions"] = []string{permReadCacheUsage, permSQL}
			manifest["tasks"] = map[string]objects.PluginTaskSpec{
				"report": {Schedule: objects.PluginScheduleSpec{Cron: "* * * * *"}},
			}
		}, map[string]string{"plugin.js": source, "ui/page.html": "<html></html>"})).ID
	}
	const pluginID = "acme.cache-usage-test"
	id := install(pluginID)
	peerID := install("acme.cache-usage-peer")
	invoke := func(id int, export string) string {
		t.Helper()
		res, err := manager.Invoke(t.Context(), InvokeRequest{InstallationID: id, Export: export})
		require.NoError(t, err)
		return string(res.Value)
	}
	require.NoError(t, manager.Enable(ctx, peerID, []string{permSQL}, nil))
	require.NoError(t, manager.Enable(ctx, id, []string{permReadCacheUsage}, nil))
	require.JSONEq(t, `{"invalid":"PERMISSION_DENIED","cache":"PERMISSION_DENIED"}`, invoke(peerID, pluginTaskExport))

	require.JSONEq(t, `"undefined"`, invoke(id, "outsideTask"))
	run, err := manager.RunTask(ctx, id, "report")
	require.NoError(t, err)
	require.Equal(t, "succeeded", run.Status)
	entry, err := manager.KVGet(ctx, pluginID, id, "default", "cache_result")
	require.NoError(t, err)
	require.NotNil(t, entry)
	require.JSONEq(t, fmt.Sprintf(`{"sql":"PERMISSION_DENIED","invalid":"INVALID_ARGUMENT","items":[
{"keyId":%d,"count":1,"validCount":1,"unknownCount":0,"invalidCount":0,"promptTokens":100,"cachedTokens":25,"cacheRate":0.25,"truncated":false}]}`, key.ID), entry.Value)
	require.NotContains(t, entry.Value, "not-in-results")

	// A direct database update simulates a revoke on another server: the local
	// execution snapshot still carries the permission until the next refresh.
	require.NoError(t, client.Plugin.UpdateOneID(id).SetGrantedPermissions([]string{}).Exec(ctx))
	require.Contains(t, manager.Current().ByID(id).GrantedPermissions, permReadCacheUsage)
	require.JSONEq(t, `{"sql":"PERMISSION_DENIED","cache":"PERMISSION_DENIED","invalid":"INVALID_ARGUMENT"}`, invoke(id, pluginTaskExport))
	require.NoError(t, client.Plugin.UpdateOneID(id).SetGrantedPermissions([]string{permReadCacheUsage}).SetEnabled(false).Exec(ctx))
	require.JSONEq(t, `{"sql":"PERMISSION_DENIED","cache":"PERMISSION_DENIED","invalid":"INVALID_ARGUMENT"}`, invoke(id, pluginTaskExport))

	// A grant added after the invocation snapshot cannot expand it.
	require.NoError(t, manager.Enable(ctx, id, nil, nil))
	require.NoError(t, client.Plugin.UpdateOneID(id).SetGrantedPermissions([]string{permReadCacheUsage}).Exec(ctx))
	require.NotContains(t, manager.Current().ByID(id).GrantedPermissions, permReadCacheUsage)
	require.JSONEq(t, `{"sql":"PERMISSION_DENIED","cache":"PERMISSION_DENIED","invalid":"PERMISSION_DENIED"}`, invoke(id, pluginTaskExport))
}
