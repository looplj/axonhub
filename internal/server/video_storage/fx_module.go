package video_storage

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module("video_storage",
	fx.Provide(NewWorker),
	fx.Invoke(func(lc fx.Lifecycle, worker *Worker) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return worker.Start(ctx)
			},
			OnStop: func(ctx context.Context) error {
				return worker.Stop(ctx)
			},
		})
	}),
)
