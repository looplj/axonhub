package log

import (
	"context"
)

type Hook interface {
	Apply(ctx context.Context, msg string, fields ...Field) []Field
}

type HookFunc func(context.Context, string, ...Field) []Field

func (h HookFunc) Apply(ctx context.Context, msg string, fields ...Field) []Field {
	return h(ctx, msg, fields...)
}

type fieldsHook struct {
	fields []Field
}

func (f *fieldsHook) Apply(ctx context.Context, msg string, fields ...Field) []Field {
	return append(f.fields, fields...)
}

func DefaultPreHook(ctx context.Context, msg string, fields ...Field) []Field {
	if ctx == nil {
		return nil
	}

	if ctx.Err() != nil {
		fields = append(fields, NamedError("context_error", ctx.Err()))
	}

	if ts, ok := ctx.Deadline(); ok {
		fields = append(fields, Time("context_deadline", ts))
	}

	return fields
}
