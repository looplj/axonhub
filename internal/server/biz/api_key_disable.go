package biz

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/scopes"
)

// DisableAPIKey requires read_api_keys and write_api_keys on a service account.
// Only user keys in its project can be disabled; administrators may re-enable them.
func (s *APIKeyService) DisableAPIKey(ctx context.Context, id int) (*ent.APIKey, error) {
	caller, ok := contexts.GetAPIKey(ctx)
	if !ok || caller == nil || caller.Type != apikey.TypeServiceAccount || caller.Status != apikey.StatusEnabled || caller.DeletedAt != 0 {
		return nil, fmt.Errorf("an enabled service account is required")
	}
	if err := authz.RequireScope(ctx, scopes.ScopeWriteAPIKeys); err != nil {
		return nil, err
	}
	if caller.ID == id {
		return nil, fmt.Errorf("cannot disable the calling API key")
	}
	target, err := s.GetForRead(ctx, &id, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find API key in caller project: %w", err)
	}
	if target.Type != apikey.TypeUser {
		return nil, fmt.Errorf("only user API keys can be disabled through OpenAPI")
	}
	return s.UpdateAPIKeyStatus(ctx, target.ID, apikey.StatusDisabled)
}
