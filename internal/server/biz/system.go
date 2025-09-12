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

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/log"
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
)

// StoragePolicy represents the storage policy configuration.
type StoragePolicy struct {
	StoreChunks    bool            `json:"store_chunks"`
	CleanupOptions []CleanupOption `json:"cleanup_options"`
}

// CleanupOption represents cleanup configuration for a specific resource type.
type CleanupOption struct {
	ResourceType string `json:"resource_type"`
	Enabled      bool   `json:"enabled"`
	CleanupDays  int    `json:"cleanup_days"`
}

type SystemServiceParams struct {
	fx.In
}

func NewSystemService(params SystemServiceParams) *SystemService {
	return &SystemService{}
}

type SystemService struct{}

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
	client := ent.FromContext(ctx)

	sys, err := client.System.Query().Where(system.KeyEQ(SystemKeySecretKey)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", fmt.Errorf("secret key not found, system may not be initialized")
		}

		return "", fmt.Errorf("failed to get secret key: %w", err)
	}

	return sys.Value, nil
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

	return nil
}

var defaultStoragePolicy = StoragePolicy{
	StoreChunks: false,
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

// StoragePolicy retrieves the storage policy configuration.
func (s *SystemService) StoragePolicy(ctx context.Context) (*StoragePolicy, error) {
	client := ent.FromContext(ctx)

	sys, err := client.System.Query().Where(system.KeyEQ(SystemKeyStoragePolicy)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultStoragePolicy), nil
		}

		return nil, fmt.Errorf("failed to get storage policy: %w", err)
	}

	var policy StoragePolicy
	if err := json.Unmarshal([]byte(sys.Value), &policy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal storage policy: %w", err)
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
