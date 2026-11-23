package plugin

import (
	"context"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/server/scheduler"
)

// Module provides the process-local plugin manager.
// OnStart loads installed plugins. OnStop waits for in-flight acquires and
// removes this process's temp directory.
var Module = fx.Module("plugin",
	fx.Provide(NewManager),
	fx.Invoke(func(lc fx.Lifecycle, manager *Manager, s *scheduler.Scheduler) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				if err := manager.Start(ctx); err != nil {
					return err
				}
				if err := s.Register(ctx, scheduler.TaskSpec{Name: tableSchedulerName, FixRate: scheduleTick}, manager.scanTablePurges); err != nil {
					return err
				}
				return manager.registerTaskScheduler(ctx, s)
			},
			OnStop: func(ctx context.Context) error {
				s.Unregister(taskSchedulerName)
				s.Unregister(tableSchedulerName)
				return manager.Stop(ctx)
			},
		})
	}),
)
