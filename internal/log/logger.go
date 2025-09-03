package log

import (
	"context"

	"dario.cat/mergo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config ...
type Config struct {
	Name        string   `conf:"name" yaml:"name" json:"name"`
	Debug       bool     `conf:"debug" yaml:"debug" json:"debug"`
	SkipLevel   int      `conf:"skip_level" yaml:"skip_level" json:"skip_level"`
	Level       Level    `conf:"level" yaml:"level" json:"level"`
	LevelKey    string   `conf:"level_key" yaml:"level_key" json:"level_key"`
	TimeKey     string   `conf:"time_key" yaml:"time_key" json:"time_key"`
	CallerKey   string   `conf:"caller_key" yaml:"caller_key" json:"caller_key"`
	FunctionKey string   `conf:"function_key" yaml:"function_key" json:"function_key"`
	NameKey     string   `conf:"name_key" yaml:"name_key" json:"name_key"`
	Encoding    string   `conf:"encoding" yaml:"encoding" json:"encoding"`
	Includes    []string `conf:"includes" yaml:"includes" json:"includes"`
	Excludes    []string `conf:"excludes" yaml:"excludes" json:"excludes"`
}

type Logger struct {
	logger *zap.Logger
	config Config
	hooks  []Hook
}

type (
	Field           = zap.Field
	Level           = zapcore.Level
	ObjectEncoder   = zapcore.ObjectEncoder
	ObjectMarshaler = zapcore.ObjectMarshaler
)

var (
	String     = zap.String
	Bool       = zap.Bool
	Strings    = zap.Strings
	ByteString = zap.ByteString
	Float64    = zap.Float64
	Int64      = zap.Int64
	Int32      = zap.Int32
	Int        = zap.Int
	Uint       = zap.Uint
	Uint64     = zap.Uint64
	Duration   = zap.Duration
	Object     = zap.Object
	Namespace  = zap.Namespace
	Reflect    = zap.Reflect
	Stack      = zap.Stack
	Time       = zap.Time
	Skip       = zap.Skip()

	Cause = func(err error) zap.Field {
		return NamedError("error", err)
	}

	NamedError = func(key string, err error) Field {
		if err == nil {
			return Skip
		} else {
			return Any(key, err)
		}
	}

	Any = func(key string, value interface{}) Field {
		if value == nil {
			return Skip
		}
		return zap.Any(key, value)
	}

	EncodeStringSlice = func(lines []string) zapcore.ArrayMarshalerFunc {
		return func(encoder zapcore.ArrayEncoder) error {
			for _, line := range lines {
				encoder.AppendString(line)
			}
			return nil
		}
	}
)

func New(config Config) *Logger {
	err := mergo.Merge(&config, defaultConfig)
	if err != nil {
		panic(err)
	}

	zapConf := zap.Config{
		Level:    zap.NewAtomicLevelAt(zapcore.DebugLevel),
		Encoding: config.Encoding,
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:     "message",
			LevelKey:       config.LevelKey,
			TimeKey:        config.TimeKey,
			NameKey:        config.NameKey,
			CallerKey:      config.CallerKey,
			FunctionKey:    config.FunctionKey,
			StacktraceKey:  "stacktrace",
			SkipLineEnding: false,
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.RFC3339TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	opts := []zap.Option{zap.AddStacktrace(zapcore.DPanicLevel)}
	if config.SkipLevel != 0 {
		opts = append(opts, zap.AddCallerSkip(config.SkipLevel))
	} else {
		opts = append(opts, zap.AddCallerSkip(defaultConfig.SkipLevel))
	}

	if len(config.Includes) > 0 || len(config.Excludes) > 0 {
		opts = append(opts, withNameFilter(config.Includes, config.Excludes))
	}

	zapLogger, err := zapConf.Build(opts...)
	if err != nil {
		panic(err)
	}

	zapLogger = zapLogger.Named(config.Name)

	return &Logger{
		config: config,
		logger: zapLogger,
		hooks:  []Hook{HookFunc(contextFields), HookFunc(traceFields)},
	}
}

func (l *Logger) WithName(name string) *Logger {
	config := l.config
	config.Name = name
	// global 需要特殊处理 skip level
	if l == globalLogger {
		config.SkipLevel--
	}

	logger := New(config)
	logger.hooks = l.hooks

	return logger
}

func (l *Logger) WithFields(fields ...Field) *Logger {
	if len(fields) == 0 {
		return l
	}

	nl := *l
	nl.hooks = append(nl.hooks, &fieldsHook{fields: fields})

	return &nl
}

func (l *Logger) AddHook(hook Hook) {
	l.hooks = append(l.hooks, hook)
}

func (l *Logger) SetLevel(level Level) {
	l.config.Level = level
	*l = *New(l.config)
}

func (l *Logger) executeHooks(ctx context.Context, msg string, fields ...zap.Field) []zap.Field {
	for _, hook := range globalHooks {
		fields = hook.Apply(ctx, msg, fields...)
	}

	for _, hook := range l.hooks {
		fields = hook.Apply(ctx, msg, fields...)
	}

	return fields
}

func (l *Logger) Debug(ctx context.Context, msg string, fields ...zap.Field) {
	if !l.DebugEnabled(ctx) {
		return
	}

	fields = l.executeHooks(ctx, msg, fields...)
	l.logger.Debug(msg, fields...)
}

func (l *Logger) Info(ctx context.Context, msg string, fields ...zap.Field) {
	if !l.InfoEnabled(ctx) {
		return
	}

	fields = l.executeHooks(ctx, msg, fields...)
	l.logger.Info(msg, fields...)
}

func (l *Logger) Warn(ctx context.Context, msg string, fields ...zap.Field) {
	if !l.WarnEnabled(ctx) {
		return
	}

	fields = l.executeHooks(ctx, msg, fields...)
	l.logger.Warn(msg, fields...)
}

func (l *Logger) Error(ctx context.Context, msg string, fields ...zap.Field) {
	if !l.ErrorEnabled(ctx) {
		return
	}

	fields = l.executeHooks(ctx, msg, fields...)
	l.logger.Error(msg, fields...)
}

func (l *Logger) Panic(ctx context.Context, msg string, fields ...zap.Field) {
	if !l.PanicEnabled(ctx) {
		return
	}

	fields = l.executeHooks(ctx, msg, fields...)
	l.logger.Panic(msg, fields...)
}
