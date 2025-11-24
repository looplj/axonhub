package dumper

import "go.uber.org/fx"

// Module is the fx module for the dumper.
func Module() fx.Option {
	return fx.Options(
		fx.Provide(New),
	)
}
