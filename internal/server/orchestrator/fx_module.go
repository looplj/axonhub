package orchestrator

import "go.uber.org/fx"

var Module = fx.Module("orchestrator",
	fx.Provide(NewDefaultSelector),
	fx.Provide(NewCandidateSelectorDiagnostics),
)
