package openapi

import (
	"context"
	"errors"
	"maps"

	"github.com/99designs/gqlgen/graphql"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func presentError(ctx context.Context, err error) *gqlerror.Error {
	result := graphql.DefaultErrorPresenter(ctx, err)
	if coded, ok := xerrors.IsCodedError(err); ok {
		result.Message = coded.Message
		result.Extensions = maps.Clone(coded.Extensions)
		if result.Extensions == nil {
			result.Extensions = make(map[string]any)
		}
		result.Extensions["code"] = coded.Code
		return result
	}
	var code xerrors.ErrorCode
	switch {
	case errors.Is(err, privacy.Deny):
		code = xerrors.ErrCodeForbidden
	case ent.IsNotFound(err):
		code = xerrors.ErrCodeNotFound
	default:
		return result
	}
	if result.Extensions == nil {
		result.Extensions = make(map[string]any)
	}
	result.Extensions["code"] = code
	return result
}
