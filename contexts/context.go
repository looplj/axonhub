package contexts

import (
	"context"

	"github.com/looplj/axonhub/ent"
)

// ContextKey 定义 context key 类型
type ContextKey string

const (
	// APIKeyContextKey 用于在 context 中存储 API key entity
	APIKeyContextKey ContextKey = "api_key"
	// UserContextKey 用于在 context 中存储用户 entity
	UserContextKey ContextKey = "user"
)

// WithAPIKey 将 API key entity 存储到 context 中
func WithAPIKey(ctx context.Context, apiKey *ent.APIKey) context.Context {
	return context.WithValue(ctx, APIKeyContextKey, apiKey)
}

// GetAPIKey 从 context 中获取 API key entity
func GetAPIKey(ctx context.Context) (*ent.APIKey, bool) {
	apiKey, ok := ctx.Value(APIKeyContextKey).(*ent.APIKey)
	return apiKey, ok
}

// GetAPIKeyString 从 context 中获取 API key 字符串（向后兼容）
func GetAPIKeyString(ctx context.Context) (string, bool) {
	apiKey, ok := GetAPIKey(ctx)
	if !ok || apiKey == nil {
		return "", false
	}
	return apiKey.Key, true
}

// WithUser 将用户 entity 存储到 context 中
func WithUser(ctx context.Context, user *ent.User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// GetUser 从 context 中获取用户 entity
func GetUser(ctx context.Context) (*ent.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*ent.User)
	return user, ok
}