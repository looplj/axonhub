package dependencies

import (
	"context"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/db"
	"github.com/looplj/axonhub/llm/httpclient"
)

var Module = fx.Module("dependencies",
	fx.Provide(log.New),
	fx.Provide(db.NewEntClient),
	fx.Provide(httpclient.NewHttpClient),
	fx.Provide(NewExecutors),
	fx.Invoke(func(lc fx.Lifecycle, executor executors.ScheduledExecutor) {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return executor.Shutdown(ctx)
			},
		})
	}),
)
