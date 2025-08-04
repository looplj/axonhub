package scopes

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/privacy"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
)

// OwnerRule 允许 owner 用户访问所有功能.
func OwnerRule() privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		user, ok := contexts.GetUser(ctx)
		if !ok || user == nil {
			return privacy.Denyf("no user in context")
		}

		// owner 用户拥有所有权限
		if user.IsOwner {
			return privacy.Allow
		}

		return privacy.Skip
	})
}

// ScopeRule 检查用户是否拥有指定的 scope 权限.
func ScopeRule(requiredScope Scope) privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		user, ok := contexts.GetUser(ctx)
		if !ok || user == nil {
			return privacy.Denyf("no user in context")
		}

		// owner 用户拥有所有权限
		if user.IsOwner {
			return privacy.Allow
		}

		// 检查用户直接拥有的 scopes
		if hasScope(user.Scopes, string(requiredScope)) {
			return privacy.Allow
		}

		// 检查用户角色的 scopes
		if hasRoleScope(ctx, user, requiredScope) {
			return privacy.Allow
		}

		return privacy.Denyf("user does not have required scope: %s", requiredScope)
	})
}

// scopeQueryRule 自定义 QueryRule 实现.
type scopeQueryRule struct {
	requiredScope Scope
}

func (r scopeQueryRule) EvalQuery(ctx context.Context, q ent.Query) error {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return privacy.Denyf("no user in context")
	}

	// owner 用户拥有所有权限
	if user.IsOwner {
		return privacy.Allow
	}

	// 检查用户直接拥有的 scopes
	if hasScope(user.Scopes, string(r.requiredScope)) {
		return privacy.Allow
	}

	// 检查用户角色的 scopes
	if hasRoleScope(ctx, user, r.requiredScope) {
		return privacy.Allow
	}

	return privacy.Denyf("user does not have required read scope: %s", r.requiredScope)
}

// ReadScopeRule 检查读取权限.
func ReadScopeRule(readScope Scope) privacy.QueryRule {
	return scopeQueryRule{requiredScope: readScope}
}

// WriteScopeRule 检查写入权限.
func WriteScopeRule(writeScope Scope) privacy.MutationRule {
	return privacy.MutationRuleFunc(func(ctx context.Context, m ent.Mutation) error {
		user, ok := contexts.GetUser(ctx)
		if !ok || user == nil {
			return privacy.Denyf("no user in context")
		}

		// owner 用户拥有所有权限
		if user.IsOwner {
			return privacy.Allow
		}

		// 检查用户直接拥有的 scopes
		if hasScope(user.Scopes, string(writeScope)) {
			return privacy.Allow
		}

		// 检查用户角色的 scopes
		if hasRoleScope(ctx, user, writeScope) {
			return privacy.Allow
		}

		return privacy.Denyf("user does not have required write scope: %s", writeScope)
	})
}

// userOwnedQueryRule 自定义 QueryRule 实现用于用户拥有的资源.
type userOwnedQueryRule struct{}

func (r userOwnedQueryRule) EvalQuery(ctx context.Context, q ent.Query) error {
	ctxUser, ok := contexts.GetUser(ctx)
	if !ok || ctxUser == nil {
		return privacy.Denyf("no user in context")
	}

	// owner 用户拥有所有权限
	if ctxUser.IsOwner {
		return privacy.Allow
	}

	// 对于查询，过滤只属于当前用户的资源
	switch query := q.(type) {
	case *ent.APIKeyQuery:
		query.Where(apikey.UserID(ctxUser.ID))
		return privacy.Allow
	case *ent.RequestQuery:
		query.Where(request.UserID(ctxUser.ID))
		return privacy.Allow
	case *ent.RoleQuery:
		query.Where(role.HasUsersWith(func(s *sql.Selector) {
			s.Where(sql.EQ(user.FieldID, ctxUser.ID))
		}))
		return privacy.Allow
	}

	return privacy.Skip
}

// UserOwnedQueryRule 检查用户是否拥有资源（用于用户拥有的资源如 API Key）.
func UserOwnedQueryRule() privacy.QueryRule {
	return userOwnedQueryRule{}
}

// UserOwnedMutationRule 检查用户是否可以修改自己的资源.
func UserOwnedMutationRule() privacy.MutationRule {
	return privacy.MutationRuleFunc(func(ctx context.Context, m ent.Mutation) error {
		user, ok := contexts.GetUser(ctx)
		if !ok || user == nil {
			return privacy.Denyf("no user in context")
		}

		// owner 用户拥有所有权限
		if user.IsOwner {
			return privacy.Allow
		}

		// 对于变更，检查是否操作自己的资源
		switch mutation := m.(type) {
		case *ent.APIKeyMutation:
			if userID, exists := mutation.UserID(); exists && userID == user.ID {
				return privacy.Allow
			}
			return privacy.Denyf("user can only access their own API keys")
		case *ent.RequestMutation:
			if userID, exists := mutation.UserID(); exists && userID == user.ID {
				return privacy.Allow
			}
			return privacy.Denyf("user can only access their own requests")
		}

		return privacy.Skip
	})
}

// AdminScopeRule 检查管理员权限.
func AdminScopeRule() privacy.QueryMutationRule {
	return ScopeRule(ScopeAdmin)
}

// DenyIfNoUser 如果没有用户上下文则拒绝访问.
func DenyIfNoUser() privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		user, ok := contexts.GetUser(ctx)
		if !ok || user == nil {
			return privacy.Denyf("no user in context")
		}
		return privacy.Skip
	})
}

// hasScope 检查用户是否拥有指定的 scope.
func hasScope(userScopes []string, requiredScope string) bool {
	for _, scope := range userScopes {
		if scope == requiredScope {
			return true
		}
	}
	return false
}

// hasRoleScope 检查用户的角色是否拥有指定的 scope.
func hasRoleScope(ctx context.Context, user *ent.User, requiredScope Scope) bool {
	// 这里需要查询用户的角色，但为了避免循环依赖，我们需要在调用时确保角色已经加载
	// 或者使用一个专门的服务来处理这个逻辑

	// 如果用户的角色已经预加载，我们可以直接检查
	if user.Edges.Roles != nil {
		for _, role := range user.Edges.Roles {
			if hasScope(role.Scopes, string(requiredScope)) {
				return true
			}
		}
	}

	return false
}

// GetUserScopes 获取用户的所有有效 scopes（包括角色的 scopes）.
func GetUserScopes(ctx context.Context, user *ent.User) []string {
	if user.IsOwner {
		return AllScopesAsStrings()
	}

	scopeSet := make(map[string]bool)

	// 添加用户直接拥有的 scopes
	for _, scope := range user.Scopes {
		scopeSet[scope] = true
	}

	// 添加角色的 scopes
	if user.Edges.Roles != nil {
		for _, role := range user.Edges.Roles {
			for _, scope := range role.Scopes {
				scopeSet[scope] = true
			}
		}
	}

	// 转换为切片
	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}

	return scopes
}

// HasScope 检查用户是否拥有指定的 scope.
func HasScope(ctx context.Context, user *ent.User, requiredScope Scope) bool {
	if user.IsOwner {
		return true
	}

	// 检查用户直接拥有的 scopes
	if hasScope(user.Scopes, string(requiredScope)) {
		return true
	}

	// 检查用户角色的 scopes
	return hasRoleScope(ctx, user, requiredScope)
}
