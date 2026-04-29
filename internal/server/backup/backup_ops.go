package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/system"
)

func (svc *BackupService) Backup(ctx context.Context, opts BackupOptions) ([]byte, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	if !user.IsOwner {
		return nil, fmt.Errorf("only owners can perform backup operations")
	}

	return svc.doBackup(ctx, opts)
}

// BackupWithoutAuth performs backup without user authentication check.
// This is used by the auto-backup scheduler which runs in a privileged context.
func (svc *BackupService) BackupWithoutAuth(ctx context.Context, opts BackupOptions) ([]byte, error) {
	return svc.doBackup(ctx, opts)
}

func (svc *BackupService) doBackup(ctx context.Context, opts BackupOptions) ([]byte, error) {
	var (
		projectDataList           []*BackupProject
		channelDataList           []*BackupChannel
		channelModelPriceDataList []*BackupChannelModelPrice
	)

	if opts.IncludeProjects {
		projects, err := svc.db.Project.Query().All(ctx)
		if err != nil {
			return nil, err
		}

		projectDataList = lo.Map(projects, func(proj *ent.Project, _ int) *BackupProject {
			return &BackupProject{Project: *proj}
		})
	}

	if opts.IncludeChannels {
		channels, err := svc.db.Channel.Query().All(ctx)
		if err != nil {
			return nil, err
		}

		channelDataList = lo.Map(channels, func(ch *ent.Channel, _ int) *BackupChannel {
			return &BackupChannel{
				Channel:     *ch,
				Credentials: ch.Credentials,
			}
		})
	}

	if opts.IncludeModelPrices {
		prices, err := svc.db.ChannelModelPrice.Query().
			WithChannel().
			All(ctx)
		if err != nil {
			return nil, err
		}

		channelModelPriceDataList = lo.FilterMap(prices, func(p *ent.ChannelModelPrice, _ int) (*BackupChannelModelPrice, bool) {
			if p.Edges.Channel == nil {
				return nil, false
			}

			return &BackupChannelModelPrice{
				ChannelName: p.Edges.Channel.Name,
				ModelID:     p.ModelID,
				Price:       p.Price,
				ReferenceID: p.ReferenceID,
			}, true
		})
	}

	var modelDataList []*BackupModel

	if opts.IncludeModels {
		models, err := svc.db.Model.Query().All(ctx)
		if err != nil {
			return nil, err
		}

		modelDataList = lo.Map(models, func(m *ent.Model, _ int) *BackupModel {
			return &BackupModel{
				Model: *m,
			}
		})
	}

	var apiKeyDataList []*BackupAPIKey

	if opts.IncludeAPIKeys {
		apiKeys, err := svc.db.APIKey.Query().WithProject().All(ctx)
		if err != nil {
			return nil, err
		}

		apiKeyDataList = lo.Map(apiKeys, func(ak *ent.APIKey, _ int) *BackupAPIKey {
			projectName := ""
			if ak.Edges.Project != nil {
				projectName = ak.Edges.Project.Name
			}

			return &BackupAPIKey{
				APIKey:      *ak,
				ProjectName: projectName,
			}
		})
	}

	// F-D26: Include Users, Roles, UserProjects, UserRoles, SystemConfig in backup.
	userDataList, err := svc.collectUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect users: %w", err)
	}

	roleDataList, err := svc.collectRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect roles: %w", err)
	}

	userProjectDataList, err := svc.collectUserProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect user projects: %w", err)
	}

	userRoleDataList, err := svc.collectUserRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect user roles: %w", err)
	}

	systemConfigList, err := svc.collectSystemConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect system config: %w", err)
	}

	backupData := &BackupData{
		Version:            BackupVersion,
		Timestamp:          time.Now(),
		Projects:           projectDataList,
		Channels:           channelDataList,
		Models:             modelDataList,
		ChannelModelPrices: channelModelPriceDataList,
		APIKeys:            apiKeyDataList,
		Users:              userDataList,
		Roles:              roleDataList,
		UserProjects:       userProjectDataList,
		UserRoles:          userRoleDataList,
		SystemConfig:       systemConfigList,
	}

	return json.MarshalIndent(backupData, "", "  ")
}

// collectUsers serializes all active users including password hashes and status.
func (svc *BackupService) collectUsers(ctx context.Context) ([]*BackupUser, error) {
	users, err := svc.db.User.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	return lo.Map(users, func(u *ent.User, _ int) *BackupUser {
		return &BackupUser{
			ID:             u.ID,
			Email:          u.Email,
			Status:         string(u.Status),
			PreferLanguage: u.PreferLanguage,
			Password:       u.Password,
			FirstName:      u.FirstName,
			LastName:       u.LastName,
			Avatar:         u.Avatar,
			IsOwner:        u.IsOwner,
			Scopes:         u.Scopes,
			CreatedAt:      u.CreatedAt,
			UpdatedAt:      u.UpdatedAt,
		}
	}), nil
}

// collectRoles serializes all roles including permissions/scopes.
func (svc *BackupService) collectRoles(ctx context.Context) ([]*BackupRole, error) {
	roles, err := svc.db.Role.Query().Where(role.DeletedAt(0)).All(ctx)
	if err != nil {
		return nil, err
	}

	return lo.Map(roles, func(r *ent.Role, _ int) *BackupRole {
		return &BackupRole{
			ID:        r.ID,
			Name:      r.Name,
			Level:     string(r.Level),
			ProjectID: r.ProjectID,
			Scopes:    r.Scopes,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		}
	}), nil
}

// collectUserProjects serializes user-project associations.
func (svc *BackupService) collectUserProjects(ctx context.Context) ([]*BackupUserProject, error) {
	userProjects, err := svc.db.UserProject.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	return lo.Map(userProjects, func(up *ent.UserProject, _ int) *BackupUserProject {
		return &BackupUserProject{
			ID:        up.ID,
			UserID:    up.UserID,
			ProjectID: up.ProjectID,
			IsOwner:   up.IsOwner,
			Scopes:    up.Scopes,
			CreatedAt: up.CreatedAt,
			UpdatedAt: up.UpdatedAt,
		}
	}), nil
}

// collectUserRoles serializes user-role associations.
func (svc *BackupService) collectUserRoles(ctx context.Context) ([]*BackupUserRole, error) {
	userRoles, err := svc.db.UserRole.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	return lo.Map(userRoles, func(ur *ent.UserRole, _ int) *BackupUserRole {
		return &BackupUserRole{
			ID:        ur.ID,
			UserID:    ur.UserID,
			RoleID:    ur.RoleID,
			CreatedAt: ur.CreatedAt,
			UpdatedAt: ur.UpdatedAt,
		}
	}), nil
}

// collectSystemConfig serializes all system key-value config pairs.
func (svc *BackupService) collectSystemConfig(ctx context.Context) ([]BackupSystemConfig, error) {
	entries, err := svc.db.System.Query().
		Where(system.KeyNEQ(biz.SystemKeySecretKey)). // Exclude JWT secret key from backup for security.
		All(ctx)
	if err != nil {
		return nil, err
	}

	return lo.Map(entries, func(s *ent.System, _ int) BackupSystemConfig {
		return BackupSystemConfig{
			Key:   s.Key,
			Value: s.Value,
		}
	}), nil
}
