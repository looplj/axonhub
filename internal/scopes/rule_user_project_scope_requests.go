package scopes

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/entql"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
)

// UserProjectScopeReadRequestsRule is a query rule that restricts users' access to Request and UsageLog records.
// It ensures that project-level users can only see:
// 1. Playground Requests / UsageLogs created by the current user.
// 2. Requests / UsageLogs that were made using a non-personal API key.
// 3. Requests / UsageLogs that were made using a personal API key created by the user themselves.
func UserProjectScopeReadRequestsRule(requiredScope ScopeSlug) privacy.QueryRule {
	return privacy.FilterFunc(func(ctx context.Context, q privacy.Filter) error {
		// Check if project ID is in context
		projectID, hasProjectID := contexts.GetProjectID(ctx)
		if !hasProjectID {
			return privacy.Skipf("Project ID not found in context")
		}

		currentUser, err := getUserFromContext(ctx)
		if err != nil {
			return privacy.Skipf("User not found in context")
		}

		// Check if user has global scope permission or project scope permission.
		if !HasSystemScope(currentUser, requiredScope) && !userHasProjectScope(currentUser, projectID, requiredScope) {
			return privacy.Skipf("User %d can not query project %d with scope %s", currentUser.ID, projectID, requiredScope)
		}

		// Apply project_id filtering
		if pf, ok := q.(ProjectOwnedFilter); ok {
			pf.WhereProjectID(entql.IntEQ(projectID))
		} else {
			return privacy.Skipf("Not a project-owned query")
		}

		if HasSystemScope(currentUser, requiredScope) || userIsProjectOwner(currentUser, projectID) {
			return privacy.Allowf("User %d can query all requests in project %d with scope %s", currentUser.ID, projectID, requiredScope)
		}

		// Apply personal API key log visibility filter
		switch q := q.(type) {
		case *ent.RequestFilter:
			q.Where(entql.Or(
				entql.And(
					entql.FieldEQ(request.FieldSource, string(request.SourcePlayground)),
					entql.FieldEQ("user_id", currentUser.ID),
				),
				entql.And(
					entql.FieldNEQ(request.FieldSource, string(request.SourcePlayground)),
					entql.Or(
						entql.FieldNil(request.FieldAPIKeyID),
						entql.HasEdgeWith("api_key", sqlgraph.WrapFunc(func(s *sql.Selector) {
							apikey.Or(
								apikey.TypeNEQ(apikey.TypePersonal),
								apikey.UserID(currentUser.ID),
							)(s)
						})),
					),
				),
			))
		case *ent.UsageLogFilter:
			q.WhereHasRequestWith(
				request.Or(
					request.And(
						request.SourceEQ(request.SourcePlayground),
						request.UserIDEQ(currentUser.ID),
					),
					request.And(
						request.SourceNEQ(request.SourcePlayground),
						request.Or(
							request.APIKeyIDIsNil(),
							request.HasAPIKeyWith(
								apikey.Or(
									apikey.TypeNEQ(apikey.TypePersonal),
									apikey.UserID(currentUser.ID),
								),
							),
						),
					),
				),
			)
		}

		return privacy.Allowf("User %d can query requests in project %d with personal key filtering", currentUser.ID, projectID)
	})
}

// userIsProjectOwner reports whether a user owns the selected project.
func userIsProjectOwner(user *ent.User, projectID int) bool {
	for _, membership := range user.Edges.ProjectUsers {
		if membership.ProjectID == projectID && membership.IsOwner {
			return true
		}
	}

	return false
}
