package backup

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module("backup",
	fx.Provide(NewBackupService),
	fx.Invoke(func(lc fx.Lifecycle, svc *BackupService) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return svc.Start(ctx)
			},
			OnStop: func(ctx context.Context) error {
				return svc.Stop(ctx)
			},
		})
	}),
)
