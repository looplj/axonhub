package log

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/tracing"
)

func TestTraceHook(t *testing.T) {
	hook := HookFunc(traceFields)

	t.Run("with trace ID", func(t *testing.T) {
		ctx := tracing.WithTraceID(context.Background(), "at-test-trace-id")
		fields := hook.Apply(ctx, "test message")
		assert.Len(t, fields, 1)
		assert.Equal(t, "trace_id", fields[0].Key)
		assert.Equal(t, "at-test-trace-id", fields[0].String)
	})

	t.Run("with operation name", func(t *testing.T) {
		ctx := tracing.WithOperationName(context.Background(), "test-operation-name")
		fields := hook.Apply(ctx, "test message")
		assert.Len(t, fields, 1)
		assert.Equal(t, "operation_name", fields[0].Key)
		assert.Equal(t, "test-operation-name", fields[0].String)
	})

	t.Run("with context that has trace ID", func(t *testing.T) {
		ctx := tracing.WithTraceID(context.Background(), "at-test-trace-id")
		fields := hook.Apply(ctx, "test message")
		assert.Len(t, fields, 1)
		assert.Equal(t, "trace_id", fields[0].Key)
		assert.Equal(t, "at-test-trace-id", fields[0].String)
	})

	t.Run("with context that doesn't have trace ID", func(t *testing.T) {
		ctx := context.Background()
		fields := hook.Apply(ctx, "test message")
		assert.Len(t, fields, 0)
	})

	t.Run("with nil context", func(t *testing.T) {
		fields := hook.Apply(nil, "test message")
		assert.Len(t, fields, 0)
	})
}
