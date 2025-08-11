package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// APIKeyQueryRule checks API Key permissions for queries.
func APIKeyQueryRule(requiredScope Scope) privacy.QueryRule {
	return apiKeyQueryRule{requiredScope: requiredScope}
}

// apiKeyQueryRule custom QueryRule implementation for checking API Key scopes.
type apiKeyQueryRule struct {
	requiredScope Scope
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
func APIKeyScopeMutationRule(requiredScope Scope) privacy.MutationRule {
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
