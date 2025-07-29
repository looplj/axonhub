package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/ent/system"
	"go.uber.org/fx"
)

const (
	SystemKeyInitialized = "initialized"
	SystemKeySecretKey   = "secret_key"
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
	sys, err := s.Ent.System.Query().Where(system.KeyEQ(SystemKeyInitialized)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return strings.EqualFold(sys.Value, "true"), nil
}

// Initialize initializes the system with a secret key and sets the initialized flag
func (s *SystemService) Initialize(ctx context.Context, secretKey string) error {
	// Check if system is already initialized
	isInitialized, err := s.IsInitialized(ctx)
	if err != nil {
		return fmt.Errorf("failed to check initialization status: %w", err)
	}
	if isInitialized {
		// System is already initialized, nothing to do
		return nil
	}

	// If no secret key provided, generate one
	if secretKey == "" {
		generatedKey, err := s.generateSecretKey()
		if err != nil {
			return fmt.Errorf("failed to generate secret key: %w", err)
		}
		secretKey = generatedKey
	}

	// Start a transaction
	tx, err := s.Ent.Tx(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Set secret key
	err = s.setSystemValue(ctx, tx.System, SystemKeySecretKey, secretKey)
	if err != nil {
		return fmt.Errorf("failed to set secret key: %w", err)
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

// GetSecretKey retrieves the JWT secret key from system settings
func (s *SystemService) GetSecretKey(ctx context.Context) (string, error) {
	sys, err := s.Ent.System.Query().Where(system.KeyEQ(SystemKeySecretKey)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", fmt.Errorf("secret key not found, system may not be initialized")
		}
		return "", fmt.Errorf("failed to get secret key: %w", err)
	}
	return sys.Value, nil
}

// SetSecretKey sets a new JWT secret key
func (s *SystemService) SetSecretKey(ctx context.Context, secretKey string) error {
	return s.setSystemValue(ctx, s.Ent.System, SystemKeySecretKey, secretKey)
}

// generateSecretKey generates a random secret key for JWT
func (s *SystemService) generateSecretKey() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// setSystemValue sets or updates a system key-value pair
func (s *SystemService) setSystemValue(ctx context.Context, client interface{}, key, value string) error {
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
