package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

const (
	// SystemKeyInitialized is the key used to store the initialized flag in the system table.
	SystemKeyInitialized = "system_initialized"

	// SystemKeyVersion is the key used to store the version in the system table.
	SystemKeyVersion = "system_version"

	// SystemKeySecretKey is the key used to store the secret key in the system table.
	//
	//nolint:gosec // Not a secret.
	SystemKeySecretKey = "system_jwt_secret_key"

	// SystemKeyBrandName is the key for the brand name.
	SystemKeyBrandName = "system_brand_name"

	// SystemKeyBrandLogo is the key for the brand logo (base64 encoded).
	SystemKeyBrandLogo = "system_brand_logo"

	// SystemKeyStoreChunks is the key used to store the store_chunks flag in the system table.
	// If set to true, the system will store chunks in the database.
	// Default value is false.
	SystemKeyStoreChunks = "requests_store_chunks"

	// SystemKeyStoragePolicy is the key used to store the storage policy configuration.
	// The value is JSON-encoded StoragePolicy struct.
	SystemKeyStoragePolicy = "storage_policy"

	// SystemKeyRetryPolicy is the key used to store the retry policy configuration.
	// The value is JSON-encoded RetryPolicy struct.
	SystemKeyRetryPolicy = "retry_policy"

	// SystemKeyDefaultDataStorage is the key used to store the default data storage ID.
	// If not set, the primary data storage will be used.
	SystemKeyDefaultDataStorage = "default_data_storage_id"

	// SystemKeyOnboarded is the key used to store the onboarding status and version.
	// The value is JSON-encoded OnboardingInfo struct.
	SystemKeyOnboarded = "system_onboarded"

	// SystemKeyModelSettings is the key used to store model-related settings.
	// The value is JSON-encoded SystemModelSettings struct.
	SystemKeyModelSettings = "system_model_settings"

	// SystemKeyChannelSettings is the key used to store channel settings.
	// The value is JSON-encoded SystemChannelSettings struct.
	SystemKeyChannelSettings = "system_channel_settings"

	// SystemKeyGeneralSettings is the key used to store general settings.
	// The value is JSON-encoded SystemGeneralSettings struct.
	SystemKeyGeneralSettings = "system_general_settings"

	// SystemKeyAutoBackupSettings is the key used to store auto backup configuration.
	// The value is JSON-encoded AutoBackupSettings struct.
	SystemKeyAutoBackupSettings = "system_auto_backup_settings"
)

// SystemGeneralSettings represents general system configuration settings.
type SystemGeneralSettings struct {
	// CurrencyCode is the code used for currency display (e.g., USD, RMB).
	CurrencyCode string `json:"currency_code"`
	Timezone     string `json:"timezone"`
}

// BackupFrequency represents how often automatic backups should run.
type BackupFrequency string

const (
	BackupFrequencyDaily   BackupFrequency = "daily"
	BackupFrequencyWeekly  BackupFrequency = "weekly"
	BackupFrequencyMonthly BackupFrequency = "monthly"
)

// BackupTargetType represents the type of backup target.
type BackupTargetType string

const (
	BackupTargetTypeWebDAV BackupTargetType = "webdav"
)

