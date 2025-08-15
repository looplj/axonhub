package biz

import (
	"context"
	"fmt"
	"strings"

	"entgo.io/ent/privacy"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/log"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	// SystemKeyInitialized is the key used to store the initialized flag in the system table.
	SystemKeyInitialized = "initialized"
	// SystemKeySecretKey is the key used to store the secret key in the system table.
	SystemKeySecretKey = "secret_key"

	// SystemKeyStoreChunks is the key used to store the store_chunks flag in the system table.
	// If set to true, the system will store chunks in the database.
	// Default value is false.
	SystemKeyStoreChunks = "store_chunks"

	// SystemKeyBrandName is the key for the brand name.
	SystemKeyBrandName = "brand_name"
)

type SystemServiceParams struct {
	fx.In

	Ent *ent.Client
}

func NewSystemService(params SystemServiceParams) *SystemService {
	svc := &SystemService{
		Ent: params.Ent,
	}

	return svc
}

type SystemService struct {
	Ent *ent.Client
}

func (s *SystemService) IsInitialized(ctx context.Context) (bool, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	sys, err := s.Ent.System.Query().Where(system.KeyEQ(SystemKeyInitialized)).Only(ctx)
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

	// Start a transaction
	tx, err := s.Ent.Tx(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tErr := tx.Rollback()
			if tErr != nil {
				log.Error(ctx, "failed to rollback transaction", zap.Error(tErr))
			}
		}
	}()
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
	err = s.setSystemValue(ctx, tx.System, SystemKeySecretKey, secretKey)
	if err != nil {
		return fmt.Errorf("failed to set secret key: %w", err)
	}

	// Set brand name
	err = s.setSystemValue(ctx, tx.System, SystemKeyBrandName, args.BrandName)
	if err != nil {
		return fmt.Errorf("failed to set brand name: %w", err)
	}

	// Set initialized flag
	err = s.setSystemValue(ctx, tx.System, SystemKeyInitialized, "true")
	if err != nil {
		return fmt.Errorf("failed to set initialized flag: %w", err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SecretKey retrieves the JWT secret key from system settings.
func (s *SystemService) SecretKey(ctx context.Context) (string, error) {
	sys, err := s.Ent.System.Query().Where(system.KeyEQ(SystemKeySecretKey)).Only(ctx)
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
	return s.setSystemValue(ctx, s.Ent.System, SystemKeySecretKey, secretKey)
}

// StoreChunks retrieves the store_chunks flag.
func (s *SystemService) StoreChunks(ctx context.Context) (bool, error) {
	sys, err := s.Ent.System.Query().Where(system.KeyEQ(SystemKeyStoreChunks)).Only(ctx)
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
	return s.setSystemValue(ctx, s.Ent.System, SystemKeyStoreChunks, fmt.Sprintf("%t", storeChunks))
}

// BrandName retrieves the brand name.
func (s *SystemService) BrandName(ctx context.Context) (string, error) {
	sys, err := s.Ent.System.Query().Where(system.KeyEQ(SystemKeyBrandName)).Only(ctx)
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
	return s.setSystemValue(ctx, s.Ent.System, SystemKeyBrandName, brandName)
}

// setSystemValue sets or updates a system key-value pair.
func (s *SystemService) setSystemValue(
	ctx context.Context,
	client interface{},
	key, value string,
) error {
	switch c := client.(type) {
	case *ent.SystemClient:
		// Check if record exists
		existing, err := c.Query().Where(system.KeyEQ(key)).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				// Create new record
				_, err = c.Create().
					SetKey(key).
					SetValue(value).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to create system setting: %w", err)
				}

				return nil
			}

			return fmt.Errorf("failed to query system setting: %w", err)
		}

		// Update existing record
		_, err = c.UpdateOneID(existing.ID).
			SetValue(value).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update system setting: %w", err)
		}

	case *ent.Tx:
		// Check if record exists
		existing, err := c.System.Query().Where(system.KeyEQ(key)).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				// Create new record
				_, err = c.System.Create().
					SetKey(key).
					SetValue(value).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to create system setting: %w", err)
				}

				return nil
			}

			return fmt.Errorf("failed to query system setting: %w", err)
		}

		// Update existing record
		_, err = c.System.UpdateOneID(existing.ID).
			SetValue(value).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update system setting: %w", err)
		}

	default:
		return fmt.Errorf("unsupported client type")
	}

	return nil
}
