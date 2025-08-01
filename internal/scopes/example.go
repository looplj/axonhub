package scopes

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"

	"github.com/looplj/axonhub/internal/contexts"
)

// ExampleUsage 展示如何使用权限控制系统
func ExampleUsage() {
	// 这是一个示例，展示如何在实际代码中使用权限控制

	// 1. 在 GraphQL resolver 或 HTTP handler 中，首先将用户信息放入 context
	ctx := context.Background()

	// 假设我们有一个用户
	user := &ent.User{
		ID:      1,
		Email:   "user@example.com",
		IsOwner: false,
		Scopes:  []string{string(ScopeReadChannels), string(ScopeWriteAPIKeys)},
	}

	// 将用户信息放入 context
	ctx = contexts.WithUser(ctx, user)

	// 2. 检查用户是否拥有特定权限
	if HasScope(ctx, user, ScopeReadChannels) {
		fmt.Println("用户可以读取渠道信息")
	}

	if !HasScope(ctx, user, ScopeWriteChannels) {
		fmt.Println("用户不能写入渠道信息")
	}

	// 3. 获取用户的所有权限
	userScopes := GetUserScopes(ctx, user)
	fmt.Printf("用户拥有的权限: %v\n", userScopes)

	// 4. 在 ent 查询中，privacy 规则会自动应用
	// 例如：
	// client := ent.NewClient(ent.Driver(driver))
	// channels, err := client.Channel.Query().All(ctx)
	// 这个查询会自动应用 Channel 的 privacy 策略

	// 5. Owner 用户示例
	ownerUser := &ent.User{
		ID:      2,
		Email:   "owner@example.com",
		IsOwner: true,
		Scopes:  []string{}, // Owner 不需要具体的 scopes
	}

	ownerCtx := contexts.WithUser(ctx, ownerUser)

	if HasScope(ownerCtx, ownerUser, ScopeWriteChannels) {
		fmt.Println("Owner 用户拥有所有权限")
	}

	ownerScopes := GetUserScopes(ownerCtx, ownerUser)
	fmt.Printf("Owner 用户拥有的权限: %v\n", ownerScopes)
}

// ExampleRoleBasedAccess 展示基于角色的权限控制
func ExampleRoleBasedAccess() {
	ctx := context.Background()

	// 创建一个角色
	adminRole := &ent.Role{
		ID:     1,
		Code:   "admin",
		Name:   "管理员",
		Scopes: []string{string(ScopeReadUsers), string(ScopeWriteUsers), string(ScopeReadChannels)},
	}

	// 创建一个用户，该用户拥有管理员角色
	user := &ent.User{
		ID:      3,
		Email:   "admin@example.com",
		IsOwner: false,
		Scopes:  []string{string(ScopeReadAPIKeys)}, // 用户直接拥有的权限
		Edges: ent.UserEdges{
			Roles: []*ent.Role{adminRole}, // 用户的角色
		},
	}

	ctx = contexts.WithUser(ctx, user)

	// 检查用户权限（包括角色权限）
	if HasScope(ctx, user, ScopeReadUsers) {
		fmt.Println("用户通过角色拥有读取用户的权限")
	}

	if HasScope(ctx, user, ScopeReadAPIKeys) {
		fmt.Println("用户直接拥有读取API密钥的权限")
	}

	// 获取用户的所有权限（包括角色权限）
	allScopes := GetUserScopes(ctx, user)
	fmt.Printf("用户拥有的所有权限: %v\n", allScopes)
}

// ExampleMiddleware 展示如何在中间件中使用权限控制
func ExampleMiddleware() {
	// 这是一个示例中间件，展示如何在 HTTP 请求处理中集成权限控制

	// 在实际的 HTTP handler 中：
	// func (h *Handler) GetChannels(w http.ResponseWriter, r *http.Request) {
	//     ctx := r.Context()
	//
	//     // 从 JWT token 或 session 中获取用户信息
	//     user, err := h.getUserFromRequest(r)
	//     if err != nil {
	//         http.Error(w, "Unauthorized", http.StatusUnauthorized)
	//         return
	//     }
	//
	//     // 将用户信息放入 context
	//     ctx = contexts.WithUser(ctx, user)
	//
	//     // 检查权限
	//     if !HasScope(ctx, user, ScopeReadChannels) {
	//         http.Error(w, "Forbidden", http.StatusForbidden)
	//         return
	//     }
	//
	//     // 执行查询，privacy 规则会自动应用
	//     channels, err := h.client.Channel.Query().All(ctx)
	//     if err != nil {
	//         http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	//         return
	//     }
	//
	//     // 返回结果
	//     json.NewEncoder(w).Encode(channels)
	// }

	fmt.Println("参考上面的注释代码，了解如何在 HTTP handler 中使用权限控制")
}
