package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"go.uber.org/fx"
	"go.uber.org/zap"

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
)

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

// RetryPolicy represents the retry policy configuration.
type RetryPolicy struct {
	// MaxChannelRetries defines the maximum number of different channels to retry
	MaxChannelRetries int `json:"max_channel_retries"`
	// MaxSingleChannelRetries defines the maximum number of retries for a single channel
	MaxSingleChannelRetries int `json:"max_single_channel_retries"`
	// RetryDelayMs defines the delay between retries in milliseconds
	RetryDelayMs int `json:"retry_delay_ms"`
	// Enabled controls whether retry policy is active
	Enabled bool `json:"enabled"`
}

type SystemServiceParams struct {
	fx.In

	CacheConfig xcache.Config
}

func NewSystemService(params SystemServiceParams) *SystemService {
	return &SystemService{Cache: xcache.NewFromConfig[ent.System](params.CacheConfig)}
}

type SystemService struct {
	Cache xcache.Cache[ent.System]
}

func (s *SystemService) IsInitialized(ctx context.Context) (bool, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	sys, err := client.System.Query().Where(system.KeyEQ(SystemKeyInitialized)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return strings.EqualFold(sys.Value, "true"), nil
}

type InitializeSystemArgs struct {
	OwnerEmail     string
	OwnerPassword  string
	OwnerFirstName string
	OwnerLastName  string
	BrandName      string
}

// Initialize initializes the system with a secret key and sets the initialized flag.
func (s *SystemService) Initialize(ctx context.Context, args *InitializeSystemArgs) (err error) {
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

	db := ent.FromContext(ctx)

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

	hashedPassword, err := HashPassword(args.OwnerPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create owner user.
	user, err := tx.User.Create().
		SetEmail(args.OwnerEmail).
		SetPassword(hashedPassword).
		SetFirstName(args.OwnerFirstName).
		SetLastName(args.OwnerLastName).
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
	err = s.setSystemValue(ctx, SystemKeyBrandName, args.BrandName)
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
	client := ent.FromContext(ctx)

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
	client := ent.FromContext(ctx)

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

	client := ent.FromContext(ctx)

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
	client := ent.FromContext(ctx)

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
	Enabled:                 true,
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
	jsonBytes, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal retry policy: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyRetryPolicy, string(jsonBytes))
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
