package biz

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	entuser "github.com/looplj/axonhub/internal/ent/user"
)

// PermissionValidator provides methods to validate permission hierarchies.
type PermissionValidator struct {
	*AbstractService
}

// NewPermissionValidator creates a new PermissionValidator.
func NewPermissionValidator() *PermissionValidator {
	return &PermissionValidator{
		AbstractService: &AbstractService{}, // No db needed for validator, but embed for consistency
	}
}

// getUserScopes returns all scopes for a user (direct + role-based).
// Note: user must have Roles and ProjectUsers edges loaded.
func (v *PermissionValidator) getUserScopes(ctx context.Context, user *ent.User, projectID *int) ([]string, error) {
	scopeSet := make(map[string]bool)

	// Add user's direct scopes
	for _, scope := range user.Scopes {
		scopeSet[scope] = true
	}

	// Add scopes from global roles
	for _, role := range user.Edges.Roles {
		if role.IsSystemRole() {
			for _, scope := range role.Scopes {
				scopeSet[scope] = true
			}
		}
	}

	// Add project-specific scopes if projectID is provided
	if projectID != nil {
		for _, up := range user.Edges.ProjectUsers {
			if up.ProjectID == *projectID {
				for _, scope := range up.Scopes {
					scopeSet[scope] = true
				}

				break
			}
		}

		// Add scopes from project roles
		for _, role := range user.Edges.Roles {
			if !role.IsSystemRole() && role.ProjectID != nil && *role.ProjectID == *projectID {
				for _, scope := range role.Scopes {
					scopeSet[scope] = true
				}
			}
		}
	}

	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}

	return scopes, nil
}

// isProjectOwner checks if a user is an owner (global or project-specific).
// Note: user must have ProjectUsers edge loaded.
func (v *PermissionValidator) isProjectOwner(ctx context.Context, user *ent.User, projectID *int) (bool, error) {
	// Global owner
	if user.IsOwner {
		return true, nil
	}

	// Check project owner
	if projectID != nil {
		for _, up := range user.Edges.ProjectUsers {
			if up.ProjectID == *projectID && up.IsOwner {
				return true, nil
			}
		}
	}

	return false, nil
}

// CanGrantScopes checks if the current user can grant the specified scopes.
// Rule: Users can only grant scopes they possess themselves, unless they are owners.
func (v *PermissionValidator) CanGrantScopes(ctx context.Context, scopesToGrant []string, projectID *int) error {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return fmt.Errorf("user not found in context")
	}

	// Owners can grant any scopes
	isOwner, err := v.isProjectOwner(ctx, currentUser, projectID)
	if err != nil {
		return err
	}

	if isOwner {
		return nil
	}

	// Get current user's scopes
	userScopes, err := v.getUserScopes(ctx, currentUser, projectID)
	if err != nil {
		return err
	}

	userScopeSet := make(map[string]bool)
	for _, scope := range userScopes {
		userScopeSet[scope] = true
	}

	// Check if all scopes to grant are within user's permissions
	for _, scope := range scopesToGrant {
		if !userScopeSet[scope] {
			return fmt.Errorf("insufficient permissions: cannot grant scope '%s' that you don't possess", scope)
		}
	}

	return nil
}

// CanGrantRole checks if the current user can grant a role with the specified scopes.
func (v *PermissionValidator) CanGrantRole(ctx context.Context, roleScopes []string, projectID *int) error {
	return v.CanGrantScopes(ctx, roleScopes, projectID)
}

// CanEditUserPermissions checks if the current user can edit another user's permissions.
// Rule: Cannot edit users with higher or equal permissions, unless you are an owner.
func (v *PermissionValidator) CanEditUserPermissions(ctx context.Context, targetUserID int, projectID *int) error {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return fmt.Errorf("user not found in context")
	}

	// Get target user
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := v.entFromContext(ctx)

	targetUser, err := client.User.Query().
		Where(entuser.IDEQ(targetUserID)).
		WithRoles().
		WithProjectUsers().
		Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to get target user: %w", err)
	}

	// Check if target is an owner
	targetIsOwner, err := v.isProjectOwner(ctx, targetUser, projectID)
	if err != nil {
		return err
	}

	// Current user must be owner to edit another owner
	if targetIsOwner {
		currentIsOwner, err := v.isProjectOwner(ctx, currentUser, projectID)
		if err != nil {
			return err
		}

		if !currentIsOwner {
			return fmt.Errorf("insufficient permissions: cannot edit owner users")
		}

		return nil
	}

	// Owners can edit anyone
	currentIsOwner, err := v.isProjectOwner(ctx, currentUser, projectID)
	if err != nil {
		return err
	}

	if currentIsOwner {
		return nil
	}

	// Get both users' scopes
	currentUserScopes, err := v.getUserScopes(ctx, currentUser, projectID)
	if err != nil {
		return err
	}

	targetUserScopes, err := v.getUserScopes(ctx, targetUser, projectID)
	if err != nil {
		return err
	}

	currentScopeSet := make(map[string]bool)
	for _, scope := range currentUserScopes {
		currentScopeSet[scope] = true
	}

	// Check if all target user's scopes are within current user's permissions
	for _, scope := range targetUserScopes {
		if !currentScopeSet[scope] {
			return fmt.Errorf("insufficient permissions: target user has scope '%s' that you don't possess", scope)
		}
	}

	return nil
}

// CanEditRole checks if the current user can edit a role.
func (v *PermissionValidator) CanEditRole(ctx context.Context, roleID int, projectID *int) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := v.entFromContext(ctx)

	role, err := client.Role.Get(ctx, roleID)
	if err != nil {
		return fmt.Errorf("failed to get role: %w", err)
	}

	return v.CanGrantRole(ctx, role.Scopes, projectID)
}
