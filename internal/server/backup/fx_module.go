package backup

import (
	"context"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/server/scheduler"
)

var Module = fx.Module("backup",
	fx.Provide(NewBackupService),
	fx.Invoke(func(lc fx.Lifecycle, svc *BackupService, s *scheduler.Scheduler) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return svc.RegisterScheduledTasks(ctx, s)
			},
		})
	}),
)
