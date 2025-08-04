package dependencies

import (
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"go.uber.org/fx"
)

var Module = fx.Module("dependencies",
	fx.Provide(NewEntClient),
	fx.Provide(httpclient.NewHttpClient),
	fx.Provide(NewExecutors),
)
