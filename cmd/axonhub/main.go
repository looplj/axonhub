package main

import (
	"context"
	"os"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/conf"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server"
)

func main() {
	server.Run(
		fx.Provide(conf.Load),
		fx.Invoke(func(lc fx.Lifecycle, server *server.Server, ent *ent.Client) {
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
