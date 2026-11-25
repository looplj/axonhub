package biz

import (
	"context"
	"fmt"
	"slices"

	"github.com/samber/lo"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/ent/userrole"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

type UserServiceParams struct {
	fx.In

	CacheConfig   xcache.Config
	Ent           *ent.Client
	APIKeyService *APIKeyService
}

type UserService struct {
	*AbstractService

	UserCache           xcache.Cache[ent.User]
	apiKeyService       *APIKeyService
	permissionValidator *PermissionValidator
}

func NewUserService(params UserServiceParams) *UserService {
	return &UserService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		UserCache:           xcache.NewFromConfig[ent.User](params.CacheConfig),
		apiKeyService:       params.APIKeyService,
		permissionValidator: NewPermissionValidator(),
	}
}

// CreateUser creates a new user with hashed password.
func (s *UserService) CreateUser(ctx context.Context, input ent.CreateUserInput) (*ent.User, error) {
	if len(input.Scopes) > 0 {
		if err := s.permissionValidator.CanGrantScopes(ctx, input.Scopes, nil); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}
	for _, roleID := range input.RoleIDs {
		if err := s.permissionValidator.CanEditRole(ctx, roleID, nil); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	client := s.entFromContext(ctx)

	// Hash the password
	var hashedPassword string
	if input.Password == OIDC_ONLY_PLACEHOLDER {
		hashedPassword = OIDC_ONLY_PLACEHOLDER
	} else {
		var err error

		hashedPassword, err = HashPassword(input.Password)
		if err != nil {
			return nil, err
		}
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
	targetUser, err := s.permissionValidator.editableUser(ctx, id, nil)
	if err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}
	if input.Email != nil && *input.Email == targetUser.Email {
		input.Email = nil
	}
	if input.Password != nil || input.Email != nil {
		if err := s.permissionValidator.canManageUserAccount(ctx, targetUser); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}
	ownerChange, err := s.permissionValidator.ownerChange(ctx, nil, input.IsOwner, targetUser.IsOwner)
	if err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Validate scope grants if scopes are being updated
	if input.Scopes != nil {
		if err := s.permissionValidator.CanGrantScopes(ctx, input.Scopes, nil); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	if len(input.AppendScopes) > 0 {
		if err := s.permissionValidator.CanGrantScopes(ctx, input.AppendScopes, nil); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	roleIDsToAdd := lo.Without(input.AddRoleIDs, input.RemoveRoleIDs...)
	roleIDsToRemove := input.RemoveRoleIDs
	if input.ClearRoles {
		// Remove only the assignments covered by this permission check.
		roleIDsToRemove = lo.Map(targetUser.Edges.Roles, func(role *ent.Role, _ int) int { return role.ID })
	}
	for _, roleID := range lo.Uniq(slices.Concat(input.AddRoleIDs, roleIDsToRemove)) {
		if err := s.permissionValidator.CanEditRole(ctx, roleID, nil); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	client := s.entFromContext(ctx)

	mut := client.User.UpdateOneID(id).
		SetNillableEmail(input.Email).
		SetNillableFirstName(input.FirstName).
		SetNillableLastName(input.LastName).
		SetNillableIsOwner(ownerChange).
		SetNillablePreferLanguage(input.PreferLanguage)

	if input.ClearAvatar {
		mut.ClearAvatar()
	} else {
		mut.SetNillableAvatar(input.Avatar)
	}

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

	if len(roleIDsToRemove) > 0 {
		mut.RemoveRoleIDs(roleIDsToRemove...)
	}
	if len(roleIDsToAdd) > 0 {
		mut.AddRoleIDs(roleIDsToAdd...)
	}

	user, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	runAfterCommit(ctx, func(ctx context.Context) {
		s.invalidateUserCache(ctx, id)
	})

	return user, nil
}

// UpdateOwnProfile updates fields users are allowed to change for their own account.
func (s *UserService) UpdateOwnProfile(ctx context.Context, input ent.UpdateUserInput) (*ent.User, error) {
	if err := authz.RequireDirectUser(ctx); err != nil {
		return nil, err
	}

	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	id := currentUser.ID

	return authz.RunWithSystemBypass(ctx, "update-own-profile", func(ctx context.Context) (*ent.User, error) {
		client := s.entFromContext(ctx)

		mut := client.User.UpdateOneID(id).
			SetNillableFirstName(input.FirstName).
			SetNillableLastName(input.LastName).
			SetNillablePreferLanguage(input.PreferLanguage)

		if input.ClearAvatar {
			mut.ClearAvatar()
		} else {
			mut.SetNillableAvatar(input.Avatar)
		}

		if input.Password != nil {
			hashedPassword, err := HashPassword(*input.Password)
			if err != nil {
				return nil, err
			}

			mut.SetPassword(hashedPassword)
		}

		user, err := mut.Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to update user profile: %w", err)
		}

		// Invalidate cache
		s.invalidateUserCache(ctx, id)

		return user, nil
	})
}

// UpdateUserStatus updates the status of a user.
func (s *UserService) UpdateUserStatus(ctx context.Context, id int, status user.Status) (*ent.User, error) {
	var updatedUser *ent.User

	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		target, err := s.permissionValidator.editableUser(ctx, id, nil)
		if err != nil {
			return fmt.Errorf("permission denied: %w", err)
		}
		if err := s.permissionValidator.canManageUserAccount(ctx, target); err != nil {
			return fmt.Errorf("permission denied: %w", err)
		}
		client := s.entFromContext(ctx)

		result, err := client.User.UpdateOneID(id).
			SetStatus(status).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update user status: %w", err)
		}
		updatedUser = result

		if status == user.StatusDeactivated {
			if err := s.apiKeyService.disablePersonalAPIKeysByUser(ctx, id); err != nil {
				return fmt.Errorf("failed to disable user personal API keys: %w", err)
			}
		}

		runAfterCommit(ctx, func(ctx context.Context) {
			s.invalidateUserCache(ctx, id)
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

// GetUserByID gets a user by ID with caching.
func (s *UserService) GetUserByID(ctx context.Context, id int) (*ent.User, error) {
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
		WithOidcIdentities().
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
		effectiveScopeSet := make(map[string]bool, len(up.Scopes))
		for _, scope := range up.Scopes {
			effectiveScopeSet[scope] = true
		}

		projectRoleInfos := make([]objects.RoleInfo, 0, len(roles))
		for _, r := range roles {
			projectRoleInfos = append(projectRoleInfos, objects.RoleInfo{
				Name: r.Name,
			})
			for _, scope := range r.Scopes {
				effectiveScopeSet[scope] = true
			}
		}

		userProjects = append(userProjects, objects.UserProjectInfo{
			ProjectID:       objects.GUID{Type: ent.TypeProject, ID: up.ProjectID},
			IsOwner:         up.IsOwner,
			Scopes:          up.Scopes,
			EffectiveScopes: lo.Keys(effectiveScopeSet),
			Roles:           projectRoleInfos,
		})
	}

	// Convert OIDC identities
	oidcIdentities := make([]objects.OIDCIdentityInfo, 0, len(u.Edges.OidcIdentities))
	for _, identity := range u.Edges.OidcIdentities {
		oidcIdentities = append(oidcIdentities, objects.OIDCIdentityInfo{
			ID:      objects.GUID{Type: ent.TypeOIDCIdentity, ID: identity.ID},
			IdpName: identity.IdpName,
			Issuer:  identity.Issuer,
			Subject: identity.Subject,
			Email:   identity.Email,
		})
	}

	return &objects.UserInfo{
		ID:             objects.GUID{Type: ent.TypeUser, ID: u.ID},
		Email:          u.Email,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		IsOwner:        u.IsOwner,
		PreferLanguage: u.PreferLanguage,
		Avatar:         &u.Avatar,
		Scopes:         lo.Keys(allScopes),
		Roles:          userRoles,
		Projects:       userProjects,
		OIDCIdentities: oidcIdentities,
		HasPassword:    u.Password != OIDC_ONLY_PLACEHOLDER,
	}
}

// AddUserToProject adds a user to a project with optional owner status, scopes, and roles.
func (s *UserService) AddUserToProject(ctx context.Context, userID, projectID int, isOwner *bool, scopes []string, roleIDs []int) (*ent.UserProject, error) {
	ownerChange, err := s.permissionValidator.ownerChange(ctx, &projectID, isOwner, false)
	if err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}
	principal, hasPrincipal := authz.GetPrincipal(ctx)
	if !hasPrincipal || !principal.IsTest() {
		if scopes != nil {
			if err := s.permissionValidator.CanGrantScopes(ctx, scopes, &projectID); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
		for _, roleID := range roleIDs {
			projectRole, err := authz.RunAsHost(ctx, s.entFromContext(ctx), "project-role-assignment", func(ctx context.Context) (*ent.Role, error) {
				return s.entFromContext(ctx).Role.Query().Where(role.IDEQ(roleID), role.ProjectIDEQ(projectID)).Only(ctx)
			})
			if err != nil {
				if ent.IsNotFound(err) {
					return nil, fmt.Errorf("project role %d not found", roleID)
				}
				return nil, fmt.Errorf("failed to load project role %d: %w", roleID, err)
			}
			if err := s.permissionValidator.CanGrantRole(ctx, projectRole.Scopes, &projectID); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
	}

	var userProject *ent.UserProject
	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)
		mut := client.UserProject.Create().
			SetUserID(userID).
			SetProjectID(projectID).
			SetNillableIsOwner(ownerChange)

		if scopes != nil {
			mut.SetScopes(scopes)
		}

		created, err := mut.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to add user to project: %w", err)
		}
		userProject = created

		return updateAuthorizedProjectUserRoles(ctx, client, userID, roleIDs, nil)
	})
	if err != nil {
		return nil, err
	}

	runAfterCommit(ctx, func(ctx context.Context) {
		s.invalidateUserCache(ctx, userID)
	})

	return userProject, nil
}

// RemoveUserFromProject removes a user from a project.
func (s *UserService) RemoveUserFromProject(ctx context.Context, userID, projectID int) error {
	return s.RunInTransaction(ctx, func(ctx context.Context) error {
		target, err := s.permissionValidator.editableUser(ctx, userID, &projectID)
		if err != nil {
			return fmt.Errorf("permission denied: %w", err)
		}

		membership, ok := lo.Find(target.Edges.ProjectUsers, func(m *ent.UserProject) bool {
			return m.ProjectID == projectID
		})
		if !ok {
			return nil
		}
		client := s.entFromContext(ctx)
		rows, err := client.UserProject.Delete().Where(
			userproject.IDEQ(membership.ID),
			userproject.IsOwnerEQ(membership.IsOwner),
		).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to remove user from project: %w", err)
		}
		if rows != 1 {
			return privacy.Denyf("project member could not be removed or changed concurrently")
		}
		if _, err := client.UserRole.Delete().Where(
			userrole.UserIDEQ(userID),
			userrole.HasRoleWith(role.ProjectIDEQ(projectID)),
		).Exec(ctx); err != nil {
			return fmt.Errorf("failed to remove user project roles: %w", err)
		}
		runAfterCommit(ctx, func(ctx context.Context) {
			s.invalidateUserCache(ctx, userID)
		})
		return nil
	})
}

