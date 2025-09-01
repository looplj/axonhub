package metrics

// Config specifies the configuration for metrics.
type Config struct {
	// Enabled specifies whether metrics are enabled.
	// Default is false.
	Enabled bool `conf:"enabled"`

	Exporter ExporterConfig `conf:"exporter"`
}

type ExporterConfig struct {
	Type     string `conf:"type" validate:"oneof=stdout otlpgrpc otlphttp"`
	Endpoint string `conf:"endpoint"`
	Insecure bool   `conf:"insecure"`
}
