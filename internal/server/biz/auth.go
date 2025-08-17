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

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/user"
)

type AuthServiceParams struct {
	fx.In

	SystemService *SystemService
}

func NewAuthService(params AuthServiceParams) *AuthService {
	return &AuthService{
		SystemService: params.SystemService,
	}
}

type AuthService struct {
	SystemService *SystemService
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hashedPassword), nil
}

// VerifyPassword verifies a password against a hash.
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	secretKey, err := s.SystemService.SecretKey(ctx)
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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := ent.FromContext(ctx)

	user, err := client.User.Query().
		Where(user.EmailEQ(email)).
		Where(user.StatusEQ(user.StatusActivated)).
		WithRoles().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password: %w", ErrInvalidPassword)
	}

	// Verify password
	err = VerifyPassword(user.Password, password)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password %w", ErrInvalidPassword)
	}
	return user, nil
}

// ValidateJWTToken validates a JWT token and returns the user.
func (s *AuthService) ValidateJWTToken(ctx context.Context, tokenString string) (*ent.User, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	secretKey, err := s.SystemService.SecretKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret key: %w", err)
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse jwt token: %w", ErrInvalidJWT)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", ErrInvalidJWT)
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid token claims: %w", ErrInvalidJWT)
	}

	client := ent.FromContext(ctx)
	u, err := client.User.Query().
		Where(user.ID(int(userID))).
		WithRoles().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if u.Status != user.StatusActivated {
		return nil, fmt.Errorf("user not activated: %w", ErrInvalidJWT)
	}

	return u, nil
}

func (s *AuthService) ValidateAPIKey(ctx context.Context, key string) (*ent.APIKey, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	// 查询数据库验证 API key 是否存在
	client := ent.FromContext(ctx)

	apiKey, err := client.APIKey.Query().
		WithUser().
		Where(apikey.KeyEQ(key), apikey.StatusEQ(apikey.StatusEnabled)).
		First(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get api key: %w", err)
	}

	apiOwner, err := apiKey.User(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get api key: %w", err)
	}

	if apiOwner == nil || apiOwner.Status != user.StatusActivated {
		return nil, fmt.Errorf("api key owner not valid: %w", ErrInvalidAPIKey)
	}

	return apiKey, nil
}

// GenerateAPIKey generates a new API key with ah- prefix (similar to OpenAI format).
func (s *AuthService) GenerateAPIKey() (string, error) {
	// Generate 32 bytes of random data
	bytes := make([]byte, 32)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex and add ah- prefix
	return "ah-" + hex.EncodeToString(bytes), nil
}
