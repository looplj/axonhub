package backup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

// TestBackupService_RegisterScheduledTasks_TimezoneFromSettings verifies that
// registering the backup task at startup (fx OnStart context has no user) still
// reads the configured timezone through the system bypass, instead of falling
// back to UTC when the ent privacy policy denies the read.
func TestBackupService_RegisterScheduledTasks_TimezoneFromSettings(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	sysSvc := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	require.NoError(t, sysSvc.SetGeneralSettings(ctx, biz.SystemGeneralSettings{
		CurrencyCode: "USD",
		Timezone:     "Asia/Hong_Kong",
	}))

	svc := &BackupService{
		db:            client,
		systemService: sysSvc,
	}

	exec := executors.NewPoolScheduleExecutor()
	sched := scheduler.New(exec)
	defer sched.Shutdown(context.Background())
	defer exec.Shutdown(context.Background())

	// Simulate startup registration: the context has no user; before the fix,
	// TimeLocation would be denied by privacy and fall back to UTC.
	require.NoError(t, svc.RegisterScheduledTasks(context.Background(), sched))

	tasks := sched.List()
	require.Len(t, tasks, 1)
	require.Equal(t, "backup", tasks[0].Spec.Name)
	require.Equal(t, "Asia/Hong_Kong", tasks[0].Spec.Timezone)
}
