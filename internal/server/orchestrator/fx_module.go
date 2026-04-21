package orchestrator

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module("orchestrator",
	fx.Provide(NewDefaultSelector),
	fx.Provide(NewCandidateSelectorDiagnostics),
	fx.Provide(NewChannelRequestTracker),
	fx.Provide(NewDefaultConnectionTrackerForFx),
	fx.Provide(NewModelConnectionTracker),
	fx.Provide(NewChannelCostTracker),
	fx.Invoke(HookTrackerLifecycle),
)

func HookTrackerLifecycle(lc fx.Lifecycle, tracker *ChannelRequestTracker, costTracker *ChannelCostTracker) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			tracker.Start()
			costTracker.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			tracker.Stop()
			costTracker.Stop()
			return nil
		},
	})
}

func NewDefaultConnectionTrackerForFx() *DefaultConnectionTracker {
	return NewDefaultConnectionTracker(DefaultMaxConnectionsPerChannel)
}
