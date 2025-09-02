package log

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/tracing"
)

func TestTraceHook(t *testing.T) {
	hook := HookFunc(traceFields)

	// Test with context that has trace ID
	ctx := tracing.WithTraceID(context.Background(), tracing.TraceID("at-test-trace-id"))
	fields := hook.Apply(ctx, "test message")

	assert.Len(t, fields, 1)
	assert.Equal(t, "trace_id", fields[0].Key)
	assert.Equal(t, "at-test-trace-id", fields[0].String)

	// Test with context that doesn't have trace ID
	ctx = context.Background()
	fields = hook.Apply(ctx, "test message")

	assert.Len(t, fields, 0)

	// Test with nil context
	fields = hook.Apply(context.Background(), "test message")

	assert.Len(t, fields, 0)
}
