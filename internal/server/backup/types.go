package backup

import (
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

type BackupData struct {
	Version            string                     `json:"version"`
	Timestamp          time.Time                  `json:"timestamp"`
	Projects           []*BackupProject           `json:"projects,omitempty"`
	Channels           []*BackupChannel           `json:"channels"`
	Models             []*BackupModel             `json:"models"`
	ChannelModelPrices []*BackupChannelModelPrice `json:"channel_model_prices,omitempty"`
	APIKeys            []*BackupAPIKey            `json:"api_keys,omitempty"`
	Users              []*BackupUser              `json:"users,omitempty"`
	Roles              []*BackupRole              `json:"roles,omitempty"`
	UserProjects       []*BackupUserProject       `json:"user_projects,omitempty"`
	UserRoles          []*BackupUserRole          `json:"user_roles,omitempty"`
	SystemConfig       []BackupSystemConfig       `json:"system_config,omitempty"`
}

type BackupProject struct {
	ent.Project
}

type BackupChannel struct {
	ent.Channel

	Credentials objects.ChannelCredentials `json:"credentials"`
}

type BackupModel struct {
	ent.Model
}

type BackupAPIKey struct {
	ent.APIKey

	ProjectName string `json:"project_name"`
}

type BackupChannelModelPrice struct {
	ChannelName string             `json:"channel_name"`
	ModelID     string             `json:"model_id"`
	Price       objects.ModelPrice `json:"price"`
	ReferenceID string             `json:"reference_id"`
}

const (
	BackupVersion   = "1.1"
	BackupVersionV1 = "1.0"
)

type BackupOptions struct {
	IncludeProjects    bool
	IncludeChannels    bool
	IncludeModels      bool
	IncludeAPIKeys     bool
	IncludeModelPrices bool
}

type ConflictStrategy string

const (
	ConflictStrategySkip      ConflictStrategy = "skip"
	ConflictStrategyOverwrite ConflictStrategy = "overwrite"
	ConflictStrategyError     ConflictStrategy = "error"
)

type RestoreOptions struct {
	IncludeProjects            bool
	IncludeChannels            bool
	IncludeModels              bool
	IncludeAPIKeys             bool
	IncludeModelPrices         bool
	ProjectConflictStrategy    ConflictStrategy
	ChannelConflictStrategy    ConflictStrategy
	ModelConflictStrategy      ConflictStrategy
	ModelPriceConflictStrategy ConflictStrategy
	APIKeyConflictStrategy     ConflictStrategy
}

type BackupUser struct {
	ID             int       `json:"id"`
	Email          string    `json:"email"`
	Status         string    `json:"status"`
	PreferLanguage string    `json:"prefer_language"`
	Password       string    `json:"password"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Avatar         string    `json:"avatar"`
	IsOwner        bool      `json:"is_owner"`
	Scopes         []string  `json:"scopes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BackupRole struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Level     string   `json:"level"`
	ProjectID *int     `json:"project_id,omitempty"`
	Scopes    []string `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BackupUserProject struct {
	ID        int      `json:"id"`
	UserID    int      `json:"user_id"`
	ProjectID int      `json:"project_id"`
	IsOwner   bool     `json:"is_owner"`
	Scopes    []string `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BackupUserRole struct {
	ID        int        `json:"id"`
	UserID    int        `json:"user_id"`
	RoleID    int        `json:"role_id"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type BackupSystemConfig struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
