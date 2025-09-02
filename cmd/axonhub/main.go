package main

import (
	"context"
	"os"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/conf"
	"github.com/looplj/axonhub/internal/dumper"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/server"
	sdk "go.opentelemetry.io/otel/sdk/metric"
)

func main() {
	server.Run(
		fx.Provide(conf.Load),
		fx.Provide(metrics.NewProvider),
		fx.Provide(dumper.New),
		fx.Invoke(dumper.SetGlobal),
		fx.Invoke(func(lc fx.Lifecycle, server *server.Server, provider *sdk.MeterProvider, ent *ent.Client) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return metrics.SetupMetrics(provider, server.Config.Name)
				},
				OnStop: func(ctx context.Context) error {
					return provider.Shutdown(ctx)
				},
			})
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go func() {
						err := server.Run()
						if err != nil {
							log.Error(context.Background(), "server run error:", log.Cause(err))
							os.Exit(1)
						}
					}()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					err := server.Shutdown(ctx)
					if err != nil {
						log.Error(context.Background(), "server shutdown error:", log.Cause(err))
					}
					err = ent.Close()
					if err != nil {
						log.Error(context.Background(), "ent close error:", log.Cause(err))
					}
					return nil
				},
			})
		}),
	)
}
