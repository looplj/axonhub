package log

import (
	"context"
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (l *Logger) AsSlog() *slog.Logger {
	return slog.New(&slogHandler{
		logger: l,
		attrs:  []slog.Attr{},
	})
}

type slogHandler struct {
	logger *Logger
	attrs  []slog.Attr
	groups []string
}

func (h *slogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	var zapLevel zapcore.Level

	switch level {
	case slog.LevelDebug:
		zapLevel = DebugLevel
	case slog.LevelInfo:
		zapLevel = InfoLevel
	case slog.LevelWarn:
		zapLevel = WarnLevel
	case slog.LevelError:
		zapLevel = ErrorLevel
	default:
		if level < slog.LevelInfo {
			zapLevel = DebugLevel
		} else if level < slog.LevelWarn {
			zapLevel = InfoLevel
		} else if level < slog.LevelError {
			zapLevel = WarnLevel
		} else {
			zapLevel = ErrorLevel
		}
	}

	return h.logger.config.Level.Enabled(zapLevel) || h.logger.levelEnabledFromContext(ctx, zapLevel)
}

func (h *slogHandler) Handle(ctx context.Context, record slog.Record) error {
	var fields []zap.Field

	// Add attributes from handler
	for _, attr := range h.attrs {
		fields = append(fields, h.slogAttrToZapField(attr))
	}

	// Add attributes from record
	record.Attrs(func(attr slog.Attr) bool {
		fields = append(fields, h.slogAttrToZapField(attr))
		return true
	})

	// Add time field if not already present
	timeFound := false

	for _, field := range fields {
		if field.Key == "time" {
			timeFound = true
			break
		}
	}

	if !timeFound {
		fields = append(fields, zap.Time("time", record.Time))
	}

	// Add level field if not already present
	levelFound := false

	for _, field := range fields {
		if field.Key == "level" {
			levelFound = true
			break
		}
	}

	if !levelFound {
		fields = append(fields, zap.String("level", record.Level.String()))
	}

	// Add message field
	fields = append(fields, zap.String("message", record.Message))

	// Add PC (program counter) field if available
	if record.PC != 0 {
		fields = append(fields, zap.String("caller", "unknown"))
	}

	// Log based on level
	switch record.Level {
	case slog.LevelDebug:
		h.logger.Debug(ctx, record.Message, fields...)
	case slog.LevelInfo:
		h.logger.Info(ctx, record.Message, fields...)
	case slog.LevelWarn:
		h.logger.Warn(ctx, record.Message, fields...)
	case slog.LevelError:
		h.logger.Error(ctx, record.Message, fields...)
	default:
		if record.Level < slog.LevelInfo {
			h.logger.Debug(ctx, record.Message, fields...)
		} else if record.Level < slog.LevelWarn {
			h.logger.Info(ctx, record.Message, fields...)
		} else if record.Level < slog.LevelError {
			h.logger.Warn(ctx, record.Message, fields...)
		} else {
			h.logger.Error(ctx, record.Message, fields...)
		}
	}

	return nil
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &slogHandler{
		logger: h.logger,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &slogHandler{
		logger: h.logger,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

func (h *slogHandler) slogAttrToZapField(attr slog.Attr) zap.Field {
	key := attr.Key

	if len(h.groups) > 0 {
		for _, group := range h.groups {
			key = group + "." + key
		}
	}

	switch attr.Value.Kind() {
	case slog.KindString:
		return zap.String(key, attr.Value.String())
	case slog.KindInt64:
		return zap.Int64(key, attr.Value.Int64())
	case slog.KindUint64:
		return zap.Uint64(key, attr.Value.Uint64())
	case slog.KindFloat64:
		return zap.Float64(key, attr.Value.Float64())
	case slog.KindBool:
		return zap.Bool(key, attr.Value.Bool())
	case slog.KindTime:
		return zap.Time(key, attr.Value.Time())
	case slog.KindDuration:
		return zap.Duration(key, attr.Value.Duration())
	case slog.KindGroup:
		var fields []zap.Field

		groupAttrs := attr.Value.Group()
		for _, a := range groupAttrs {
			fields = append(fields, h.slogAttrToZapField(a))
		}

		return zap.Object(key, zapcore.ObjectMarshalerFunc(func(enc zapcore.ObjectEncoder) error {
			for _, field := range fields {
				field.AddTo(enc)
			}

			return nil
		}))
	case slog.KindAny, slog.KindLogValuer:
		return zap.Any(key, attr.Value.Any())
	}

	return zap.Any(key, attr.Value.Any())
}
