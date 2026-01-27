package backup

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module("backup",
	fx.Provide(NewBackupService),
	fx.Provide(NewWorker),
	fx.Invoke(func(lc fx.Lifecycle, w *Worker) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return w.Start(ctx)
			},
			OnStop: func(ctx context.Context) error {
				return w.Stop(ctx)
			},
		})
	}),
)
