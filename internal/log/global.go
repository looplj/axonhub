package log

import (
	"context"

	"dario.cat/mergo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var defaultConfig = Config{
	Name:      "default",
	Debug:     false,
	SkipLevel: 1,
	Level:     InfoLevel,
	LevelKey:  "level",
	TimeKey:   "time",
	CallerKey: "label",
	NameKey:   "logger",
	Encoding:  "json",
	Includes:  []string{},
	Excludes:  []string{},
}

var globalConfig = Config{
	Name:      "global",
	Debug:     false,
	SkipLevel: 2,
	Level:     InfoLevel,
	LevelKey:  "level",
	TimeKey:   "time",
	CallerKey: "label",
	NameKey:   "logger",
	Encoding:  "json",
	Includes:  []string{},
	Excludes:  []string{},
}

var (
	globalLogger *Logger
	globalHooks  []Hook
)

func init() {
	err := zap.RegisterEncoder(
		"console_json",
		func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
			enc := NewConsoleJSONEncoder(config)
			return enc, nil
		},
	)
	if err != nil {
		panic(err)
	}

	globalLogger = New(globalConfig)
}

func SetGlobalConfig(cfg Config) {
	err := mergo.Merge(&cfg, globalConfig)
	if err != nil {
		panic(err)
	}

	cfg.SkipLevel = 2
	globalLogger = New(cfg)
}

func Get(name string) *Logger {
	return globalLogger.WithName(name)
}

func GetGlobalLogger() *Logger {
	return globalLogger
}

func SetLevel(level Level) {
	globalLogger.SetLevel(level)
}

func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	globalLogger.Debug(ctx, msg, fields...)
}

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	globalLogger.Info(ctx, msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	globalLogger.Warn(ctx, msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	globalLogger.Error(ctx, msg, fields...)
}

func Panic(ctx context.Context, msg string, fields ...zap.Field) {
	globalLogger.Panic(ctx, msg, fields...)
}

func DebugEnabled(ctx context.Context) bool {
	return globalLogger.DebugEnabled(ctx)
}

func InfoEnabled(ctx context.Context) bool {
	return globalLogger.InfoEnabled(ctx)
}

func WarnEnabled(ctx context.Context) bool {
	return globalLogger.WarnEnabled(ctx)
}

func ErrorEnabled(ctx context.Context) bool {
	return globalLogger.ErrorEnabled(ctx)
}

func PanicEnabled(ctx context.Context) bool {
	return globalLogger.PanicEnabled(ctx)
}

func FataEnabled(ctx context.Context) bool {
	return globalLogger.FataEnabled(ctx)
}
