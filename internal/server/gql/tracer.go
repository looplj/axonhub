package gql

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/tracing"
)

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
			log.Debug(ctx, "received graphql request",
				zap.Any("raw", opCtx.RawQuery),
				zap.Any("variables", opCtx.Variables),
			)
		}
	}

	return next(ctx)
}
