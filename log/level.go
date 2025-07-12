package log

import (
	"context"

	"go.uber.org/zap"
)

type forceEnableLogLevelKey struct{}

func ForceEnableLevel(parent context.Context, level Level) context.Context {
	return context.WithValue(parent, forceEnableLogLevelKey{}, level)
}

func ForceEnabledLevelFromContext(ctx context.Context) (Level, bool) {
	value := ctx.Value(forceEnableLogLevelKey{})
	if value == nil {
		return FatalLevel, false
	}
	return value.(Level), true
}

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in
	// production.
	DebugLevel = zap.DebugLevel
	// InfoLevel is the default logging priority.
	InfoLevel = zap.InfoLevel
	// WarnLevel logs are more important than Info, but don't need individual
	// human review.
	WarnLevel = zap.WarnLevel
	// ErrorLevel logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-Level logs.
	ErrorLevel = zap.ErrorLevel
	// PanicLevel logs a message, then panics.
	PanicLevel = zap.PanicLevel
	// FatalLevel logs a message, then calls os.Exit(1).
	FatalLevel = zap.FatalLevel
)

func (l *Logger) DebugEnabled(ctx context.Context) bool {
	enabled := l.config.Level.Enabled(DebugLevel)
	if enabled {
		return true
	}
	return l.levelEnabledFromContext(ctx, DebugLevel)
}

func (l *Logger) levelEnabledFromContext(ctx context.Context, expect Level) bool {
	if ctx == nil {
		return false
	}
	level, ok := ForceEnabledLevelFromContext(ctx)
	if !ok {
		return false
	}
	return level.Enabled(expect)
}

func (l *Logger) InfoEnabled(ctx context.Context) bool {
	enabled := l.config.Level.Enabled(InfoLevel)
	if enabled {
		return true
	}
	return l.levelEnabledFromContext(ctx, InfoLevel)
}

func (l *Logger) WarnEnabled(ctx context.Context) bool {
	enabled := l.config.Level.Enabled(WarnLevel)
	if enabled {
		return true
	}
	return l.levelEnabledFromContext(ctx, WarnLevel)
}

func (l *Logger) ErrorEnabled(ctx context.Context) bool {
	enabled := l.config.Level.Enabled(ErrorLevel)
	if enabled {
		return true
	}
	return l.levelEnabledFromContext(ctx, ErrorLevel)
}

func (l *Logger) PanicEnabled(ctx context.Context) bool {
	enabled := l.config.Level.Enabled(PanicLevel)
	if enabled {
		return true
	}
	return l.levelEnabledFromContext(ctx, PanicLevel)
}

func (l *Logger) FataEnabled(ctx context.Context) bool {
	enabled := l.config.Level.Enabled(FatalLevel)
	if enabled {
		return true
	}
	return l.levelEnabledFromContext(ctx, FatalLevel)
}
