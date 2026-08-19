package scopes

import (
	"context"

	"entgo.io/ent/entql"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// PersonalKeyProjectFilter is the interface for entities that support
// both project_id filtering and generic predicate injection.
type PersonalKeyProjectFilter interface {
	WhereProjectID(entql.IntP)
	Where(entql.P)
}

// UserPersonalAPIKeyReadRule allows users to view API keys within their project.
// Personal keys are visible to their creator and project owners.
// It replaces UserProjectScopeReadRule for the APIKey schema.
func UserPersonalAPIKeyReadRule(requiredScope ScopeSlug) privacy.QueryRule {
	return privacy.FilterFunc(func(ctx context.Context, q privacy.Filter) error {
		currentUser, err := getUserFromContext(ctx)
		if err != nil {
			return privacy.Skipf("User not found in context")
		}

		projectID, hasProjectID := contexts.GetProjectID(ctx)
		if !hasProjectID {
			return privacy.Skipf("Project ID not found in context")
		}

		// System-scope users can access any project, but personal keys are still filtered below
		if !HasSystemScope(currentUser, requiredScope) && !userHasProjectScope(currentUser, projectID, requiredScope) {
			return privacy.Skipf("User %d can not query project %d with scope %s", currentUser.ID, projectID, requiredScope)
		}

		pf, ok := q.(PersonalKeyProjectFilter)
		if !ok {
			return privacy.Skipf("Query does not support project_id and type filtering")
		}

		pf.WhereProjectID(entql.IntEQ(projectID))

		if !userIsProjectOwner(currentUser, projectID) {
			pf.Where(entql.Or(
				entql.FieldNEQ("type", "personal"),
				entql.FieldEQ("user_id", currentUser.ID),
			))
		}

		return privacy.Allowf("User %d can query project %d with scope %s", currentUser.ID, projectID, requiredScope)
	})
}
