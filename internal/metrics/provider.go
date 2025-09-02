package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"

	metric "go.opentelemetry.io/otel/metric"
	sdk "go.opentelemetry.io/otel/sdk/metric"
)

// Meter is the global meter for the application.
var Meter metric.Meter

// NewProvider initializes the OpenTelemetry metrics provider.
func NewProvider(config Config) (*sdk.MeterProvider, error) {
	ctx := context.Background()
	// If metrics are disabled, create a no-op meter provider
	if !config.Enabled {
		meterProvider := sdk.NewMeterProvider()
		otel.SetMeterProvider(meterProvider)
		Meter = meterProvider.Meter("axonhub")

		return meterProvider, nil
	}

	var (
		exporter sdk.Exporter
		err      error
	)

	switch config.Exporter.Type {
	case "stdout":
		exporter, err = stdoutmetric.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	case "otlpgrpc":
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(config.Exporter.Endpoint),
		}
		if config.Exporter.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}

		exporter, err = otlpmetricgrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create otlpgrpc exporter: %w", err)
		}
	case "otlphttp":
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(config.Exporter.Endpoint),
		}
		if config.Exporter.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}

		exporter, err = otlpmetrichttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create otlphttp exporter: %w", err)
		}
	}

	// Create meter provider
	meterProvider := sdk.NewMeterProvider(
		sdk.WithReader(sdk.NewPeriodicReader(exporter, sdk.WithInterval(5*time.Second))),
	)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Create meter
	Meter = meterProvider.Meter("axonhub")

	return meterProvider, nil
}
