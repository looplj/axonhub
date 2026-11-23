// 职责: 验证动态 SQL 的完整授权、主库撤权和插件定时任务的数据库领取。
// 关联文件:
//
//	internal/server/plugin/hostapi_sql.go
//	internal/server/plugin/tasks.go
//	internal/server/plugin/manager_test.go

package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/pluginschedule"
	"github.com/looplj/axonhub/internal/objects"
)

func TestPluginDynamicSQLFullGrant(t *testing.T) {
	testPluginDynamicSQLFullGrant(t, newPluginDB(t))
}

func testPluginDynamicSQLFullGrant(t *testing.T, client *ent.Client) {
	t.Helper()
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	source := `
export function test(ctx) {
  try {
    const bind = ctx.sql.dialect === "postgres" ? "$1" : "?";
    ctx.sql.exec("CREATE TABLE IF NOT EXISTS plugin_sql_test (test_value TEXT NOT NULL)");
    ctx.sql.exec("INSERT INTO plugin_sql_test (test_value) VALUES (" + bind + ")", ["hello 'SQL'"]);
    const column = "test_value";
    return ctx.sql.query("SELECT " + column + " FROM plugin_sql_test WHERE test_value = " + bind, ["hello 'SQL'"]);
  } catch (error) {
    return {code: error.code};
  }
}
`
	installed := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.sql-test"
		manifest["permissions"] = []string{"sql"}
	}, map[string]string{"plugin.js": source, "ui/page.html": "<html></html>"}))
	invoke := func() string {
		res, err := manager.Invoke(context.Background(), InvokeRequest{InstallationID: installed.ID, Export: "test"})
		require.NoError(t, err)
		return string(res.Value)
	}
	require.NoError(t, manager.Enable(ctx, installed.ID, nil, nil))
	require.JSONEq(t, `{"code":"PERMISSION_DENIED"}`, invoke())

	require.NoError(t, manager.Enable(ctx, installed.ID, []string{"sql"}, nil))
	require.JSONEq(t, `{"columns":["test_value"],"rows":[["hello 'SQL'"]]}`, invoke())

	// A different server can revoke the grant before this runtime refreshes.
	require.NoError(t, client.Plugin.UpdateOneID(installed.ID).SetGrantedPermissions([]string{}).Exec(ctx))
	require.Contains(t, manager.Current().ByID(installed.ID).GrantedPermissions, "sql")
	require.JSONEq(t, `{"code":"PERMISSION_DENIED"}`, invoke())
}

func TestPluginTasksClaimAndExecute(t *testing.T) {
	testPluginTasksClaimAndExecute(t, newPluginDB(t))
}

