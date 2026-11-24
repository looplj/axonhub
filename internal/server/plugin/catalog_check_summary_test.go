// 职责: 验证多个插件任务不同时完成时的目录检查摘要。
// 关联文件:
//   internal/server/plugin/catalog_check_summary.go
//   internal/server/plugin/tasks.go
// 注意事项:
//   摘要按安装共享，任务租约按任务标识独立领取。

package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCatalogCheckSummaryCompletionOrder(t *testing.T) {
	db := newPluginDB(t)
	manager := newTestManager(t, db)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		spec := objects.PluginTaskSpec{Schedule: objects.PluginScheduleSpec{Cron: "30 8 * * *"}}
		manifest["tasks"] = map[string]objects.PluginTaskSpec{"first": spec, "second": spec}
	}, map[string]string{"plugin.js": "export function onTask() {}", "ui/page.html": "<html></html>"}))
	require.NoError(t, manager.Enable(ctx, installed.ID, nil, nil))
	now := time.Now().UTC()
	claim := func(key string, start time.Time) *ent.PluginScheduleRun {
		t.Helper()
		run, err := manager.claimTask(ctx, installed.ID, key, nil, start)
		require.NoError(t, err)
		require.NotNil(t, run)
		return run
	}
	finish := func(run *ent.PluginScheduleRun, finished time.Time, available int) {
		t.Helper()
		pending := &pendingCatalogCheck{installationID: installed.ID, counts: biz.CatalogCheckCounts{Available: available}}
		require.NoError(t, manager.finishTask(ctx, run, "succeeded", finished, pending))
	}

	// Each task owns a lease. The earlier completion is the last to commit.
	earlier := claim("first", now.Add(-10*time.Second))
	later := claim("second", now.Add(-10*time.Second))
	finish(later, now, 2)
	finish(earlier, now.Add(-time.Second), 1)
	require.Equal(t, "succeeded", earlier.Status)
	summary, err := readCatalogCheckSummary(ctx, db, installed.ID)
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Equal(t, 2, summary.Available)
	require.Equal(t, now, summary.LastSuccessAt)

	expired := claim("first", now.Add(-2*scheduleLease))
	finish(expired, now.Add(time.Second), 3)
	require.Equal(t, runStatusUnknown, expired.Status)
	successor := claim("first", now)
	finish(expired, now.Add(2*time.Second), 4)
	kept, err := readCatalogCheckSummary(ctx, db, installed.ID)
	require.NoError(t, err)
	require.Equal(t, summary, kept)
	finish(successor, now.Add(3*time.Second), 5)
	updated, err := readCatalogCheckSummary(ctx, db, installed.ID)
	require.NoError(t, err)
	require.Equal(t, 5, updated.Available)

	// A different writer can hold the schedule row until this task's lease expires.
	deadline := time.Now().Truncate(time.Second).Add(time.Second)
	waiting := claim("second", deadline.Add(-scheduleLease))
	db.PluginSchedule.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			time.Sleep(time.Until(deadline))
			return next.Mutate(ctx, mutation)
		})
	})
	finish(waiting, now.Add(4*time.Second), 6)
	require.Equal(t, runStatusUnknown, waiting.Status)
	require.Equal(t, runStatusUnknown, db.PluginScheduleRun.GetX(ctx, waiting.ID).Status)
	kept, err = readCatalogCheckSummary(ctx, db, installed.ID)
	require.NoError(t, err)
	require.Equal(t, updated, kept)
}
