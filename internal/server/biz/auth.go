package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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

const OIDC_ONLY_PLACEHOLDER = "!OIDC_SSO_ONLY!"

const (
	AdminSessionCookieName = "axonhub_session"
	AdminSessionIdle       = 30 * 24 * time.Hour
	AdminSessionMaximum    = 90 * 24 * time.Hour
	AdminSessionRenewal    = 7 * 24 * time.Hour
)

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	if password == OIDC_ONLY_PLACEHOLDER {
		return OIDC_ONLY_PLACEHOLDER, nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
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
		return fmt.Errorf("failed to decode hashed password: %w", err)
	}

	return bcrypt.CompareHashAndPassword(decodedHashedPassword, []byte(password))
}

type AuthServiceParams struct {
	fx.In

	SystemService *SystemService
	APIKeyService *APIKeyService
	UserService   *UserService
	OIDCService   *OIDCService
	Ent           *ent.Client
	AllowNoAuth   bool `name:"allow_no_auth"`
}

func NewAuthService(params AuthServiceParams) *AuthService {
	return &AuthService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		SystemService: params.SystemService,
		APIKeyService: params.APIKeyService,
		UserService:   params.UserService,
		OIDCService:   params.OIDCService,
		AllowNoAuth:   params.AllowNoAuth,
	}
}

type AuthService struct {
	*AbstractService

	SystemService *SystemService
	APIKeyService *APIKeyService
	UserService   *UserService
	OIDCService   *OIDCService
	AllowNoAuth   bool
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
	now := time.Now()
	return s.GenerateJWTTokenAt(ctx, user, now, now)
}

// GenerateJWTTokenAt generates a bounded sliding-session token at the supplied time.
func (s *AuthService) GenerateJWTTokenAt(ctx context.Context, user *ent.User, now, authTime time.Time) (string, error) {
	secretKey, err := authz.RunWithSystemBypass(ctx, "auth-get-secret-key", func(bypassCtx context.Context) (string, error) {
		return s.SystemService.SecretKey(bypassCtx)
	})
	if err != nil {
		return "", fmt.Errorf("failed to get secret key: %w", err)
	}

	expiresAt := now.Add(AdminSessionIdle)
	maximumExpiresAt := authTime.Add(AdminSessionMaximum)
	if expiresAt.After(maximumExpiresAt) {
		expiresAt = maximumExpiresAt
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":   user.ID,
		"iat":       now.Unix(),
		"auth_time": authTime.Unix(),
		"exp":       expiresAt.Unix(),
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
	claims, err := s.parseJWTClaims(ctx, tokenString)
	if err != nil {
		return nil, err
	}

	authTime, err := jwtTimeClaim(claims, "auth_time")
	if _, present := claims["auth_time"]; present && err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidJWT, err)
	}
	if err == nil && !time.Now().Before(authTime.Add(AdminSessionMaximum)) {
		return nil, fmt.Errorf("%w: maximum session age exceeded", ErrInvalidJWT)
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

func (s *AuthService) parseJWTClaims(ctx context.Context, tokenString string) (jwt.MapClaims, error) {
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
	return claims, nil
}

// RefreshJWTToken renews a token only inside the renewal window and never past
// the original authentication time plus the maximum session age.
func (s *AuthService) RefreshJWTToken(ctx context.Context, tokenString string, now time.Time) (string, bool, error) {
	claims, err := s.parseJWTClaims(ctx, tokenString)
	if err != nil {
		return "", false, err
	}
	expiresAt, err := jwtTimeClaim(claims, "exp")
	if err != nil {
		return "", false, fmt.Errorf("%w: %w", ErrInvalidJWT, err)
	}
	if expiresAt.Sub(now) >= AdminSessionRenewal {
		return "", false, nil
	}
	if _, present := claims["auth_time"]; !present {
		return "", false, nil
	}
	authTime, err := jwtTimeClaim(claims, "auth_time")
	if err != nil {
		return "", false, fmt.Errorf("%w: %w", ErrInvalidJWT, err)
	}
	if !now.Before(authTime.Add(AdminSessionMaximum)) {
		return "", false, fmt.Errorf("%w: maximum session age exceeded", ErrInvalidJWT)
	}
	if !now.Add(AdminSessionIdle).Before(authTime.Add(AdminSessionMaximum)) && !expiresAt.Before(authTime.Add(AdminSessionMaximum)) {
		return "", false, nil
	}
	userID, ok := claims["user_id"].(float64)
	if !ok {
		return "", false, fmt.Errorf("%w: invalid token claims", ErrInvalidJWT)
	}
	user, err := authz.RunWithSystemBypass(ctx, "auth-lookup", func(bypassCtx context.Context) (*ent.User, error) {
		return s.UserService.GetUserByID(bypassCtx, int(userID))
	})
	if err != nil {
		return "", false, fmt.Errorf("%w: failed to get user: %w", ErrInvalidJWT, err)
	}
	refreshed, err := s.GenerateJWTTokenAt(ctx, user, now, authTime)
	if err != nil {
		return "", false, err
	}
	return refreshed, true, nil
}

func jwtTimeClaim(claims jwt.MapClaims, name string) (time.Time, error) {
	value, ok := claims[name].(float64)
	if !ok || value <= 0 {
		return time.Time{}, fmt.Errorf("missing %s claim", name)
	}
	return time.Unix(int64(value), 0), nil
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
