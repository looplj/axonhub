package plugin

import (
	"go.uber.org/fx"
)

// Module provides the process-local plugin manager.
// OnStart loads installed plugins. OnStop waits for in-flight acquires and
// removes this process's temp directory.
var Module = fx.Module("plugin",
	fx.Provide(NewManager),
	fx.Invoke(func(lc fx.Lifecycle, manager *Manager) {
		lc.Append(fx.Hook{
			OnStart: manager.Start,
			OnStop:  manager.Stop,
		})
	}),
)
