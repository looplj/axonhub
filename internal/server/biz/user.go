package biz

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

type UserServiceParams struct {
	fx.In

	CacheConfig xcache.Config
}

type UserService struct {
	UserCache xcache.Cache[ent.User]
}

func NewUserService(params UserServiceParams) *UserService {
	return &UserService{
		UserCache: xcache.NewFromConfig[ent.User](params.CacheConfig),
	}
}

// CreateUser creates a new user with hashed password.
func (s *UserService) CreateUser(ctx context.Context, input ent.CreateUserInput) (*ent.User, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	// Hash the password
	hashedPassword, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	mut := client.User.Create().
		SetNillableFirstName(input.FirstName).
		SetNillableLastName(input.LastName).
		SetEmail(input.Email).
		SetPassword(hashedPassword).
		SetScopes(input.Scopes)

	if input.RoleIDs != nil {
		mut.AddRoleIDs(input.RoleIDs...)
	}

	user, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// UpdateUser updates an existing user.
func (s *UserService) UpdateUser(ctx context.Context, id int, input ent.UpdateUserInput) (*ent.User, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	mut := client.User.UpdateOneID(id).
		SetNillableEmail(input.Email).
		SetNillableFirstName(input.FirstName).
		SetNillableLastName(input.LastName).
		SetNillableIsOwner(input.IsOwner)

	if input.Password != nil {
		hashedPassword, err := HashPassword(*input.Password)
		if err != nil {
			return nil, err
		}

		mut.SetPassword(hashedPassword)
	}

	if input.Scopes != nil {
		mut.SetScopes(input.Scopes)
	}

	if input.AppendScopes != nil {
		mut.AppendScopes(input.AppendScopes)
	}

	if input.ClearScopes {
		mut.ClearScopes()
	}

	if input.AddRoleIDs != nil {
		mut.AddRoleIDs(input.AddRoleIDs...)
	}

	if input.RemoveRoleIDs != nil {
		mut.RemoveRoleIDs(input.RemoveRoleIDs...)
	}

	if input.ClearRoles {
		mut.ClearRoles()
	}

	user, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Invalidate cache
	s.invalidateUserCache(ctx, id)

	return user, nil
}

// UpdateUserStatus updates the status of a user.
func (s *UserService) UpdateUserStatus(ctx context.Context, id int, status user.Status) (*ent.User, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	user, err := client.User.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user status: %w", err)
	}

	// Invalidate cache
	s.invalidateUserCache(ctx, id)

	return user, nil
}

// GetUserByID gets a user by ID with caching.
func (s *UserService) GetUserByID(ctx context.Context, id int) (*ent.User, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Try cache first
	cacheKey := buildUserCacheKey(id)
	if user, err := s.UserCache.Get(ctx, cacheKey); err == nil {
		return &user, nil
	}

	// Query database
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	user, err := client.User.Query().
		Where(user.IDEQ(id)).
		WithRoles().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Cache the user
	// TODO: handle role scope changed.
	err = s.UserCache.Set(ctx, cacheKey, *user)
	if err != nil {
		log.Warn(ctx, "failed to cache user", zap.Error(err))
	}

	return user, nil
}

func buildUserCacheKey(id int) string {
	return fmt.Sprintf("user:%d", id)
}

// invalidateUserCache removes a user from cache.
func (s *UserService) invalidateUserCache(ctx context.Context, id int) {
	cacheKey := buildUserCacheKey(id)
	_ = s.UserCache.Delete(ctx, cacheKey)
}
