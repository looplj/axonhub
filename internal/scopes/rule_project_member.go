package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/ent/privacy"
)

// ProjectMemberQueryRule allows users to query projects they are members of.
// TODO: implement project member query rule.
func ProjectMemberQueryRule() privacy.QueryRule {
	return privacy.FilterFunc(projectMemberQueryFilter)
}

func projectMemberQueryFilter(ctx context.Context, q privacy.Filter) error {
	return privacy.Allow
	// user, err := getUserFromContext(ctx)
	// if err != nil {
	// 	return err
	// }

	// switch q := q.(type) {
	// case *ent.ProjectQuery:
	// 	// Filter projects where the user is a member
	// 	q.Where(project.HasUsersWith(
	// 		func(uq *ent.UserQuery) {
	// 			uq.Where(func(s *ent.UserQuery) {
	// 				s.ID(user.ID)
	// 			})
	// 		},
	// 	))
	// 	return privacy.Allowf("User %d can query projects they are members of", user.ID)
	// default:
	// 	return privacy.Skipf("Not a project query")
	// }
}
