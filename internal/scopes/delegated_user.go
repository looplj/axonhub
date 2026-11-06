package scopes

import (
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
)

// DelegatedUserView returns the user as a delegated caller may see it: system and
// project scopes are the effective scopes intersected with grants. The caller struct
// is not modified.
//
// The view is not a system owner and its roles edge is nil. Project memberships keep
// their length, are not owners, and carry the project scopes that were on the
// membership or on a role for that project. Code that reads IsOwner or roles directly
// therefore cannot hand owner authority to a delegated caller.
func DelegatedUserView(user *ent.User, grants []string) *ent.User {
	if user == nil {
		return nil
	}

	copied := *user
	copied.IsOwner = false
	copied.Scopes = intersectGrants(effectiveSystemScopes(user), grants)
	copied.Edges.Roles = nil
	copied.Edges.ProjectUsers = delegatedProjectUsers(user, grants)
	return &copied
}

func effectiveSystemScopes(user *ent.User) []string {
	if user.IsOwner {
		return AllScopesAsStrings()
	}

	effective := appendUnique(nil, user.Scopes)
	for _, role := range user.Edges.Roles {
		if role == nil || !role.IsSystemRole() {
			continue
		}
		effective = appendUnique(effective, role.Scopes)
	}
	return effective
}

func effectiveProjectScopes(user *ent.User, membership *ent.UserProject) []string {
	if user.IsOwner || membership.IsOwner {
		return AllScopesAsStrings()
	}

	effective := appendUnique(nil, membership.Scopes)
	for _, role := range user.Edges.Roles {
		if role == nil || role.ProjectID == nil || *role.ProjectID != membership.ProjectID {
			continue
		}
		effective = appendUnique(effective, role.Scopes)
	}
	return effective
}

func delegatedProjectUsers(user *ent.User, grants []string) []*ent.UserProject {
	source := user.Edges.ProjectUsers
	if source == nil {
		return nil
	}

	copied := make([]*ent.UserProject, len(source))
	for i, membership := range source {
		if membership == nil {
			continue
		}
		item := *membership
		item.IsOwner = false
		item.Scopes = intersectGrants(effectiveProjectScopes(user, membership), grants)
		copied[i] = &item
	}
	return copied
}

func intersectGrants(effective, grants []string) []string {
	allowed := lo.SliceToMap(grants, func(item string) (string, struct{}) {
		return item, struct{}{}
	})
	seen := make(map[string]struct{}, len(effective))
	out := make([]string, 0, len(effective))
	for _, item := range effective {
		if _, ok := allowed[item]; !ok {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func appendUnique(dst, items []string) []string {
	seen := lo.SliceToMap(dst, func(item string) (string, struct{}) {
		return item, struct{}{}
	})
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		dst = append(dst, item)
	}
	return dst
}
