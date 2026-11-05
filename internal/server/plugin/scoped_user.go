package plugin

import (
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/scopes"
)

// BuildPluginScopedUser returns a copy of user whose scopes are the effective scopes
// intersected with ceiling. The caller struct is not modified.
//
// The copy is not a system owner and its roles edge is nil. Project memberships
// keep their length, are not owners, and carry the project scopes that were on
// the membership or on a role for that project.
func BuildPluginScopedUser(user *ent.User, ceiling []string) *ent.User {
	if user == nil {
		return nil
	}

	copied := *user
	copied.IsOwner = false
	copied.Scopes = intersectCeiling(effectiveSystemScopes(user), ceiling)
	copied.Edges.Roles = nil
	copied.Edges.ProjectUsers = copyProjectUsers(user, ceiling)
	return &copied
}

func effectiveSystemScopes(user *ent.User) []string {
	if user.IsOwner {
		return scopes.AllScopesAsStrings()
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
		return scopes.AllScopesAsStrings()
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

func copyProjectUsers(user *ent.User, ceiling []string) []*ent.UserProject {
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
		item.Scopes = intersectCeiling(effectiveProjectScopes(user, membership), ceiling)
		copied[i] = &item
	}
	return copied
}

func intersectCeiling(effective, ceiling []string) []string {
	allowed := lo.SliceToMap(ceiling, func(item string) (string, struct{}) {
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
