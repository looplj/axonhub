package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/andreazorzetto/yh/highlight"
	"github.com/hokaccha/go-prettyjson"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"gopkg.in/yaml.v3"

	sdk "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/conf"
	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/server"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			handleConfigCommand()
			return
		case "version", "--version", "-v":
			showVersion()
			return
		case "help", "--help", "-h":
			showHelp()
		case "build-info":
			showBuildInfo()
			return
		}
	}

	startServer()
}

func showBuildInfo() {
	fmt.Println(build.GetBuildInfo())
}

type logger struct{}

func (l *logger) LogEvent(event fxevent.Event) {
	log.Debug(context.Background(), "fx event", log.Any("event", event))
}

func startServer() {
	server.Run(
		fx.WithLogger(func() fxevent.Logger {
			return &logger{}
		}),
		fx.Provide(conf.Load),
		fx.Provide(metrics.NewProvider),
		fx.Invoke(func(lc fx.Lifecycle, server *server.Server, provider *sdk.MeterProvider, ent *ent.Client) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if provider != nil {
						return metrics.SetupMetrics(provider, server.Config.Name)
					}

					return nil
				},
				OnStop: func(ctx context.Context) error {
					if provider != nil {
						return provider.Shutdown(ctx)
					}

					return nil
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

func handleConfigCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: axonhub config <preview|validate|get>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "preview":
		configPreview()
	case "validate":
		configValidate()
	case "get":
		configGet()
	default:
		fmt.Println("Usage: axonhub config <preview|validate|get>")
		os.Exit(1)
	}
}

func configPreview() {
	format := "yml"

	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--format" || os.Args[i] == "-f" {
			if i+1 < len(os.Args) {
				format = os.Args[i+1]
			}
		}
	}

	config, err := conf.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	var output string

	switch format {
	case "json":
		b, err := prettyjson.Marshal(config)
		if err != nil {
			fmt.Printf("Failed to preview config: %v\n", err)
			os.Exit(1)
		}

		output = string(b)
	case "yml", "yaml":
		b, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Failed to preview config: %v\n", err)
			os.Exit(1)
		}

		output, err = highlight.Highlight(bytes.NewBuffer(b))
		if err != nil {
			fmt.Printf("Failed to preview config: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unsupported format: %s\n", format)
		os.Exit(1)
	}

	fmt.Println(output)
}

func configValidate() {
	config, err := conf.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	errors := validateConfig(config)

	if len(errors) == 0 {
		fmt.Println("Configuration is valid!")
		return
	}

	fmt.Println("Configuration validation failed:")

	for _, err := range errors {
		fmt.Printf("  - %s\n", err)
	}

	os.Exit(1)
}

func validateConfig(config conf.Config) []string {
	var errors []string

	if config.APIServer.Port <= 0 || config.APIServer.Port > 65535 {
		errors = append(errors, "server.port must be between 1 and 65535")
	}

	if config.DB.DSN == "" {
		errors = append(errors, "db.dsn cannot be empty")
	}

	if config.Log.Name == "" {
		errors = append(errors, "log.name cannot be empty")
	}

	if config.APIServer.CORS.Enabled && len(config.APIServer.CORS.AllowedOrigins) == 0 {
		errors = append(errors, "server.cors.allowed_origins cannot be empty when CORS is enabled")
	}

	return errors
}

func configGet() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: axonhub config get <key>")
		fmt.Println("")
		fmt.Println("Available keys:")
		fmt.Println("  server.port    Server port number")
		fmt.Println("  server.name    Server name")
		fmt.Println("  db.dialect     Database dialect")
		fmt.Println("  db.dsn         Database DSN")
		os.Exit(1)
	}

	key := os.Args[3]

	config, err := conf.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	var value interface{}

	switch key {
	case "server.port":
		value = config.APIServer.Port
	case "server.name":
		value = config.APIServer.Name
	case "server.base_path":
		value = config.APIServer.BasePath
	case "server.debug":
		value = config.APIServer.Debug
	case "db.dialect":
		value = config.DB.Dialect
	case "db.dsn":
		value = config.DB.DSN
	default:
		fmt.Fprintf(os.Stderr, "Unknown config key: %s\n", key)
		os.Exit(1)
	}

	fmt.Println(value)
}

func showHelp() {
	fmt.Println("AxonHub AI Gateway")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  axonhub                    Start the server (default)")
	fmt.Println("  axonhub config preview     Preview configuration")
	fmt.Println("  axonhub config validate    Validate configuration")
	fmt.Println("  axonhub config get <key>   Get a specific config value")
	fmt.Println("  axonhub version            Show version")
	fmt.Println("  axonhub help               Show this help message")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -f, --format FORMAT       Output format for config preview (yml, json)")
}

func showVersion() {
	fmt.Println(build.Version)
}
