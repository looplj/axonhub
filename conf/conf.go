package conf

import (
	"time"

	"go.uber.org/fx"
)

type Config struct {
	fx.Out
	APIServer Server
}

type Server struct {
	Port           int
	Name           string
	BasePath       string
	RequestTimeout time.Duration
	Debug          bool
	CORS           CORS
}

type CORS struct {
	Debug              bool
	Enabled            bool
	AllowedOrigins     []string
	AllowedMethods     []string
	AllowedHeaders     []string
	ExposedHeaders     []string
	AllowCredentials   bool
	MaxAge             int
	OptionsPassthrough bool
}

func Load() Config {
	return Config{
		APIServer: Server{
			Port:           8090,
			Name:           "AxonHub",
			BasePath:       "",
			RequestTimeout: 0,
			Debug:          false,
			CORS: CORS{
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
