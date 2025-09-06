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

	"github.com/looplj/axonhub/internal/dumper"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/server"
	"github.com/looplj/axonhub/internal/server/db"
)

type Config struct {
	fx.Out `yaml:"-" json:"-"`

	DB        db.Config      `conf:"db" yaml:"db" json:"db"`
	Log       log.Config     `conf:"log" yaml:"log" json:"log"`
	APIServer server.Config  `conf:"server" yaml:"server" json:"server"`
	Metrics   metrics.Config `conf:"metrics" yaml:"metrics" json:"metrics"`
	Dumper    dumper.Config  `conf:"dumper" yaml:"dumper" json:"dumper"`
}

// Load loads configuration from YAML file and environment variables.
func Load() (Config, error) {
	v := viper.New()

	// Set config file name and paths
	v.SetConfigName("config")
	v.SetConfigType("yml")
	v.AddConfigPath(".")
	v.AddConfigPath("./conf")
	v.AddConfigPath("/etc/axonhub/")
	v.AddConfigPath("$HOME/.axonhub")

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
		println("Invalid log level, use default log level:", err.Error())
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

	log.Debug(context.Background(), "Config loaded successfully", log.Any("config", config))

	return config, nil
}

var (
	_TypeTextUnmarshaler = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	_TypeDuration        = reflect.TypeOf(time.Duration(1))
)

func customizedDecodeHook(srcType reflect.Type, dstType reflect.Type, data interface{}) (interface{}, error) {
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
		return time.ParseDuration(str)
	default:
		return data, nil
	}
}

// setDefaults sets default configuration values.
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8090)
	v.SetDefault("server.name", "AxonHub")
	v.SetDefault("server.base_path", "")
	v.SetDefault("server.request_timeout", "30s")
	v.SetDefault("server.llm_request_timeout", "300s")
	v.SetDefault("server.trace.trace_header", "AH-Trace-Id")
	v.SetDefault("server.debug", false)

	// Database defaults
	v.SetDefault("db.dialect", "sqlite3")
	v.SetDefault("db.dsn", "file:axonhub.db?cache=shared&_fk=1&journal_mode=WAL")
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

	// Metrics defaults
	v.SetDefault("metrics.enabled", false)

	// Dumper defaults
	v.SetDefault("dumper.enabled", false)
	v.SetDefault("dumper.dump_path", "./dumps")
	v.SetDefault("dumper.max_size", 100)
	v.SetDefault("dumper.max_age", "24h")
	v.SetDefault("dumper.max_backups", 10)
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