// UpdateProjectUser updates a user's project relationship including scopes and roles.
func (s *UserService) UpdateProjectUser(ctx context.Context, userID, projectID int, isOwner *bool, scopes []string, addRoleIDs, removeRoleIDs []int) (*ent.UserProject, error) {
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

	for _, roleID := range slices.Concat(addRoleIDs, removeRoleIDs) {
		if err := s.permissionValidator.CanEditRole(ctx, roleID, &projectID); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	var updated *ent.UserProject
	outerTx := ent.TxFromContext(ctx)
	var unwrap bool
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		unwrap = outerTx == nil && ent.TxFromContext(ctx) != nil
		client := s.entFromContext(ctx)
		membership, err := client.UserProject.Query().
			Where(userproject.UserIDEQ(userID), userproject.ProjectIDEQ(projectID)).
			Only(ctx)
		if err != nil {
			return fmt.Errorf("failed to find user project relationship: %w", err)
		}

		ownerChange, err := s.permissionValidator.ownerChange(ctx, &projectID, isOwner, membership.IsOwner)
		if err != nil {
			return fmt.Errorf("permission denied: %w", err)
		}

		mut := membership.Update().SetNillableIsOwner(ownerChange)
		if scopes != nil {
			mut.SetScopes(scopes)
		}
		updated, err = mut.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update project user: %w", err)
		}

		return updateAuthorizedProjectUserRoles(ctx, client, userID, addRoleIDs, removeRoleIDs)
	})
	if err != nil {
		return nil, err
	}
	if unwrap {
		updated.Unwrap()
	}
	runAfterCommit(ctx, func(ctx context.Context) {
		s.invalidateUserCache(ctx, userID)
	})
	return updated, nil
}

