package conf

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"go.uber.org/zap/zapcore"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server"
	"github.com/looplj/axonhub/internal/server/db"
	"github.com/looplj/axonhub/internal/server/gc"
)

type Config struct {
	fx.Out `yaml:"-" json:"-"`

	DB                                   db.Config                `conf:"db" yaml:"db" json:"db"`
	Log                                  log.Config               `conf:"log" yaml:"log" json:"log"`
	APIServer                            server.Config            `conf:"server" yaml:"server" json:"server"`
	Metrics                              metrics.Config           `conf:"metrics" yaml:"metrics" json:"metrics"`
	GC                                   gc.Config                `conf:"gc" yaml:"gc" json:"gc"`
	Cache                                xcache.Config            `conf:"cache" yaml:"cache" json:"cache"`
	ProviderQuota                        providerQuotaConfig      `conf:"provider_quota" yaml:"provider_quota" json:"provider_quota"`
	Performance                          server.PerformanceConfig `conf:"performance" yaml:"performance" json:"performance"`
	DisableSSLVerify                     bool                     `name:"disable_ssl_verify" yaml:"-" json:"-"`
	AllowNoAuth                          bool                     `name:"allow_no_auth" yaml:"-" json:"-"`
	PerformanceHistoricalRefreshInterval time.Duration            `name:"performance_historical_refresh_interval" yaml:"-" json:"-"`
	HistoricalWeight                     float64                  `name:"historical_weight" yaml:"-" json:"-"`
	RealtimeWeight                       float64                  `name:"realtime_weight" yaml:"-" json:"-"`
	HistoricalWindow                     time.Duration            `name:"historical_window" yaml:"-" json:"-"`
}

type providerQuotaConfig struct {
	CheckInterval time.Duration `conf:"check_interval" yaml:"check_interval" json:"check_interval"`
}

// Load loads configuration from YAML file and environment variables.
func Load() (Config, error) {
	v := viper.New()

	// Set config file name and paths
	v.SetConfigName("config")
	v.SetConfigType("yml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/axonhub/")
	v.AddConfigPath("$HOME/.config/axonhub/")
	v.AddConfigPath("./conf")

	// Enable environment variable support
	v.AutomaticEnv()
	v.SetEnvPrefix("AXONHUB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set default values
	setDefaults(v)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return Config{}, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, use defaults and environment variables
	}

	// Parse log level from string before unmarshaling
	logLevelStr := v.GetString("log.level")

	logLevel, err := parseLogLevel(logLevelStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid log level '%s': %w", logLevelStr, err)
	}
	// Set the parsed log level back to viper for unmarshaling
	v.Set("log.level", int(logLevel))

	// Unmarshal config
	var config Config
	if err := v.Unmarshal(&config, func(dc *mapstructure.DecoderConfig) {
		dc.DecodeHook = customizedDecodeHook
		dc.TagName = "conf"
	}); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	config.DisableSSLVerify = config.APIServer.DisableSSLVerify
	config.AllowNoAuth = config.APIServer.API.Auth.AllowNoAuth
	config.PerformanceHistoricalRefreshInterval = config.Performance.HistoricalRefreshInterval
	config.HistoricalWeight = config.Performance.HistoricalWeight
	config.RealtimeWeight = config.Performance.RealtimeWeight
	config.HistoricalWindow = config.Performance.HistoricalWindow
	if config.HistoricalWindow <= 0 {
		config.HistoricalWindow = 7 * 24 * time.Hour // Default to 7 days
		config.Performance.HistoricalWindow = config.HistoricalWindow // Sync back to Performance struct
	}

	if err := validatePerformanceConfig(config.Performance); err != nil {
		return Config{}, fmt.Errorf("invalid performance config: %w", err)
	}

	log.Debug(context.Background(), "Config loaded successfully", log.Any("config", config))

	return config, nil
}

var (
	_TypeTextUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()
	_TypeDuration        = reflect.TypeFor[time.Duration]()
)

func customizedDecodeHook(srcType reflect.Type, dstType reflect.Type, data any) (any, error) {
	str, ok := data.(string)
	if !ok {
		return data, nil
	}

	switch {
	case reflect.PointerTo(dstType).Implements(_TypeTextUnmarshaler):
		value := reflect.New(dstType)

		u, _ := value.Interface().(encoding.TextUnmarshaler)
		if err := u.UnmarshalText([]byte(str)); err != nil {
			return nil, err
		}

		return u, nil
	case dstType == _TypeDuration:
		if strings.TrimSpace(str) == "" {
			return time.Duration(0), nil
		}
		return time.ParseDuration(str)
	default:
		return data, nil
	}
}

