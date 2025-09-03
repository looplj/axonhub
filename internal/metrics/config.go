package metrics

// Config specifies the configuration for metrics.
type Config struct {
	// Enabled specifies whether metrics are enabled.
	// Default is false.
	Enabled bool `conf:"enabled" yaml:"enabled" json:"enabled"`

	Exporter ExporterConfig `conf:"exporter" yaml:"exporter" json:"exporter"`
}

type ExporterConfig struct {
	Type     string `conf:"type" validate:"oneof=stdout otlpgrpc otlphttp" yaml:"type" json:"type"`
	Endpoint string `conf:"endpoint" yaml:"endpoint" json:"endpoint"`
	Insecure bool   `conf:"insecure" yaml:"insecure" json:"insecure"`
}
