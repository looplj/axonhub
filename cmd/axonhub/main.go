package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/andreazorzetto/yh/highlight"
	"github.com/hokaccha/go-prettyjson"
	"go.uber.org/fx"
	"gopkg.in/yaml.v3"

	sdk "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/conf"
	"github.com/looplj/axonhub/internal/dumper"
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
		case "help", "--help", "-h":
			showHelp()
			return
		}
	}

	startServer()
}

func startServer() {
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

func handleConfigCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: axonhub config <preview|validate>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "preview":
		configPreview()
	case "validate":
		configValidate()
	default:
		fmt.Println("Usage: axonhub config <preview|validate>")
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

	return errors
}

func showHelp() {
	fmt.Println("AxonHub AI Gateway")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  axonhub                    Start the server (default)")
	fmt.Println("  axonhub config preview     Preview configuration")
	fmt.Println("  axonhub config validate    Validate configuration")
	fmt.Println("  axonhub help               Show this help message")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -f, --format FORMAT       Output format for config preview (yml, json)")
}