// WebDAVConfig represents WebDAV server configuration for backup.
type WebDAVConfig struct {
	URL             string `json:"url"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	InsecureSkipTLS bool   `json:"insecure_skip_tls"`
	Path            string `json:"path"` // Remote path prefix, e.g., "/backups/axonhub"
}

// AutoBackupSettings represents automatic backup configuration.
type AutoBackupSettings struct {
	// Enabled controls whether automatic backup is active
	Enabled bool `json:"enabled"`
	// Frequency defines how often backups are created
	Frequency BackupFrequency `json:"frequency"`
	// TargetType is the type of backup target (currently only webdav)
	TargetType BackupTargetType `json:"target_type"`
	// WebDAV configuration (used when TargetType is webdav)
	WebDAV *WebDAVConfig `json:"webdav,omitempty"`
	// BackupOptions defines what to include in the backup
	IncludeChannels    bool `json:"include_channels"`
	IncludeModels      bool `json:"include_models"`
	IncludeAPIKeys     bool `json:"include_api_keys"`
	IncludeModelPrices bool `json:"include_model_prices"`
	// RetentionDays defines how many days to keep backups (0 = keep all)
	RetentionDays int `json:"retention_days"`
	// LastBackupAt is the timestamp of the last successful backup
	LastBackupAt *time.Time `json:"last_backup_at,omitempty"`
	// LastBackupError is the error message from the last backup attempt (if any)
	LastBackupError string `json:"last_backup_error,omitempty"`
}

// StoragePolicy represents the storage policy configuration.
type StoragePolicy struct {
	StoreChunks       bool            `json:"store_chunks"`
	StoreRequestBody  bool            `json:"store_request_body"`
	StoreResponseBody bool            `json:"store_response_body"`
	CleanupOptions    []CleanupOption `json:"cleanup_options"`
}

// CleanupOption represents cleanup configuration for a specific resource type.
type CleanupOption struct {
	ResourceType string `json:"resource_type"`
	Enabled      bool   `json:"enabled"`
	CleanupDays  int    `json:"cleanup_days"`
}

const (
	// LoadBalancerStrategyAdaptive is a dynamic load balancer strategy that adapts to the current load.
	LoadBalancerStrategyAdaptive = "adaptive"

	// LoadBalancerStrategyFailover is a deterministic load balancer strategy that fails over to the next available channel based on the weight of the channels.
	LoadBalancerStrategyFailover = "failover"

	// LoadBalancerStrategyCircuitBreaker is a dynamic load balancer strategy that monitors the health of channels and fails over to a backup channel when the primary channel is unhealthy.
	LoadBalancerStrategyCircuitBreaker = "circuit-breaker"
)

// RetryPolicy represents the retry policy configuration.
type RetryPolicy struct {
	// Enabled controls whether retry policy is active
	Enabled bool `json:"enabled"`
	// MaxChannelRetries defines the maximum number of different channels to retry
	MaxChannelRetries int `json:"max_channel_retries"`
	// MaxSingleChannelRetries defines the maximum number of retries for a single channel
	MaxSingleChannelRetries int `json:"max_single_channel_retries"`
	// RetryDelayMs defines the delay between retries in milliseconds
	RetryDelayMs int `json:"retry_delay_ms"`
	// LoadBalancerStrategy defines which channel load balancer strategy to use.
	// Supported values: "adaptive", "failover", "circuit-breaker".
	LoadBalancerStrategy string `json:"load_balancer_strategy"`

	// AutoDisableChannel controls whether to auto-disable a channel when it exceeds the maximum number of retries.
	AutoDisableChannel AutoDisableChannel `json:"auto_disable_channel"`
}

type AutoDisableChannel struct {
	// Enabled controls whether auto-disable channel is active
	Enabled bool `json:"enabled"`

	// Statuses defines the status codes and times to auto-disable a channel
	Statuses []AutoDisableChannelStatus `json:"statuses"`
}

type AutoDisableChannelStatus struct {
	// Status is the HTTP status code to trigger auto-disable.
	Status int `json:"status"`

	// Times is the number of times the status code occurs before auto-disable the channel.
	Times int `json:"times"`
}

// SystemModelSettings represents model-related configuration settings.
type SystemModelSettings struct {
	// FallbackToChannelsOnModelNotFound controls whether to fall back to legacy channel
	// selection when the requested model is not found in AxonHub Model associations.
	// When true, if a model has no associations or doesn't exist, the system will
	// attempt to find enabled channels that support the requested model directly.
	// When false, such requests will return an error instead of falling back.
	FallbackToChannelsOnModelNotFound bool `json:"fallback_to_channels_on_model_not_found"`

	// QueryAllChannelModels controls whether models API returns all models from channels
	// or only configured models (models with explicit Model entity configuration).
	// When true, the models API will return all models supported by enabled channels.
	// When false, only models that have explicit Model entity configuration will be returned.
	QueryAllChannelModels bool `json:"query_all_channel_models"`
}

type SystemChannelSettings struct {
	Probe ChannelProbeSetting `json:"probe"`
}

// ProbeFrequency represents the frequency of channel probing.
type ProbeFrequency string

const (
	ProbeFrequency1Min  ProbeFrequency = "1m"
	ProbeFrequency5Min  ProbeFrequency = "5m"
	ProbeFrequency30Min ProbeFrequency = "30m"
	ProbeFrequency1Hour ProbeFrequency = "1h"
)

// ChannelProbeSetting represents the channel probe configuration.
type ChannelProbeSetting struct {
	// Enabled controls whether channel probing is active
	Enabled bool `json:"enabled"`
	// Frequency defines how often to probe channels
	Frequency ProbeFrequency `json:"frequency"`
}

// GetQueryRangeMinutes returns the query range in minutes based on the probe frequency.
// 1m -> 10min, 5m -> 60min, 30m -> 720min (12h), 1h -> 1440min (24h).
func (c *ChannelProbeSetting) GetQueryRangeMinutes() int {
	switch c.Frequency {
	case ProbeFrequency1Min:
		return 10
	case ProbeFrequency5Min:
		return 60
	case ProbeFrequency30Min:
		return 720
	case ProbeFrequency1Hour:
		return 1440
	default:
		return 10
	}
}

// GetIntervalMinutes returns the interval in minutes based on the probe frequency.
func (c *ChannelProbeSetting) GetIntervalMinutes() int {
	switch c.Frequency {
	case ProbeFrequency1Min:
		return 1
	case ProbeFrequency5Min:
		return 5
	case ProbeFrequency30Min:
		return 30
	case ProbeFrequency1Hour:
		return 60
	default:
		return 1
	}
}

// GetCronExpr returns the cron expression based on the probe frequency.
func (c *ChannelProbeSetting) GetCronExpr() string {
	switch c.Frequency {
	case ProbeFrequency1Min:
		return "* * * * *"
	case ProbeFrequency5Min:
		return "*/5 * * * *"
	case ProbeFrequency30Min:
		return "*/30 * * * *"
	case ProbeFrequency1Hour:
		return "0 * * * *"
	default:
		return "* * * * *"
	}
}

// MarshalGQL implements the graphql.Marshaler interface for ProbeFrequency.
func (p ProbeFrequency) MarshalGQL(w io.Writer) {
	var s string

	switch p {
	case ProbeFrequency1Min:
		s = "ONE_MINUTE"
	case ProbeFrequency5Min:
		s = "FIVE_MINUTES"
	case ProbeFrequency30Min:
		s = "THIRTY_MINUTES"
	case ProbeFrequency1Hour:
		s = "ONE_HOUR"
	default:
		s = "ONE_MINUTE"
	}

	_, _ = io.WriteString(w, `"`+s+`"`)
}

// UnmarshalGQL implements the graphql.Unmarshaler interface for ProbeFrequency.
func (p *ProbeFrequency) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("ProbeFrequency must be a string")
	}

	switch str {
	case "ONE_MINUTE":
		*p = ProbeFrequency1Min
	case "FIVE_MINUTES":
		*p = ProbeFrequency5Min
	case "THIRTY_MINUTES":
		*p = ProbeFrequency30Min
	case "ONE_HOUR":
		*p = ProbeFrequency1Hour
	default:
		return fmt.Errorf("invalid ProbeFrequency: %s", str)
	}

	return nil
}

// OnboardingRecord represents the onboarding status and version information.
type OnboardingRecord struct {
	// Onboarded indicates whether the user has completed onboarding for the system.
	Onboarded bool `json:"onboarded"`
	// Version is the system version when onboarding was completed
	Version string `json:"version"`
	// CompletedAt is the timestamp when onboarding was completed
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// SystemModelSetting tracks the onboarding status for system model configuration
	SystemModelSetting *struct {
		Onboarded   bool       `json:"onboarded"`
		CompletedAt *time.Time `json:"completed_at,omitempty"`
	} `json:"system_model_setting"`
}

type SystemServiceParams struct {
	fx.In

	CacheConfig xcache.Config
	Ent         *ent.Client
}

func NewSystemService(params SystemServiceParams) *SystemService {
	return &SystemService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		CacheConfig: params.CacheConfig,
		Cache:       xcache.NewFromConfig[ent.System](params.CacheConfig),
	}
}

type SystemService struct {
	*AbstractService

	CacheConfig xcache.Config
	Cache       xcache.Cache[ent.System]

	mu           sync.RWMutex
	timeLocation *time.Location
}

func (s *SystemService) IsInitialized(ctx context.Context) (bool, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := s.entFromContext(ctx)

	sys, err := client.System.Query().Where(system.KeyEQ(SystemKeyInitialized)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return strings.EqualFold(sys.Value, "true"), nil
}

type InitializeSystemParams struct {
	OwnerEmail     string
	OwnerPassword  string
	OwnerFirstName string
	OwnerLastName  string
	BrandName      string
}

// Initialize initializes the system with a secret key and sets the initialized flag.
func (s *SystemService) Initialize(ctx context.Context, params *InitializeSystemParams) (err error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	// Check if system is already initialized
	isInitialized, err := s.IsInitialized(ctx)
	if err != nil {
		return fmt.Errorf("failed to check initialization status: %w", err)
	}

	if isInitialized {
		// System is already initialized, nothing to do
		return nil
	}

	secretKey, err := GenerateSecretKey()
	if err != nil {
		return fmt.Errorf("failed to generate secret key: %w", err)
	}

	db := s.entFromContext(ctx)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	ctx = ent.NewContext(ctx, tx.Client())

	hashedPassword, err := HashPassword(params.OwnerPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create owner user.
	user, err := tx.User.Create().
		SetEmail(params.OwnerEmail).
		SetPassword(hashedPassword).
		SetFirstName(params.OwnerFirstName).
		SetLastName(params.OwnerLastName).
		SetIsOwner(true).
		SetScopes([]string{"*"}). // Give owner all scopes
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to create owner user: %w", err)
	}

	log.Info(ctx, "created owner user", zap.Int("user_id", user.ID))

	// Set user in context for project creation
	ctx = contexts.WithUser(ctx, user)
	// Create default project and assign owner
	projectService := NewProjectService(ProjectServiceParams{})
	projectInput := ent.CreateProjectInput{
		Name:        "Default",
		Description: lo.ToPtr("Default project"),
	}

	_, err = projectService.CreateProject(ctx, projectInput)
	if err != nil {
		return fmt.Errorf("failed to create default project: %w", err)
	}

	log.Info(ctx, "created default project", zap.String("slug", "default"))

	// Set secret key.
	err = s.setSystemValue(ctx, SystemKeySecretKey, secretKey)
	if err != nil {
		return fmt.Errorf("failed to set secret key: %w", err)
	}

	// Set brand name.
	err = s.setSystemValue(ctx, SystemKeyBrandName, params.BrandName)
	if err != nil {
		return fmt.Errorf("failed to set brand name: %w", err)
	}

	// Create primary data storage
	primaryDataStorage, err := tx.DataStorage.Create().
		SetName("Primary").
		SetDescription("Primary database storage").
		SetPrimary(true).
		SetType("database").
		SetSettings(&objects.DataStorageSettings{}).
		SetStatus("active").
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to create primary data storage: %w", err)
	}

	// Set default data storage ID.
	err = s.SetDefaultDataStorageID(ctx, primaryDataStorage.ID)
	if err != nil {
		return fmt.Errorf("failed to set default data storage ID: %w", err)
	}

	log.Info(ctx, "created primary data storage", zap.Int("data_storage_id", primaryDataStorage.ID))

	// Set initialized flag to true.
	err = s.setSystemValue(ctx, SystemKeyInitialized, "true")
	if err != nil {
		return fmt.Errorf("failed to set initialized flag: %w", err)
	}

	// Record current build version for initialized system.
	err = s.SetVersion(ctx, build.Version)
	if err != nil {
		return fmt.Errorf("failed to set system version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SecretKey retrieves the JWT secret key from system settings.
func (s *SystemService) SecretKey(ctx context.Context) (string, error) {
	value, err := s.getSystemValue(ctx, SystemKeySecretKey)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", fmt.Errorf("secret key not found, system may not be initialized")
		}

		return "", fmt.Errorf("failed to get secret key: %w", err)
	}

	return value, nil
}

// SetSecretKey sets a new JWT secret key.
func (s *SystemService) SetSecretKey(ctx context.Context, secretKey string) error {
	return s.setSystemValue(ctx, SystemKeySecretKey, secretKey)
}

// StoreChunks retrieves the store_chunks flag.
func (s *SystemService) StoreChunks(ctx context.Context) (bool, error) {
	policy, err := s.StoragePolicy(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get storage policy: %w", err)
	}

	return policy.StoreChunks, nil
}

// BrandName retrieves the brand name.
func (s *SystemService) BrandName(ctx context.Context) (string, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := s.entFromContext(ctx)

	sys, err := client.System.Query().Where(system.KeyEQ(SystemKeyBrandName)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("failed to get brand name: %w", err)
	}

	return sys.Value, nil
}

// SetBrandName sets the brand name.
func (s *SystemService) SetBrandName(ctx context.Context, brandName string) error {
	return s.setSystemValue(ctx, SystemKeyBrandName, brandName)
}

// BrandLogo retrieves the brand logo (base64 encoded).
func (s *SystemService) BrandLogo(ctx context.Context) (string, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := s.entFromContext(ctx)

	sys, err := client.System.Query().Where(system.KeyEQ(SystemKeyBrandLogo)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("failed to get brand logo: %w", err)
	}

	return sys.Value, nil
}

// SetBrandLogo sets the brand logo (base64 encoded).
func (s *SystemService) SetBrandLogo(ctx context.Context, brandLogo string) error {
	return s.setSystemValue(ctx, SystemKeyBrandLogo, brandLogo)
}

func (s *SystemService) getSystemValue(ctx context.Context, key string) (string, error) {
	cacheKey := "system:" + key
	if v, err := s.Cache.Get(ctx, cacheKey); err == nil {
		return v.Value, nil
	}

	client := s.entFromContext(ctx)

	sys, err := client.System.Query().Where(system.KeyEQ(key)).Only(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get system value: %w", err)
	}

	_ = s.Cache.Set(ctx, cacheKey, *sys)

	return sys.Value, nil
}

// setSystemValue sets or updates a system key-value pair.
func (s *SystemService) setSystemValue(
	ctx context.Context,
	key, value string,
) error {
	client := s.entFromContext(ctx)

	err := client.System.Create().
		SetKey(key).
		SetValue(value).
		OnConflict(sql.ConflictColumns("key")).
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create system setting: %w", err)
	}

	// Invalidate cache for this key
	if err := s.Cache.Delete(ctx, "system:"+key); err != nil {
		log.Warn(ctx, "failed to invalidate cache", log.String("key", key), log.Cause(err))
	}

	return nil
}

var defaultStoragePolicy = StoragePolicy{
	StoreChunks:       false,
	StoreRequestBody:  true,
	StoreResponseBody: true,
	CleanupOptions: []CleanupOption{
		{
			ResourceType: "requests",
			Enabled:      false,
			CleanupDays:  3,
		},
		{
			ResourceType: "usage_logs",
			Enabled:      false,
			CleanupDays:  30,
		},
	},
}

var defaultRetryPolicy = RetryPolicy{
	MaxChannelRetries:       3,
	MaxSingleChannelRetries: 2,
	RetryDelayMs:            1000,
	LoadBalancerStrategy:    "adaptive",
	Enabled:                 true,
}

var defaultModelSettings = SystemModelSettings{
	FallbackToChannelsOnModelNotFound: true,
	QueryAllChannelModels:             true,
}

var defaultChannelSetting = SystemChannelSettings{
	Probe: ChannelProbeSetting{
		Enabled:   true,
		Frequency: ProbeFrequency5Min,
	},
}

var defaultGeneralSettings = SystemGeneralSettings{
	CurrencyCode: "USD",
	Timezone:     "UTC",
}

var defaultAutoBackupSettings = AutoBackupSettings{
	Enabled:            false,
	Frequency:          BackupFrequencyDaily,
	TargetType:         BackupTargetTypeWebDAV,
	IncludeChannels:    true,
	IncludeModels:      true,
	IncludeAPIKeys:     false,
	IncludeModelPrices: true,
	RetentionDays:      30,
}

// StoragePolicy retrieves the storage policy configuration.
func (s *SystemService) StoragePolicy(ctx context.Context) (*StoragePolicy, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	value, err := s.getSystemValue(ctx, SystemKeyStoragePolicy)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultStoragePolicy), nil
		}

		return nil, fmt.Errorf("failed to get storage policy: %w", err)
	}

	var policy StoragePolicy
	if err := json.Unmarshal([]byte(value), &policy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal storage policy: %w", err)
	}

	// Backward compatibility: if new keys are absent in stored JSON, default them to true
	if !strings.Contains(value, "\"store_request_body\"") {
		policy.StoreRequestBody = true
	}

	if !strings.Contains(value, "\"store_response_body\"") {
		policy.StoreResponseBody = true
	}

	return &policy, nil
}

// SetStoragePolicy sets the storage policy configuration.
func (s *SystemService) SetStoragePolicy(ctx context.Context, policy *StoragePolicy) error {
	jsonBytes, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal storage policy: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyStoragePolicy, string(jsonBytes))
}

// RetryPolicy retrieves the retry policy configuration.
func (s *SystemService) RetryPolicy(ctx context.Context) (*RetryPolicy, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	value, err := s.getSystemValue(ctx, SystemKeyRetryPolicy)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultRetryPolicy), nil
		}

		return nil, fmt.Errorf("failed to get retry policy: %w", err)
	}

	var policy RetryPolicy
	if err := json.Unmarshal([]byte(value), &policy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal retry policy: %w", err)
	}

	if policy.LoadBalancerStrategy == "" {
		policy.LoadBalancerStrategy = defaultRetryPolicy.LoadBalancerStrategy
	}
	// The weighted load balancer strategy is deprecated. Use the failover strategy instead.
	if policy.LoadBalancerStrategy == "weighted" {
		policy.LoadBalancerStrategy = LoadBalancerStrategyFailover
	}

	return &policy, nil
}

func (s *SystemService) RetryPolicyOrDefault(ctx context.Context) *RetryPolicy {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	policy, err := s.RetryPolicy(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultRetryPolicy)
		}

		log.Warn(ctx, "failed to get retry policy", log.Cause(err))

		return lo.ToPtr(defaultRetryPolicy)
	}

	return policy
}

// SetRetryPolicy sets the retry policy configuration.
func (s *SystemService) SetRetryPolicy(ctx context.Context, policy *RetryPolicy) error {
	if policy.LoadBalancerStrategy == "" {
		policy.LoadBalancerStrategy = defaultRetryPolicy.LoadBalancerStrategy
	}

	jsonBytes, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal retry policy: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyRetryPolicy, string(jsonBytes))
}

// ModelSettings retrieves the model settings configuration.
func (s *SystemService) ModelSettings(ctx context.Context) (*SystemModelSettings, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	value, err := s.getSystemValue(ctx, SystemKeyModelSettings)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultModelSettings), nil
		}

		return nil, fmt.Errorf("failed to get model settings: %w", err)
	}

	var settings SystemModelSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal model settings: %w", err)
	}

	return &settings, nil
}

// ModelSettingsOrDefault retrieves the model settings or returns the default if not available.
func (s *SystemService) ModelSettingsOrDefault(ctx context.Context) *SystemModelSettings {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	settings, err := s.ModelSettings(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultModelSettings)
		}

		log.Warn(ctx, "failed to get model settings", log.Cause(err))

		return lo.ToPtr(defaultModelSettings)
	}

	return settings
}

// SetModelSettings sets the model settings configuration.
func (s *SystemService) SetModelSettings(ctx context.Context, settings SystemModelSettings) error {
	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal model settings: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyModelSettings, string(jsonBytes))
}

// ChannelSetting retrieves the channel setting configuration.
func (s *SystemService) ChannelSetting(ctx context.Context) (*SystemChannelSettings, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	value, err := s.getSystemValue(ctx, SystemKeyChannelSettings)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultChannelSetting), nil
		}

		return nil, fmt.Errorf("failed to get channel setting: %w", err)
	}

	var setting SystemChannelSettings
	if err := json.Unmarshal([]byte(value), &setting); err != nil {
		return nil, fmt.Errorf("failed to unmarshal channel setting: %w", err)
	}

	return &setting, nil
}

// ChannelSettingOrDefault retrieves the channel setting or returns the default if not available.
func (s *SystemService) ChannelSettingOrDefault(ctx context.Context) *SystemChannelSettings {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	setting, err := s.ChannelSetting(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultChannelSetting)
		}

		log.Warn(ctx, "failed to get channel setting", log.Cause(err))

		return lo.ToPtr(defaultChannelSetting)
	}

	return setting
}

// SetChannelSetting sets the channel setting configuration.
func (s *SystemService) SetChannelSetting(ctx context.Context, setting SystemChannelSettings) error {
	jsonBytes, err := json.Marshal(setting)
	if err != nil {
		return fmt.Errorf("failed to marshal channel setting: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyChannelSettings, string(jsonBytes))
}

func (s *SystemService) TimeLocation(ctx context.Context) *time.Location {
	s.mu.RLock()

	if s.timeLocation != nil {
		defer s.mu.RUnlock()
		return s.timeLocation
	}

	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check
	if s.timeLocation != nil {
		return s.timeLocation
	}

	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	settings, err := s.GeneralSettings(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			s.timeLocation = time.UTC
			return time.UTC
		}

		log.Warn(ctx, "failed to get general settings", log.Cause(err))

		return time.UTC
	}

	if settings.Timezone == "" {
		s.timeLocation = time.UTC
		return time.UTC
	}

	if l, err := time.LoadLocation(settings.Timezone); err == nil {
		s.timeLocation = l
		return l
	}

	s.timeLocation = time.UTC

	return time.UTC
}

// GeneralSettings retrieves the general settings configuration.
func (s *SystemService) GeneralSettings(ctx context.Context) (*SystemGeneralSettings, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	value, err := s.getSystemValue(ctx, SystemKeyGeneralSettings)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultGeneralSettings), nil
		}

		return nil, fmt.Errorf("failed to get general settings: %w", err)
	}

	var settings SystemGeneralSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal general settings: %w", err)
	}

	if settings.CurrencyCode == "" {
		settings.CurrencyCode = defaultGeneralSettings.CurrencyCode
	}

	if settings.Timezone == "" {
		settings.Timezone = defaultGeneralSettings.Timezone
	}

	return &settings, nil
}

// SetGeneralSettings sets the general settings configuration.
func (s *SystemService) SetGeneralSettings(ctx context.Context, settings SystemGeneralSettings) error {
	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal general settings: %w", err)
	}

	err = s.setSystemValue(ctx, SystemKeyGeneralSettings, string(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to set general settings: %w", err)
	}

	s.mu.Lock()
	s.timeLocation = nil
	s.mu.Unlock()

	return nil
}

// DefaultDataStorageID retrieves the default data storage ID from system settings.
// Returns 0 if not set.
func (s *SystemService) DefaultDataStorageID(ctx context.Context) (int, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	value, err := s.getSystemValue(ctx, SystemKeyDefaultDataStorage)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, nil
		}

		return 0, fmt.Errorf("failed to get default data storage ID: %w", err)
	}

	var id int
	if _, err := fmt.Sscanf(value, "%d", &id); err != nil {
		return 0, fmt.Errorf("failed to parse default data storage ID: %w", err)
	}

	return id, nil
}

// SetDefaultDataStorageID sets the default data storage ID.
func (s *SystemService) SetDefaultDataStorageID(ctx context.Context, id int) error {
	return s.setSystemValue(ctx, SystemKeyDefaultDataStorage, fmt.Sprintf("%d", id))
}

// Version retrieves the system version from system settings.
// Returns empty string if not set.
func (s *SystemService) Version(ctx context.Context) (string, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	value, err := s.getSystemValue(ctx, SystemKeyVersion)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("failed to get system version: %w", err)
	}

	return value, nil
}

// SetVersion sets the system version.
func (s *SystemService) SetVersion(ctx context.Context, version string) error {
	return s.setSystemValue(ctx, SystemKeyVersion, version)
}

// OnboardingInfo retrieves the onboarding information from system settings.
// Returns nil if not set.
func (s *SystemService) OnboardingInfo(ctx context.Context) (*OnboardingRecord, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	value, err := s.getSystemValue(ctx, SystemKeyOnboarded)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get onboarding info: %w", err)
	}

	var info OnboardingRecord
	if err := json.Unmarshal([]byte(value), &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal onboarding info: %w", err)
	}

	return &info, nil
}

// SetOnboardingInfo sets the onboarding information.
func (s *SystemService) SetOnboardingInfo(ctx context.Context, info *OnboardingRecord) error {
	jsonBytes, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal onboarding info: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyOnboarded, string(jsonBytes))
}

// IsOnboardingCompleted checks if onboarding has been completed for the current version.
func (s *SystemService) IsOnboardingCompleted(ctx context.Context) (bool, error) {
	info, err := s.OnboardingInfo(ctx)
	if err != nil {
		return false, err
	}

	if info == nil || !info.Onboarded {
		return false, nil
	}

	currentVersion, err := s.Version(ctx)
	if err != nil {
		return false, err
	}

	// If onboarding was completed for a different version, it needs to be redone
	return info.Version == currentVersion, nil
}

// CompleteOnboarding marks onboarding as completed for the current version.
func (s *SystemService) CompleteOnboarding(ctx context.Context) error {
	currentVersion, err := s.Version(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	existingInfo, err := s.OnboardingInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing onboarding info: %w", err)
	}

	info := &OnboardingRecord{
		Onboarded:   true,
		Version:     currentVersion,
		CompletedAt: lo.ToPtr(time.Now()),
	}

	if existingInfo != nil && existingInfo.SystemModelSetting != nil {
		info.SystemModelSetting = existingInfo.SystemModelSetting
	}

	return s.SetOnboardingInfo(ctx, info)
}

// CompleteSystemModelSettingOnboarding marks system model setting onboarding as completed.
func (s *SystemService) CompleteSystemModelSettingOnboarding(ctx context.Context) error {
	existingInfo, err := s.OnboardingInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing onboarding info: %w", err)
	}

	info := &OnboardingRecord{}
	if existingInfo != nil {
		info.Onboarded = existingInfo.Onboarded
		info.Version = existingInfo.Version
		info.CompletedAt = existingInfo.CompletedAt
	}

	now := time.Now()
	info.SystemModelSetting = &struct {
		Onboarded   bool       `json:"onboarded"`
		CompletedAt *time.Time `json:"completed_at,omitempty"`
	}{
		Onboarded:   true,
		CompletedAt: &now,
	}

	return s.SetOnboardingInfo(ctx, info)
}

// VersionCheckResult contains the result of a version check.
type VersionCheckResult struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	ReleaseURL     string `json:"release_url"`
}

// CheckForUpdate checks if there is a newer version available on GitHub.
func (s *SystemService) CheckForUpdate(ctx context.Context) (*VersionCheckResult, error) {
	currentVersion := build.Version

	latestVersion, err := s.fetchLatestGitHubRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	hasUpdate := s.isNewerVersion(currentVersion, latestVersion)
	releaseURL := fmt.Sprintf("https://github.com/looplj/axonhub/releases/tag/%s", latestVersion)

	return &VersionCheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		HasUpdate:      hasUpdate,
		ReleaseURL:     releaseURL,
	}, nil
}

// fetchLatestGitHubRelease fetches the latest stable release tag from GitHub.
// It skips beta and rc versions.
func (s *SystemService) fetchLatestGitHubRelease(ctx context.Context) (string, error) {
	return FetchLatestGitHubRelease(ctx)
}

// isNewerVersion compares two semantic versions and returns true if latest is newer than current.
func (s *SystemService) isNewerVersion(current, latest string) bool {
	return IsNewerVersion(current, latest)
}

// AutoBackupSettings retrieves the auto backup settings configuration.
func (s *SystemService) AutoBackupSettings(ctx context.Context) (*AutoBackupSettings, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	value, err := s.getSystemValue(ctx, SystemKeyAutoBackupSettings)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultAutoBackupSettings), nil
		}

		return nil, fmt.Errorf("failed to get auto backup settings: %w", err)
	}

	var settings AutoBackupSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auto backup settings: %w", err)
	}

	return &settings, nil
}

// SetAutoBackupSettings sets the auto backup settings configuration.
func (s *SystemService) SetAutoBackupSettings(ctx context.Context, settings AutoBackupSettings) error {
	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal auto backup settings: %w", err)
	}

	err = s.setSystemValue(ctx, SystemKeyAutoBackupSettings, string(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to set auto backup settings: %w", err)
	}

	return nil
}

// UpdateAutoBackupLastRun updates the last backup timestamp and error status.
func (s *SystemService) UpdateAutoBackupLastRun(ctx context.Context, lastBackupAt time.Time, lastError string) error {
	settings, err := s.AutoBackupSettings(ctx)
	if err != nil {
		return err
	}

	settings.LastBackupAt = &lastBackupAt
	settings.LastBackupError = lastError

	return s.SetAutoBackupSettings(ctx, *settings)
}
