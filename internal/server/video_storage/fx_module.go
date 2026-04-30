package video_storage

import (
	"context"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/server/scheduler"
)

var Module = fx.Module("video_storage",
	fx.Provide(NewWorker),
	fx.Invoke(func(lc fx.Lifecycle, worker *Worker, s *scheduler.Scheduler) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return worker.RegisterScheduledTasks(ctx, s)
			},
		})
	}),
)
