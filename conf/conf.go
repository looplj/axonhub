package conf

import (
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/server"
)

type Config struct {
	fx.Out
	APIServer server.Config
}

func Load() Config {
	return Config{
		APIServer: server.Config{
			Port:           8090,
			Name:           "AxonHub",
			BasePath:       "",
			RequestTimeout: 0,
			Debug:          false,
			CORS: server.CORS{
				Debug:   false,
				Enabled: true,
				AllowedOrigins: []string{
					"http://localhost:3000",
					"http://localhost:5173",
				},
				AllowedMethods:     []string{"GET", "POST", "DELETE", "PATCH", "PUT", "OPTIONS", "HEAD"},
				AllowedHeaders:     []string{"*"},
				ExposedHeaders:     nil,
				AllowCredentials:   true,
				MaxAge:             0,
				OptionsPassthrough: false,
			},
		},
	}
}
