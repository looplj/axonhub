package orchestrator

import "go.uber.org/fx"

var Module = fx.Module("orchestrator",
	fx.Provide(NewDefaultSelector),
	fx.Provide(NewCandidateSelectorDiagnostics),
	fx.Provide(NewChannelRequestTracker),
	fx.Provide(NewDefaultConnectionTrackerForFx),
	fx.Provide(NewModelConnectionTracker),
	fx.Provide(NewChannelCostTracker),
)

func NewDefaultConnectionTrackerForFx() *DefaultConnectionTracker {
	return NewDefaultConnectionTracker(defaultMaxConnectionsPerChannel)
}
