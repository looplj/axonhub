package biz

import (
	"context"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/log"
	"go.uber.org/fx"
	"go.uber.org/zap"
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
)

type SystemServiceParams struct {
	fx.In
}

func NewSystemService(params SystemServiceParams) *SystemService {
	svc := &SystemService{}

	return svc
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
func (s *SystemService) Initialize(ctx context.Context, args *InitializeSystemArgs) error {
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

	tx := ent.FromContext(ctx)

	// Hash the owner password
	hashedPassword, err := HashPassword(args.OwnerPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create the owner user
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

	// Set secret key
	err = s.setSystemValue(ctx, SystemKeySecretKey, secretKey)
	if err != nil {
		return fmt.Errorf("failed to set secret key: %w", err)
	}

	// Set brand name
	err = s.setSystemValue(ctx, SystemKeyBrandName, args.BrandName)
	if err != nil {
		return fmt.Errorf("failed to set brand name: %w", err)
	}

	// Set initialized flag
	err = s.setSystemValue(ctx, SystemKeyInitialized, "true")
	if err != nil {
		return fmt.Errorf("failed to set initialized flag: %w", err)
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
	client := ent.FromContext(ctx)

	sys, err := client.System.Query().Where(system.KeyEQ(SystemKeyStoreChunks)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get store_chunks flag: %w", err)
	}

	return sys.Value == "true", nil
}

// SetStoreChunks sets the store_chunks flag.
func (s *SystemService) SetStoreChunks(ctx context.Context, storeChunks bool) error {
	return s.setSystemValue(ctx, SystemKeyStoreChunks, fmt.Sprintf("%t", storeChunks))
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
		OnConflict().
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create system setting: %w", err)
	}

	return nil
}
