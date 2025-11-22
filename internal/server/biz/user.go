package biz

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

type UserServiceParams struct {
	fx.In

	CacheConfig xcache.Config
	Ent         *ent.Client
}

type UserService struct {
	*AbstractService

	UserCache           xcache.Cache[ent.User]
	permissionValidator *PermissionValidator
}

func NewUserService(params UserServiceParams) *UserService {
	return &UserService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		UserCache:           xcache.NewFromConfig[ent.User](params.CacheConfig),
		permissionValidator: NewPermissionValidator(),
	}
}

// CreateUser creates a new user with hashed password.
func (s *UserService) CreateUser(ctx context.Context, input ent.CreateUserInput) (*ent.User, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := s.entFromContext(ctx)

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
	// Validate permissions before updating
	if err := s.permissionValidator.CanEditUserPermissions(ctx, id, nil); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Validate scope grants if scopes are being updated
	if input.Scopes != nil {
		if err := s.permissionValidator.CanGrantScopes(ctx, input.Scopes, nil); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	// Validate role grants if roles are being added
	if input.AddRoleIDs != nil {
		for _, roleID := range input.AddRoleIDs {
			if err := s.permissionValidator.CanEditRole(ctx, roleID, nil); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
	}

	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := s.entFromContext(ctx)

	mut := client.User.UpdateOneID(id).
		SetNillableEmail(input.Email).
		SetNillableFirstName(input.FirstName).
		SetNillableLastName(input.LastName).
		SetNillableIsOwner(input.IsOwner).
		SetNillablePreferLanguage(input.PreferLanguage)

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
	client := s.entFromContext(ctx)

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
	client := s.entFromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	user, err := client.User.Query().
		Where(user.IDEQ(id)).
		WithRoles().
		WithProjects().
		WithProjectUsers().
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

// clearUserCache clears all user cache.
func (s *UserService) clearUserCache(ctx context.Context) {
	_ = s.UserCache.Clear(ctx)
}

// ConvertUserToUserInfo converts ent.User to objects.UserInfo.
// This method handles the conversion of user data including roles, scopes, and projects.
// Note: This function panics if the provided user is nil.
func ConvertUserToUserInfo(ctx context.Context, u *ent.User) *objects.UserInfo {
	// Convert ent.Role to objects.RoleInfo (global roles only)
	userRoles := make([]objects.RoleInfo, 0)

	for _, r := range u.Edges.Roles {
		if !r.IsSystemRole() {
			// Skip project-specific roles, they will be included in project info
			continue
		}

		userRoles = append(userRoles, objects.RoleInfo{
			Name: r.Name,
		})
	}

	// Calculate all scopes (user scopes + global role scopes)
	allScopes := make(map[string]bool)

	// Add user's direct scopes
	for _, scope := range u.Scopes {
		allScopes[scope] = true
	}

	projectRoles := map[int][]*ent.Role{}

	// Add scopes from all global roles
	for _, r := range u.Edges.Roles {
		if !r.IsSystemRole() {
			projectRoles[*r.ProjectID] = append(projectRoles[*r.ProjectID], r)
			continue
		}

		for _, scope := range r.Scopes {
			allScopes[scope] = true
		}
	}

	// Convert user projects to objects.UserProjectInfo
	userProjects := make([]objects.UserProjectInfo, 0, len(u.Edges.ProjectUsers))

	for _, up := range u.Edges.ProjectUsers {
		// Convert project roles to objects.RoleInfo
		roles := projectRoles[up.ProjectID]

		projectRoleInfos := make([]objects.RoleInfo, 0, len(roles))
		for _, r := range roles {
			projectRoleInfos = append(projectRoleInfos, objects.RoleInfo{
				Name: r.Name,
			})
		}

		userProjects = append(userProjects, objects.UserProjectInfo{
			ProjectID: objects.GUID{Type: ent.TypeProject, ID: up.ProjectID},
			IsOwner:   up.IsOwner,
			Scopes:    up.Scopes,
			Roles:     projectRoleInfos,
		})
	}

	return &objects.UserInfo{
		Email:          u.Email,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		IsOwner:        u.IsOwner,
		PreferLanguage: u.PreferLanguage,
		Avatar:         &u.Avatar,
		Scopes:         lo.Keys(allScopes),
		Roles:          userRoles,
		Projects:       userProjects,
	}
}

// AddUserToProject adds a user to a project with optional owner status, scopes, and roles.
func (s *UserService) AddUserToProject(ctx context.Context, userID, projectID int, isOwner *bool, scopes []string, roleIDs []int) (*ent.UserProject, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := s.entFromContext(ctx)

	// Create the project user relationship
	mut := client.UserProject.Create().
		SetUserID(userID).
		SetProjectID(projectID)

	if isOwner != nil {
		mut.SetIsOwner(*isOwner)
	}

	if scopes != nil {
		mut.SetScopes(scopes)
	}

	userProject, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to add user to project: %w", err)
	}

	// Add roles if provided
	if len(roleIDs) > 0 {
		user, err := client.User.Get(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}

		err = user.Update().AddRoleIDs(roleIDs...).Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to add roles to user: %w", err)
		}
	}

	// Invalidate user cache
	s.invalidateUserCache(ctx, userID)

	return userProject, nil
}

// RemoveUserFromProject removes a user from a project.
func (s *UserService) RemoveUserFromProject(ctx context.Context, userID, projectID int) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := s.entFromContext(ctx)

	// Find the UserProject relationship
	userProject, err := client.UserProject.Query().
		Where(
			userproject.UserID(userID),
			userproject.ProjectID(projectID),
		).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to find user project relationship: %w", err)
	}

	// Delete the relationship (soft delete if enabled)
	err = client.UserProject.DeleteOne(userProject).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove user from project: %w", err)
	}

	// Invalidate user cache
	s.invalidateUserCache(ctx, userID)

	return nil
}

// UpdateProjectUser updates a user's project relationship including scopes and roles.
func (s *UserService) UpdateProjectUser(ctx context.Context, userID, projectID int, scopes []string, addRoleIDs, removeRoleIDs []int) (*ent.UserProject, error) {
	// Validate permissions before updating
	if err := s.permissionValidator.CanEditUserPermissions(ctx, userID, &projectID); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Validate scope grants if scopes are being updated
	if scopes != nil {
		if err := s.permissionValidator.CanGrantScopes(ctx, scopes, &projectID); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	// Validate role grants if roles are being added
	if len(addRoleIDs) > 0 {
		for _, roleID := range addRoleIDs {
			if err := s.permissionValidator.CanEditRole(ctx, roleID, &projectID); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
	}

	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := s.entFromContext(ctx)

	// Find the UserProject relationship
	userProject, err := client.UserProject.Query().
		Where(
			userproject.UserID(userID),
			userproject.ProjectID(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find user project relationship: %w", err)
	}

	// Update the UserProject (note: isOwner is immutable, so we can only update scopes)
	mut := userProject.Update()

	if scopes != nil {
		mut.SetScopes(scopes)
	}

	userProject, err = mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update project user: %w", err)
	}

	// Update roles if provided
	user, err := client.User.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	userMut := user.Update()

	if len(addRoleIDs) > 0 {
		userMut.AddRoleIDs(addRoleIDs...)
	}

	if len(removeRoleIDs) > 0 {
		userMut.RemoveRoleIDs(removeRoleIDs...)
	}

	err = userMut.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user roles: %w", err)
	}

	// Invalidate user cache
	s.invalidateUserCache(ctx, userID)

	return userProject, nil
}
