package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"entgo.io/ent/privacy"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
	"golang.org/x/crypto/bcrypt"

	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/ent/user"
)

type AuthServiceParams struct {
	fx.In

	Ent           *ent.Client
	SystemService *SystemService
}

func NewAuthService(params AuthServiceParams) *AuthService {
	return &AuthService{
		Ent:           params.Ent,
		SystemService: params.SystemService,
	}
}

type AuthService struct {
	Ent           *ent.Client
	SystemService *SystemService
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedPassword), nil
}

// VerifyPassword verifies a password against a hash
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// GenerateSecretKey generates a random secret key for JWT
func GenerateSecretKey() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateJWTToken generates a JWT token for a user
func (s *AuthService) GenerateJWTToken(ctx context.Context, user *ent.User) (string, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	secretKey, err := s.SystemService.GetSecretKey(ctx)
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

// AuthenticateUser authenticates a user with email and password
func (s *AuthService) AuthenticateUser(ctx context.Context, email, password string) (*ent.User, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	user, err := s.Ent.User.Query().
		Where(user.EmailEQ(email)).
		WithRoles().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Verify password
	err = VerifyPassword(user.Password, password)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	return user, nil
}

// ValidateJWTToken validates a JWT token and returns the user
func (s *AuthService) ValidateJWTToken(ctx context.Context, tokenString string) (*ent.User, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	secretKey, err := s.SystemService.GetSecretKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret key: %w", err)
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	user, err := s.Ent.User.Get(ctx, int(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}
