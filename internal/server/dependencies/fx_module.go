package dependencies

import (
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/db"
)

var Module = fx.Module("dependencies",
	fx.Provide(log.New),
	fx.Provide(db.NewEntClient),
	fx.Provide(httpclient.NewHttpClient),
	fx.Provide(NewExecutors),
)