// setDefaults sets default configuration values.
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8090)
	v.SetDefault("server.name", "AxonHub")
	v.SetDefault("server.base_path", "")
	v.SetDefault("server.request_timeout", "30s")
	v.SetDefault("server.llm_request_timeout", "600s")
	v.SetDefault("server.trace.thread_header", "AH-Thread-Id")
	v.SetDefault("server.trace.trace_header", "AH-Trace-Id")
	v.SetDefault("server.trace.extra_trace_headers", []string{})
	v.SetDefault("server.trace.extra_trace_body_fields", []string{})
	v.SetDefault("server.trace.claude_code_trace_enabled", false)
	v.SetDefault("server.trace.codex_trace_enabled", false)

	// Dashboard defaults
	v.SetDefault("server.dashboard.all_time_token_stats_soft_ttl", "1h")
	v.SetDefault("server.dashboard.all_time_token_stats_hard_ttl", "24h")

	v.SetDefault("server.debug", false)
	v.SetDefault("server.disable_ssl_verify", false)

	// CORS defaults
	v.SetDefault("server.cors.enabled", false)
	v.SetDefault("server.cors.debug", false)
	v.SetDefault("server.cors.allowed_origins", []string{"http://localhost:8090"})
	v.SetDefault("server.cors.allowed_methods", []string{"GET", "POST", "DELETE", "PATCH", "PUT", "OPTIONS", "HEAD"})
	v.SetDefault("server.cors.allowed_headers", []string{"Content-Type", "Authorization", "X-API-Key", "X-Goog-Api-Key", "X-Project-ID", "X-Thread-ID", "X-Trace-ID"})
	v.SetDefault("server.cors.exposed_headers", []string{})
	v.SetDefault("server.cors.allow_credentials", false)
	v.SetDefault("server.cors.max_age", "30m")
	v.SetDefault("server.api.auth.allow_no_auth", false)

	// Database defaults
	v.SetDefault("db.dialect", "sqlite3")
	v.SetDefault("db.dsn", "file:axonhub.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)")
	v.SetDefault("db.debug", false)

	// Log defaults
	v.SetDefault("log.name", "axonhub")
	v.SetDefault("log.debug", false)
	v.SetDefault("log.skip_level", 1)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.level_key", "level")
	v.SetDefault("log.time_key", "time")
	v.SetDefault("log.caller_key", "label")
	v.SetDefault("log.function_key", "")
	v.SetDefault("log.name_key", "logger")
	v.SetDefault("log.encoding", "json")
	v.SetDefault("log.includes", []string{})
	v.SetDefault("log.excludes", []string{})
	v.SetDefault("log.output", "stdio")
	v.SetDefault("log.file.path", "logs/axonhub.log")
	v.SetDefault("log.file.max_size", 100)   // MB
	v.SetDefault("log.file.max_age", 30)     // days
	v.SetDefault("log.file.max_backups", 10) // files
	v.SetDefault("log.file.local_time", true)

	// Metrics defaults
	v.SetDefault("metrics.enabled", false)

	// GC defaults
	v.SetDefault("gc.cron", "0 2 * * *") // Daily at 2:00 AM
	v.SetDefault("gc.vacuum_enabled", false)
	v.SetDefault("gc.vacuum_full", false)

	// Provider quota defaults
	v.SetDefault("provider_quota.check_interval", "20m") // Check every 20 minutes

	// Cache defaults
	v.SetDefault("cache.mode", "memory")
	v.SetDefault("cache.default_expiration", "5m")
	v.SetDefault("cache.cleanup_interval", "10m")
	v.SetDefault("cache.redis.addr", "")
	v.SetDefault("cache.redis.url", "")
	v.SetDefault("cache.redis.username", "")
	v.SetDefault("cache.redis.password", "")
	// Note: cache.redis.db has no default value to allow explicit override to 0
	v.SetDefault("cache.redis.tls", false)
	v.SetDefault("cache.redis.tls_insecure_skip_verify", false)

	// Performance defaults
	v.SetDefault("performance.historical_window", "168h")         // 7 days
	v.SetDefault("performance.historical_refresh_interval", "2h") // 2 hours
	v.SetDefault("performance.historical_weight", 0.4)            // 40% weight
	v.SetDefault("performance.realtime_weight", 0.6)              // 60% weight
}

// parseLogLevel converts a string log level to zapcore.Level.
func parseLogLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "panic":
		return zapcore.PanicLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

// validatePerformanceConfig validates the PerformanceConfig values.
func validatePerformanceConfig(cfg server.PerformanceConfig) error {
	const weightTolerance = 0.001

	// Validate HistoricalWindow > 0
	if cfg.HistoricalWindow <= 0 {
		return fmt.Errorf("historical_window must be greater than 0, got %v", cfg.HistoricalWindow)
	}

	// HistoricalRefreshInterval can be <= 0 to disable periodic refresh
	// No validation error for non-positive values - they disable the feature

	// Validate HistoricalWeight in [0, 1]
	if cfg.HistoricalWeight < 0 || cfg.HistoricalWeight > 1 {
		return fmt.Errorf("historical_weight must be between 0 and 1, got %f", cfg.HistoricalWeight)
	}

	// Validate RealtimeWeight in [0, 1]
	if cfg.RealtimeWeight < 0 || cfg.RealtimeWeight > 1 {
		return fmt.Errorf("realtime_weight must be between 0 and 1, got %f", cfg.RealtimeWeight)
	}

	// Validate weights sum to 1.0 (within tolerance)
	weightSum := cfg.HistoricalWeight + cfg.RealtimeWeight
	if weightSum < 1.0-weightTolerance || weightSum > 1.0+weightTolerance {
		return fmt.Errorf("weights must sum to 1.0, got %f (historical: %f, realtime: %f)",
			weightSum, cfg.HistoricalWeight, cfg.RealtimeWeight)
	}

	return nil
}
