package dependencies

import (
	"go.uber.org/fx"
)

var Module = fx.Module("dependencies",
	fx.Provide(NewEntClient),
)
