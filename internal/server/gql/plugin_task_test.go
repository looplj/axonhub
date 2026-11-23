// 职责: 验证插件定时任务通过管理 GraphQL 执行动态 SQL 的授权和事务边界。
// 关联文件:
//
//	internal/server/gql/plugin.resolvers.go
//	internal/server/gql/plugin_endpoint_test.go
//	internal/server/plugin/tasks.go
//	internal/server/plugin/hostapi_sql.go
package gql

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPluginTaskGraphqlFullSQLGrant(t *testing.T) {
	fx := newPluginEndpointFixture(t, "plugin_task_graphql")
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, contents := range map[string]string{
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
	} {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, contents)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	installed, err := fx.manager.Install(fx.setupCtx, buf.Bytes(), nil, nil)
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
