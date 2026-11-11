package events

import (
	"go.uber.org/fx"
)

// Module provides the process-local event bus.
// OnStop closes it and waits for subscription workers to exit.
var Module = fx.Module("events",
	fx.Provide(NewBus),
	fx.Invoke(func(lc fx.Lifecycle, bus *Bus) {
		lc.Append(fx.Hook{
			OnStop: bus.Close,
		})
	}),
)
