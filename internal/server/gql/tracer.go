package gql

import (
	"context"

	"github.com/99designs/gqlgen/graphql"

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
	}

	return next(ctx)
}
