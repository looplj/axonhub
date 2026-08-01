package biz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/invitation"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/scopes"
)

const defaultInvitationLifetime = 7 * 24 * time.Hour

// InvitationServiceParams contains dependencies for the invitation service.
type InvitationServiceParams struct {
	fx.In

	Ent *ent.Client
}

// InvitationService manages project invitations and invitation registration.
type InvitationService struct {
	*AbstractService

	permissionValidator *PermissionValidator
}

// NewInvitationService creates an invitation service.
func NewInvitationService(params InvitationServiceParams) *InvitationService {
	return &InvitationService{
		AbstractService:     &AbstractService{db: params.Ent},
		permissionValidator: NewPermissionValidator(),
	}
}

// InvitationInfo describes the current state of an invitation.
type InvitationInfo struct {
	ProjectName   string
	ExpiresAt     *time.Time
	MaxUses       int
	UsedCount     int
	RemainingUses int
}

// CreatedInvitation contains a newly generated invitation token and metadata.
type CreatedInvitation struct {
	Token string
	Info  InvitationInfo
}

// CreateInvitation creates a role-bound invitation for a project.
func (s *InvitationService) CreateInvitation(ctx context.Context, projectID, roleID int, expiresInHours *int, maxUses int) (*CreatedInvitation, error) {
	if err := s.canInvite(ctx, projectID); err != nil {
		return nil, err
	}
	if roleID == 0 {
		return nil, fmt.Errorf("project role is required")
	}

	if maxUses != 0 && maxUses != 1 {
		return nil, fmt.Errorf("max uses must be 1 or 0")
	}

	expiresAt, err := invitationExpiry(expiresInHours)
	if err != nil {
		return nil, err
	}

	token, err := generateInvitationToken()
	if err != nil {
		return nil, err
	}

	projectRow, err := authz.RunWithSystemBypass(ctx, "invitation-project-lookup", func(ctx context.Context) (*ent.Project, error) {
		return s.entFromContext(ctx).Project.Query().Where(project.IDEQ(projectID)).Only(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	projectRole, err := authz.RunWithSystemBypass(ctx, "invitation-role-lookup", func(ctx context.Context) (*ent.Role, error) {
		return s.entFromContext(ctx).Role.Query().
			Where(role.IDEQ(roleID), role.ProjectIDEQ(projectID)).
			Only(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("project role not found")
	}
	if err := s.permissionValidator.CanGrantRole(ctx, projectRole.Scopes, &projectID); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	created, err := authz.RunWithSystemBypass(ctx, "invitation-create", func(ctx context.Context) (*ent.Invitation, error) {
		return s.entFromContext(ctx).Invitation.Create().
			SetTokenHash(hashInvitationToken(token)).
			SetProjectID(projectID).
			SetRoleID(roleID).
			SetNillableExpiresAt(expiresAt).
			SetMaxUses(maxUses).
			Save(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	return &CreatedInvitation{
		Token: token,
		Info:  invitationInfo(created, projectRow.Name),
	}, nil
}

// GetInvitation returns valid invitation metadata for a token.
func (s *InvitationService) GetInvitation(ctx context.Context, token string) (*InvitationInfo, error) {
	row, err := authz.RunWithSystemBypass(ctx, "invitation-read", func(ctx context.Context) (*ent.Invitation, error) {
		return s.entFromContext(ctx).Invitation.Query().
			Where(invitation.TokenHashEQ(hashInvitationToken(token))).
			WithProject().
			Only(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("invitation not found")
	}

	if invitationIsExpired(row, time.Now()) || (row.MaxUses > 0 && row.UsedCount >= row.MaxUses) {
		return nil, fmt.Errorf("invitation is no longer valid")
	}

	info := invitationInfo(row, row.Edges.Project.Name)
	return &info, nil
}

// RegisterInvitation creates a user and assigns the invitation's project role.
func (s *InvitationService) RegisterInvitation(ctx context.Context, token, email, password, firstName, lastName string) (*ent.User, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}
	if len(password) < 7 {
		return nil, fmt.Errorf("password must be at least 7 characters")
	}

	var createdUser *ent.User
	err := authz.RunWithSystemBypassVoid(ctx, "invitation-register", func(ctx context.Context) error {
		return s.RunInTransaction(ctx, func(ctx context.Context) error {
			client := s.entFromContext(ctx)
			invitationRow, err := client.Invitation.Query().
				Where(invitation.TokenHashEQ(hashInvitationToken(token))).
				Only(ctx)
			if err != nil {
				return fmt.Errorf("invitation not found")
			}
			if invitationIsExpired(invitationRow, time.Now()) || (invitationRow.MaxUses > 0 && invitationRow.UsedCount >= invitationRow.MaxUses) {
				return fmt.Errorf("invitation is no longer valid")
			}
			roleID := invitationRow.RoleID
			if roleID == nil {
				return fmt.Errorf("invitation role is required")
			}
			if _, err := client.Role.Query().Where(
				role.IDEQ(*roleID),
				role.ProjectIDEQ(invitationRow.ProjectID),
			).Only(ctx); err != nil {
				return fmt.Errorf("invitation role is no longer available")
			}
			exists, err := client.User.Query().Where(user.EmailEQ(email)).Exist(ctx)
			if err != nil {
				return fmt.Errorf("failed to check existing user: %w", err)
			}
			if exists {
				return fmt.Errorf("a user with this email already exists")
			}

			now := time.Now()
			expiryPredicate := invitation.Or(
				invitation.ExpiresAtIsNil(),
				invitation.ExpiresAtGT(now),
			)

			if invitationRow.MaxUses > 0 {
				updated, err := client.Invitation.Update().
					Where(
						invitation.IDEQ(invitationRow.ID),
						invitation.UsedCountLT(invitationRow.MaxUses),
						expiryPredicate,
					).
					AddUsedCount(1).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to use invitation: %w", err)
				}
				if updated != 1 {
					return fmt.Errorf("invitation is no longer valid")
				}
			} else {
				updated, err := client.Invitation.Update().
					Where(
						invitation.IDEQ(invitationRow.ID),
						expiryPredicate,
					).
					AddUsedCount(1).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to use invitation: %w", err)
				}
				if updated != 1 {
					return fmt.Errorf("invitation is no longer valid")
				}
			}

			hashedPassword, err := HashPassword(password)
			if err != nil {
				return err
			}
			createdUser, err = client.User.Create().
				SetEmail(email).
				SetPassword(hashedPassword).
				SetFirstName(strings.TrimSpace(firstName)).
				SetLastName(strings.TrimSpace(lastName)).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}

			if _, err := client.UserProject.Create().SetUserID(createdUser.ID).SetProjectID(invitationRow.ProjectID).Save(ctx); err != nil {
				return fmt.Errorf("failed to add user to project: %w", err)
			}
			if err := client.User.UpdateOneID(createdUser.ID).AddRoleIDs(*roleID).Exec(ctx); err != nil {
				return fmt.Errorf("failed to assign invitation role: %w", err)
			}

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "invitation-user-lookup", func(ctx context.Context) (*ent.User, error) {
		return s.entFromContext(ctx).User.Query().
			Where(user.IDEQ(createdUser.ID)).
			WithRoles().
			WithProjectUsers().
			WithOidcIdentities().
			Only(ctx)
	})
}

func (s *InvitationService) canInvite(ctx context.Context, projectID int) error {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return fmt.Errorf("user not found in context")
	}
	if currentUser.IsOwner || scopes.UserHasScope(ctx, scopes.ScopeWriteUsers) {
		return nil
	}

	for _, membership := range currentUser.Edges.ProjectUsers {
		if membership.ProjectID != projectID {
			continue
		}
		if membership.IsOwner || slices.Contains(membership.Scopes, string(scopes.ScopeWriteUsers)) {
			return nil
		}
		for _, role := range currentUser.Edges.Roles {
			if role.ProjectID != nil && *role.ProjectID == projectID && slices.Contains(role.Scopes, string(scopes.ScopeWriteUsers)) {
				return nil
			}
		}
	}

	return fmt.Errorf("permission denied: project user management access required")
}

func invitationExpiry(hours *int) (*time.Time, error) {
	if hours == nil {
		expiresAt := time.Now().Add(defaultInvitationLifetime)
		return &expiresAt, nil
	}
	if *hours < 0 {
		return nil, fmt.Errorf("expiration must not be negative")
	}
	if *hours == 0 {
		return nil, nil
	}
	if *hours > 100000 {
		return nil, fmt.Errorf("expiration hours exceed maximum limit")
	}
	expiresAt := time.Now().Add(time.Duration(*hours) * time.Hour)
	return &expiresAt, nil
}

func invitationInfo(row *ent.Invitation, projectName string) InvitationInfo {
	remaining := 0
	if row.MaxUses > 0 {
		remaining = max(row.MaxUses-row.UsedCount, 0)
	}
	return InvitationInfo{
		ProjectName:   projectName,
		ExpiresAt:     row.ExpiresAt,
		MaxUses:       row.MaxUses,
		UsedCount:     row.UsedCount,
		RemainingUses: remaining,
	}
}

func invitationIsExpired(row *ent.Invitation, now time.Time) bool {
	return row.ExpiresAt != nil && !row.ExpiresAt.After(now)
}

func generateInvitationToken() (string, error) {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate invitation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashInvitationToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
