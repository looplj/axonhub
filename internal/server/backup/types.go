package backup

import (
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

type BackupData struct {
	Version            string                     `json:"version"`
	Timestamp          time.Time                  `json:"timestamp"`
	Channels           []*BackupChannel           `json:"channels"`
	Models             []*BackupModel             `json:"models"`
	ChannelModelPrices []*BackupChannelModelPrice `json:"channel_model_prices,omitempty"`
	APIKeys            []*BackupAPIKey            `json:"api_keys,omitempty"`
}

type BackupChannel struct {
	ent.Channel

	Credentials *objects.ChannelCredentials `json:"credentials,omitempty"`
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
	IncludeChannels            bool
	IncludeModels              bool
	IncludeAPIKeys             bool
	IncludeModelPrices         bool
	ChannelConflictStrategy    ConflictStrategy
	ModelConflictStrategy      ConflictStrategy
	ModelPriceConflictStrategy ConflictStrategy
	APIKeyConflictStrategy     ConflictStrategy
}