func testPluginTasksClaimAndExecute(t *testing.T, client *ent.Client) {
	t.Helper()
	manager := newTestManager(t, client)
	peer := newTestManager(t, client)
	ctx := pluginCtx()
	source := `
export function onTask(ctx, run) {
  if (!["manual", "scheduled"].includes(run.trigger) || (run.trigger === "manual") !== (run.scheduledAt === null)) {
    throw new Error("unexpected task trigger");
  }
  const bind = ctx.sql.dialect === "postgres" ? "$1" : "?";
  ctx.sql.exec("CREATE TABLE IF NOT EXISTS plugin_scheduled_test (test_key TEXT)");
  ctx.sql.exec("INSERT INTO plugin_scheduled_test (test_key) VALUES (" + bind + ")", [run.key]);
  ctx.kv.set("result", String(ctx.sql.query("SELECT COUNT(*) FROM plugin_scheduled_test").rows[0][0]));
}
`
	installed := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.schedule-test"
		manifest["permissions"] = []string{"sql"}
		manifest["tasks"] = map[string]objects.PluginTaskSpec{
			"report": {Schedule: objects.PluginScheduleSpec{Cron: "* * * * *"}},
		}
	}, map[string]string{"plugin.js": source, "ui/page.html": "<html></html>"}))
	require.NoError(t, manager.Enable(ctx, installed.ID, []string{"sql"}, nil))
	require.NoError(t, peer.ForceRefresh(ctx))

	// Another instance can update the stored package before these runtimes refresh.
	cachedSHA := manager.Current().ByID(installed.ID).PackageSHA256
	nextSHA := sha256Hex([]byte("another plugin package"))
	require.NotEqual(t, cachedSHA, nextSHA)
	require.NoError(t, client.Plugin.UpdateOneID(installed.ID).SetPackageSha256(nextSHA).Exec(ctx))
	require.Equal(t, cachedSHA, manager.Current().ByID(installed.ID).PackageSHA256)

	slot := time.Now().UTC().Add(-time.Hour).Truncate(time.Minute)
	run, err := manager.claimTask(ctx, installed.ID, "report", &slot, slot.Add(time.Second))
	require.NoError(t, err)
	require.NotNil(t, run)

	duplicate, err := peer.claimTask(ctx, installed.ID, "report", &slot, slot.Add(time.Second))
	require.NoError(t, err)
	require.Nil(t, duplicate)

	next := slot.Add(time.Minute)
	duplicate, err = peer.claimTask(ctx, installed.ID, "report", &next, next)
	require.NoError(t, err)
	require.Nil(t, duplicate)
	require.NoError(t, manager.finishTask(ctx, run, "succeeded", slot.Add(30*time.Second)))

	// Another tick in the skipped minute cannot reclaim that slot.
	duplicate, err = peer.claimTask(ctx, installed.ID, "report", &next, next)
	require.NoError(t, err)
	require.Nil(t, duplicate)

	manual, err := manager.RunTask(ctx, installed.ID, "report")
	require.NoError(t, err)
	require.Equal(t, "succeeded", manual.Status)
	require.Nil(t, manual.ScheduledAt)
	require.NoError(t, client.Plugin.UpdateOneID(installed.ID).SetPackageSha256(cachedSHA).Exec(ctx))

	state, err := peer.ListTasks(ctx, installed.ID)
	require.NoError(t, err)
	require.Len(t, state, 1)
	require.Len(t, state[0].Runs, 2)
	require.Equal(t, manual.ID, state[0].Runs[0].ID)
	require.NotNil(t, state[0].NextRunAt)
	entry, err := manager.KVGet(ctx, installed.PluginID, installed.ID, "default", "result")
	require.NoError(t, err)
	require.NotNil(t, entry)
	require.Equal(t, "1", entry.Value)

	minute := time.Now().Truncate(time.Minute)
	manager.scanTasks(ctx)
	// A tick can cross the minute boundary before its worker picks up the task.
	if !time.Now().Truncate(time.Minute).Equal(minute) {
		manager.scanTasks(ctx)
	}
	state, err = peer.ListTasks(ctx, installed.ID)
	require.NoError(t, err)
	require.Greater(t, state[0].Runs[0].ID, manual.ID)
	require.NotNil(t, state[0].Runs[0].ScheduledAt)
	require.Equal(t, "succeeded", state[0].Runs[0].Status)

	override := "0 2 * * *"
	updated, err := peer.UpdateTask(ctx, installed.ID, "report", true, &override, nil)
	require.NoError(t, err)
	require.True(t, updated.Paused)
	require.True(t, updated.Overridden)
	require.Equal(t, override, updated.Cron)
	require.Nil(t, updated.NextRunAt)

	require.NoError(t, manager.Disable(ctx, installed.ID))
	_, err = manager.RunTask(ctx, installed.ID, "report")
	require.ErrorContains(t, err, "not active")
	require.NoError(t, manager.Delete(ctx, installed.ID))
	count, err := client.PluginSchedule.Query().Where(pluginschedule.InstallationIDEQ(installed.ID)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestPluginTaskRunSurvivesUpgrade(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	versionPackage := func(version string) []byte {
		t.Helper()
		return uiZip(t, func(_ *testing.T, manifest map[string]any) {
			manifest["id"] = "acme.task-upgrade"
			manifest["version"] = version
			manifest["tasks"] = map[string]objects.PluginTaskSpec{
				"report": {Schedule: objects.PluginScheduleSpec{Cron: "* * * * *"}},
			}
		}, map[string]string{
			"plugin.js":    `export function onTask(ctx) { ctx.kv.set("version", "` + version + `"); }`,
			"ui/page.html": "<html></html>",
		})
	}
	installed := installUI(t, manager, versionPackage("1.0.0"))
	require.NoError(t, manager.Enable(ctx, installed.ID, nil, nil))
	run, err := manager.claimTask(ctx, installed.ID, "report", nil, time.Now().UTC())
	require.NoError(t, err)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	require.True(t, manager.trackTask(run.ID, pluginTaskRun{key: "report", installation: installed.ID, cancel: cancel}))
	defer func() {
		manager.taskMu.Lock()
		delete(manager.taskRuns, run.ID)
		manager.taskMu.Unlock()
	}()

	upgrade, err := manager.Install(ctx, versionPackage("1.1.0"), nil, nil)
	require.NoError(t, err)
	require.Nil(t, upgrade.Preview)
	require.Equal(t, installed.ID, upgrade.Plugin.ID)
	require.NotEqual(t, installed.PackageSha256, upgrade.Plugin.PackageSha256)
	require.NoError(t, runCtx.Err())

	require.NoError(t, manager.finishTask(ctx, run, "succeeded", time.Now().UTC()))
	next, err := manager.RunTask(ctx, installed.ID, "report")
	require.NoError(t, err)
	require.Equal(t, "succeeded", next.Status)
	entry, err := manager.KVGet(ctx, installed.PluginID, installed.ID, "default", "version")
	require.NoError(t, err)
	require.NotNil(t, entry)
	require.Equal(t, "1.1.0", entry.Value)

	require.NoError(t, manager.Disable(ctx, installed.ID))
	require.ErrorIs(t, runCtx.Err(), context.Canceled)
}