// updateAuthorizedProjectUserRoles applies validated role changes after the
// membership write has passed project authorization in the same transaction.
func updateAuthorizedProjectUserRoles(ctx context.Context, client *ent.Client, userID int, addRoleIDs, removeRoleIDs []int) error {
	if len(addRoleIDs) == 0 && len(removeRoleIDs) == 0 {
		return nil
	}

	_, err := authz.RunAsHost(ctx, client, "update-project-user-roles", func(hostCtx context.Context) (struct{}, error) {
		user, err := client.User.Get(hostCtx, userID)
		if err != nil {
			return struct{}{}, fmt.Errorf("failed to get user: %w", err)
		}
		mut := user.Update()
		if len(addRoleIDs) > 0 {
			mut.AddRoleIDs(addRoleIDs...)
		}
		if len(removeRoleIDs) > 0 {
			mut.RemoveRoleIDs(removeRoleIDs...)
		}
		return struct{}{}, mut.Exec(hostCtx)
	})
	if err != nil {
		return fmt.Errorf("failed to update user roles: %w", err)
	}
	return nil
}

// DeleteUser soft deletes a user and handles all related data.
// This method performs the following operations:
// 1. Validates permissions
// 2. Checks if user is owner (cannot delete owner)
// 3. Removes user from all projects (UserProject)
// 4. Removes all user roles (UserRole)
// 5. Archives the user's personal API keys
// 6. Soft deletes the user
// 7. Invalidates user cache.
func (s *UserService) DeleteUser(ctx context.Context, id int) error {
	// Validate permissions before deleting
	if err := s.permissionValidator.CanDeleteUser(ctx, id); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	return s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		// Get user to check if it's an owner
		u, err := client.User.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		// Cannot delete owner users
		if u.IsOwner {
			return fmt.Errorf("cannot delete owner user, transfer ownership first")
		}

		// 1. Delete UserProject relationships
		_, err = client.UserProject.Delete().
			Where(userproject.UserIDEQ(id)).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete user projects: %w", err)
		}

		// 2. Delete UserRole relationships
		_, err = client.UserRole.Delete().
			Where(userrole.UserIDEQ(id)).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete user roles: %w", err)
		}

		// 3. Archive the user's personal API keys
		if err = s.apiKeyService.archivePersonalAPIKeysByUser(ctx, id); err != nil {
			return fmt.Errorf("failed to archive user personal API keys: %w", err)
		}

		// 4. Soft delete the user
		err = client.User.DeleteOneID(id).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}

		runAfterCommit(ctx, func(ctx context.Context) {
			s.invalidateUserCache(ctx, id)
		})

		return nil
	})
}
