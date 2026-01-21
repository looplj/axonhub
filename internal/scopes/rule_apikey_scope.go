package scopes

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// APIKeyScopeQueryRule checks API Key permissions for queries.
func APIKeyScopeQueryRule(requiredScope ScopeSlug) privacy.QueryRule {
	return apiKeyQueryRule{requiredScope: requiredScope}
}

// apiKeyQueryRule custom QueryRule implementation for checking API Key scopes.
type apiKeyQueryRule struct {
	requiredScope ScopeSlug
}

func (r apiKeyQueryRule) EvalQuery(ctx context.Context, q ent.Query) error {
	apiKey, err := getAPIKeyFromContext(ctx)
	if err != nil {
		return err
	}

	if hasScope(apiKey.Scopes, string(r.requiredScope)) {
		return privacy.Allow
	}

	return privacy.Denyf("API key does not have required scope: %s", r.requiredScope)
}

// APIKeyScopeMutationRule checks API Key write permissions.
func APIKeyScopeMutationRule(requiredScope ScopeSlug) privacy.MutationRule {
	return privacy.MutationRuleFunc(func(ctx context.Context, m ent.Mutation) error {
		apiKey, err := getAPIKeyFromContext(ctx)
		if err != nil {
			return err
		}

		if hasScope(apiKey.Scopes, string(requiredScope)) {
			return privacy.Allow
		}

		return privacy.Denyf("API key does not have required scope: %s", requiredScope)
	})
}

// APIKeyProjectScopeWriteRule checks API key scope and project ownership for mutations.
func APIKeyProjectScopeWriteRule(requiredScope ScopeSlug) privacy.MutationRule {
	return apiKeyProjectMutationRule{requiredScope: requiredScope}
}

type apiKeyProjectMutationRule struct {
	requiredScope ScopeSlug
}

func (r apiKeyProjectMutationRule) EvalMutation(ctx context.Context, m ent.Mutation) error {
	apiKey, err := getAPIKeyFromContext(ctx)
	if err != nil {
		return err
	}

	if !hasScope(apiKey.Scopes, string(r.requiredScope)) {
		return privacy.Denyf("API key does not have required scope: %s", r.requiredScope)
	}

	switch mutation := m.(type) {
	case ProjectOwnedMutation:
		switch mutation.Op() {
		case ent.OpCreate:
			mProjectID, ok := mutation.ProjectID()
			if !ok {
				return privacy.Denyf("Project ID not found")
			}

			if mProjectID != apiKey.ProjectID {
				return privacy.Denyf("API key %d can not create resources in project %d", apiKey.ID, mProjectID)
			}

			return privacy.Allowf("API key %d can create resources in project %d", apiKey.ID, mProjectID)
		case ent.OpUpdateOne, ent.OpDeleteOne, ent.OpUpdate, ent.OpDelete:
			mutation.WhereP(func(s *sql.Selector) {
				s.Where(sql.EQ("project_id", apiKey.ProjectID))
			})

			return privacy.Allowf("API key %d can modify resources in project %d", apiKey.ID, apiKey.ProjectID)
		default:
			return privacy.Denyf("Unsupported operation %s", mutation.Op())
		}
	default:
		return privacy.Skipf("Not a project-related mutation")
	}
}
