package scheduler

import (
	"context"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"
)

var Module = fx.Module("scheduler",
	fx.Provide(func(exec executors.ScheduledExecutor) *Scheduler {
		return New(exec)
	}),
	fx.Invoke(func(lc fx.Lifecycle, s *Scheduler) {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				s.Shutdown(ctx)
				return nil
			},
		})
	}),
)
