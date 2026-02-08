package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
	"golang.org/x/crypto/bcrypt"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/log"
)

type AuthServiceParams struct {
	fx.In

	SystemService *SystemService
	APIKeyService *APIKeyService
	UserService   *UserService
	Ent           *ent.Client
}

func NewAuthService(params AuthServiceParams) *AuthService {
	return &AuthService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		SystemService: params.SystemService,
		APIKeyService: params.APIKeyService,
		UserService:   params.UserService,
	}
}

type AuthService struct {
	*AbstractService

	SystemService *SystemService
	APIKeyService *APIKeyService
	UserService   *UserService
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return hex.EncodeToString(hashedPassword), nil
}

// VerifyPassword verifies a password against a hash.
func VerifyPassword(hashedPassword, password string) error {
	decodedHashedPassword, err := hex.DecodeString(hashedPassword)
	if err != nil {
		return fmt.Errorf("failed to decode hashed password: %w", err)
	}

	return bcrypt.CompareHashAndPassword(decodedHashedPassword, []byte(password))
}

// GenerateSecretKey generates a random secret key for JWT.
func GenerateSecretKey() (string, error) {
	bytes := make([]byte, 32) // 256 bits

	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}

// GenerateJWTToken generates a JWT token for a user.
func (s *AuthService) GenerateJWTToken(ctx context.Context, user *ent.User) (string, error) {
	secretKey, err := authz.RunWithSystemBypass(ctx, "auth-get-secret-key", func(bypassCtx context.Context) (string, error) {
		return s.SystemService.SecretKey(bypassCtx)
	})
	if err != nil {
		return "", fmt.Errorf("failed to get secret key: %w", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
	})

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return tokenString, nil
}

// AuthenticateUser authenticates a user with email and password.
func (s *AuthService) AuthenticateUser(
	ctx context.Context,
	email, password string,
) (*ent.User, error) {
	u, err := authz.RunWithSystemBypass(ctx, "auth-lookup", func(bypassCtx context.Context) (*ent.User, error) {
		client := s.entFromContext(bypassCtx)

		return client.User.Query().
			Where(user.EmailEQ(email)).
			Where(user.StatusEQ(user.StatusActivated)).
			WithRoles().
			Only(bypassCtx)
	})
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("invalid email or password: %w", ErrInvalidPassword)
		}

		log.Error(ctx, "failed to get user", log.Cause(err))

		return nil, ErrInternal
	}

	err = VerifyPassword(u.Password, password)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password %w", ErrInvalidPassword)
	}

	log.Debug(ctx, "user authenticated", log.Int("user_id", u.ID))

	return u, nil
}

// AuthenticateJWTToken validates a JWT token and returns the user.
func (s *AuthService) AuthenticateJWTToken(ctx context.Context, tokenString string) (*ent.User, error) {
	secretKey, err := authz.RunWithSystemBypass(ctx, "auth-get-secret-key", func(bypassCtx context.Context) (string, error) {
		return s.SystemService.SecretKey(bypassCtx)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret key: %w", err)
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method: %v", ErrInvalidJWT, token.Header["alg"])
		}

		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse jwt token: %w", ErrInvalidJWT, err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("%w: invalid token", ErrInvalidJWT)
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("%w: invalid token claims", ErrInvalidJWT)
	}

	u, err := authz.RunWithSystemBypass(ctx, "auth-lookup", func(bypassCtx context.Context) (*ent.User, error) {
		return s.UserService.GetUserByID(bypassCtx, int(userID))
	})
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get user: %w", ErrInvalidJWT, err)
	}

	if u.Status != user.StatusActivated {
		return nil, fmt.Errorf("%w: user not activated", ErrInvalidJWT)
	}

	return u, nil
}

func (s *AuthService) AnthenticateAPIKey(ctx context.Context, key string) (*ent.APIKey, error) {
	apiKey, err := authz.RunWithSystemBypass(ctx, "auth-lookup", func(bypassCtx context.Context) (*ent.APIKey, error) {
		return s.APIKeyService.GetAPIKey(bypassCtx, key)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get api key: %w", err)
	}

	if apiKey.Status != apikey.StatusEnabled {
		return nil, fmt.Errorf("api key not enabled: %w", ErrInvalidAPIKey)
	}

	proj, err := apiKey.Project(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get api key project: %w", err)
	}

	if proj == nil || proj.Status != project.StatusActive {
		return nil, fmt.Errorf("api key project not valid: %w", ErrInvalidAPIKey)
	}

	return apiKey, nil
}
