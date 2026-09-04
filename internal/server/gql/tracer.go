package gql

import (
	"context"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/tracing"
)

// sensitiveVariableKeys are variable paths whose values must never reach the
// debug log. Matching is case-insensitive on the final key segment; nested
// maps and slices are walked.
var sensitiveVariableKeys = map[string]struct{}{
	"authcookie":   {},
	"apikey":       {},
	"credentials":  {},
	"accesstoken":  {},
	"refreshtoken": {},
	"password":     {},
	"secret":       {},
}

func redactSensitiveVariables(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			if _, ok := sensitiveVariableKeys[strings.ToLower(k)]; ok {
				out[k] = "<redacted>"
			} else {
				out[k] = redactSensitiveVariables(item)
			}
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = redactSensitiveVariables(item)
		}
		return out
	default:
		return v
	}
}

type loggingTracer struct{}

var _ interface {
	graphql.HandlerExtension
	graphql.ResponseInterceptor
} = &loggingTracer{}

func (t *loggingTracer) ExtensionName() string {
	return "logging_tracer"
}

func (t *loggingTracer) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

func (t *loggingTracer) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	if graphql.HasOperationContext(ctx) {
		opCtx := graphql.GetOperationContext(ctx)
		ctx = tracing.WithOperationName(ctx, opCtx.OperationName)

		if log.DebugEnabled(ctx) {
			// The raw query text is intentionally not logged: GraphQL clients
			// may inline sensitive literals (e.g. a channel quota cookie)
			// instead of passing them as variables, so only the operation name
			// and redacted variables are recorded.
			log.Debug(ctx, "received graphql request",
				zap.String("operation", opCtx.OperationName),
				zap.Any("variables", redactSensitiveVariables(opCtx.Variables)),
			)
		}
	}

	resp := next(ctx)

	// Capture GraphQL errors to context for access logging
	if resp != nil && len(resp.Errors) > 0 {
		for _, gqlErr := range resp.Errors {
			contexts.AddError(ctx, gqlErr)
		}
	}

	return resp
}
