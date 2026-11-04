package biz

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/scopes"
)

// DisableAPIKey requires read_api_keys and write_api_keys on a service account.
// Only user keys in its project can be disabled; administrators may re-enable them.
func (s *APIKeyService) DisableAPIKey(ctx context.Context, id int) (*ent.APIKey, error) {
	caller, ok := contexts.GetAPIKey(ctx)
	if !ok || caller == nil || caller.Type != apikey.TypeServiceAccount || caller.Status != apikey.StatusEnabled || caller.DeletedAt != 0 {
		return nil, xerrors.ForbiddenError("an enabled service account is required")
	}
	if err := authz.RequireScope(ctx, scopes.ScopeWriteAPIKeys); err != nil {
		return nil, xerrors.ForbiddenError("write_api_keys scope is required").WithCause(err)
	}
	if err := authz.RequireScope(ctx, scopes.ScopeReadAPIKeys); err != nil {
		return nil, xerrors.ForbiddenError("read_api_keys scope is required").WithCause(err)
	}
	if caller.ID == id {
		return nil, xerrors.ValidationError("cannot disable the calling API key")
	}
	target, err := s.GetForRead(ctx, &id, nil, nil)
	if err != nil {
		if errors.Is(err, privacy.Deny) {
			return nil, xerrors.ForbiddenError("API key access denied").WithCause(err)
		}
		if ent.IsNotFound(err) {
			return nil, xerrors.NotFoundError("api_key").WithCause(err)
		}
		return nil, xerrors.NewCodedError(xerrors.ErrCodeInternalServerError, "failed to find API key in caller project").WithCause(err)
	}
	if target.Type != apikey.TypeUser {
		return nil, xerrors.ValidationError("only user API keys can be disabled through OpenAPI")
	}
	if target.Status == apikey.StatusArchived {
		return nil, xerrors.ValidationError("archived API keys cannot be disabled")
	}
	disabled, err := s.UpdateAPIKeyStatus(ctx, target.ID, apikey.StatusDisabled)
	if err != nil {
		return nil, xerrors.NewCodedError(xerrors.ErrCodeInternalServerError, "failed to disable API key").WithCause(err)
	}
	return disabled, nil
}
