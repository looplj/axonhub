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

// UserProjectScopeReadRequestsRule restricts project request and usage log reads.
// Members see their own Playground records and API requests permitted by key visibility;
// project owners and system-scoped users can audit all records in the project.
func UserProjectScopeReadRequestsRule(requiredScope ScopeSlug) privacy.QueryRule {
	return privacy.FilterFunc(func(ctx context.Context, q privacy.Filter) error {
		projectID, hasProjectID := contexts.GetProjectID(ctx)
		if !hasProjectID {
			return privacy.Skipf("Project ID not found in context")
		}

		currentUser, err := getUserFromContext(ctx)
		if err != nil {
			return privacy.Skipf("User not found in context")
		}

		if !HasSystemScope(currentUser, requiredScope) && !userHasProjectScope(currentUser, projectID, requiredScope) {
			return privacy.Skipf("User %d can not query project %d with scope %s", currentUser.ID, projectID, requiredScope)
		}

		if pf, ok := q.(ProjectOwnedFilter); ok {
			pf.WhereProjectID(entql.IntEQ(projectID))
		} else {
			return privacy.Skipf("Not a project-owned query")
		}

		if HasSystemScope(currentUser, requiredScope) || userIsProjectOwner(currentUser, projectID) {
			return privacy.Allowf("User %d can audit requests in project %d", currentUser.ID, projectID)
		}

		switch q := q.(type) {
		case *ent.RequestFilter:
			q.Where(entql.Or(
				entql.And(
					entql.FieldEQ(request.FieldSource, string(request.SourcePlayground)),
					entql.FieldEQ(request.FieldUserID, currentUser.ID),
				),
				entql.And(
					entql.FieldNEQ(request.FieldSource, string(request.SourcePlayground)),
					entql.Or(
						entql.FieldNil(request.FieldAPIKeyID),
						entql.HasEdgeWith("api_key", sqlgraph.WrapFunc(func(s *sql.Selector) {
							apikey.Or(apikey.TypeNEQ(apikey.TypePersonal), apikey.UserID(currentUser.ID))(s)
						})),
					),
				),
			))
		case *ent.UsageLogFilter:
			q.WhereHasRequestWith(request.Or(
				request.And(request.SourceEQ(request.SourcePlayground), request.UserIDEQ(currentUser.ID)),
				request.And(
					request.SourceNEQ(request.SourcePlayground),
					request.Or(request.APIKeyIDIsNil(), request.HasAPIKeyWith(
						apikey.Or(apikey.TypeNEQ(apikey.TypePersonal), apikey.UserID(currentUser.ID)),
					)),
				),
			))
		}

		return privacy.Allowf("User %d can query permitted requests in project %d", currentUser.ID, projectID)
	})
}
