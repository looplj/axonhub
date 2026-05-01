package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/log"
)

const OIDC_ONLY_PLACEHOLDER = "!OIDC_SSO_ONLY!"

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	if password == OIDC_ONLY_PLACEHOLDER {
		return OIDC_ONLY_PLACEHOLDER, nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password=[MASKED_SECRET]", err)
	}

	return hex.EncodeToString(hashedPassword), nil
}

// VerifyPassword verifies a password against a hash.
func VerifyPassword(hashedPassword, password string) error {
	if hashedPassword == OIDC_ONLY_PLACEHOLDER {
		return ErrOIDCLoginRequired
	}

	decodedHashedPassword, err := hex.DecodeString(hashedPassword)
	if err != nil {
		return fmt.Errorf("failed to decode hashed password=[MASKED_SECRET]", err)
	}

	return bcrypt.CompareHashAndPassword(decodedHashedPassword, []byte(password))
}

// TokenRevocationService maintains an in-memory revocation list for JWT tokens.
type TokenRevocationService struct {
	mu       sync.RWMutex
	revoked  map[string]time.Time // jti -> expiry (used for cleanup)
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func NewTokenRevocationService() *TokenRevocationService {
	return &TokenRevocationService{
		revoked: make(map[string]time.Time),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// Revoke adds a token jti to the revocation list.
func (s *TokenRevocationService) Revoke(jti string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[jti] = time.Now().Add(24 * time.Hour)
}

// IsRevoked checks whether a token jti has been revoked.
func (s *TokenRevocationService) IsRevoked(jti string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.revoked[jti]
	return ok
}

// StartSweeper periodically removes expired entries from the revocation list.
func (s *TokenRevocationService) StartSweeper(ctx context.Context) {
	go func() {
		defer close(s.doneCh)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.sweep()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the sweeper goroutine.
func (s *TokenRevocationService) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

func (s *TokenRevocationService) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for jti, exp := range s.revoked {
		if now.After(exp) {
			delete(s.revoked, jti)
		}
	}
}

type AuthServiceParams struct {
	fx.In

	SystemService      *SystemService
	APIKeyService      *APIKeyService
	UserService        *UserService
	OIDCService        *OIDCService
	TokenRevocationSvc *TokenRevocationService
	Ent                *ent.Client
	AllowNoAuth        bool `name:"allow_no_auth"`
}

func NewAuthService(params AuthServiceParams) *AuthService {
	return &AuthService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		SystemService:      params.SystemService,
		APIKeyService:      params.APIKeyService,
		UserService:        params.UserService,
		OIDCService:        params.OIDCService,
		TokenRevocationSvc: params.TokenRevocationSvc,
		AllowNoAuth:        params.AllowNoAuth,
	}
}

type AuthService struct {
	*AbstractService

	SystemService      *SystemService
	APIKeyService      *APIKeyService
	UserService        *UserService
	OIDCService        *OIDCService
	TokenRevocationSvc *TokenRevocationService
	AllowNoAuth        bool
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

	jti := uuid.New().String()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"jti":     jti,
	})

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return tokenString, nil
}

// RevokeJWT revokes a JWT token by its jti claim.
func (s *AuthService) RevokeJWT(jti string) {
	s.TokenRevocationSvc.Revoke(jti)
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
			return nil, fmt.Errorf("invalid email or password=[MASKED_SECRET]", ErrInvalidPassword)
		}

		log.Error(ctx, "failed to get user", log.Cause(err))

		return nil, ErrInternal
	}

	if s.OIDCService != nil && s.OIDCService.IsUserRestrictedToOIDC(ctx, u) {
		return nil, ErrOIDCLoginRequired
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
		if errors.Is(err, ErrSystemNotInitialized) {
			return nil, fmt.Errorf("%w: system not initialized", ErrInvalidJWT)
		}
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

	// Check revocation by jti.
	if jti, ok := claims["jti"].(string); ok && s.TokenRevocationSvc.IsRevoked(jti) {
		return nil, fmt.Errorf("%w: token revoked", ErrInvalidJWT)
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

func (s *AuthService) AuthenticateAPIKey(ctx context.Context, key string) (*ent.APIKey, error) {
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

	if apiKey.Type == apikey.TypeNoauth {
		return nil, fmt.Errorf("noauth api key is only available when api auth is disabled: %w", ErrInvalidAPIKey)
	}

	return apiKey, nil
}

func (s *AuthService) AuthenticateNoAuth(ctx context.Context) (*ent.APIKey, error) {
	if !s.AllowNoAuth {
		return nil, fmt.Errorf("%w: API key required", ErrInvalidAPIKey)
	}

	apiKey, err := authz.RunWithSystemBypass(ctx, "auth-noauth", func(bypassCtx context.Context) (*ent.APIKey, error) {
		return s.APIKeyService.EnsureNoAuthAPIKey(bypassCtx)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to ensure noauth api key: %w", err)
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
