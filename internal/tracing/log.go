package tracing

import (
	"context"

	"github.com/looplj/axonhub/internal/log"
)

func SetupLogger(logger *log.Logger) {
	logger.AddHook(log.HookFunc(TraceFieldsHooks))
}

// TraceFieldsHooks adds trace ID and request ID to log entries if they exist in the context.
func TraceFieldsHooks(ctx context.Context, msg string, fields ...log.Field) []log.Field {
	if ctx == nil {
		return fields
	}

	// Try to get trace ID from context
	if traceID, ok := GetTraceID(ctx); ok {
		// Add trace ID to fields
		fields = append(fields, log.String("trace_id", traceID))
	}

	// Try to get request ID from context
	if requestID, ok := GetRequestID(ctx); ok {
		// Add request ID to fields
		fields = append(fields, log.String("request_id", requestID))
	}

	if operationName, ok := GetOperationName(ctx); ok {
		fields = append(fields, log.String("operation_name", operationName))
	}

	return fields
}
