package dependencies

import (
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

var Module = fx.Module("dependencies",
	fx.Provide(NewEntClient),
	fx.Provide(httpclient.NewHttpClient),
	fx.Provide(NewExecutors),
)
