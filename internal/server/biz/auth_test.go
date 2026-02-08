package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
)

func TestHashPassword(t *testing.T) {
	password := "test-password-123"

	hashedPassword, err := HashPassword(password)
	require.NoError(t, err)
	require.NotEmpty(t, hashedPassword)
	require.NotEqual(t, password, hashedPassword)

	// Test that same password produces different hashes (due to salt)
	hashedPassword2, err := HashPassword(password)
	require.NoError(t, err)
	require.NotEqual(t, hashedPassword, hashedPassword2)
}

func TestVerifyPassword(t *testing.T) {
	password := "test-password-123"
	wrongPassword := "wrong-password"

	hashedPassword, err := HashPassword(password)
	require.NoError(t, err)

	// Test correct password
	err = VerifyPassword(hashedPassword, password)
	require.NoError(t, err)

	// Test wrong password
	err = VerifyPassword(hashedPassword, wrongPassword)
	require.Error(t, err)

	// Test invalid hash
	err = VerifyPassword("invalid-hash", password)
	require.Error(t, err)
}

func TestGenerateSecretKey(t *testing.T) {
	secretKey, err := GenerateSecretKey()
	require.NoError(t, err)
	require.NotEmpty(t, secretKey)
	require.Len(t, secretKey, 64) // 32 bytes * 2 (hex encoding)

	// Test that multiple calls produce different keys
	secretKey2, err := GenerateSecretKey()
	require.NoError(t, err)
	require.NotEqual(t, secretKey, secretKey2)
}

func setupTestDB(t *testing.T) *ent.Client {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	return client
}

func setupTestAuthService(t *testing.T, cacheConfig xcache.Config) (*AuthService, *ent.Client, func()) {
	t.Helper()
	client := setupTestDB(t)

	// Create a mock system service
	systemService := &SystemService{
		Cache: xcache.NewFromConfig[ent.System](cacheConfig),
	}

	// Set up a test secret key in the system service
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create system entry for secret key
	secretKey, err := GenerateSecretKey()
	require.NoError(t, err)

	_, err = client.System.Create().
		SetKey(SystemKeySecretKey).
		SetValue(secretKey).
		Save(ctx)
	require.NoError(t, err)

	userService := &UserService{
		UserCache: xcache.NewFromConfig[ent.User](cacheConfig),
	}

	projectService := &ProjectService{
		ProjectCache: xcache.NewFromConfig[ent.Project](cacheConfig),
	}

	apiKeyService := NewAPIKeyService(APIKeyServiceParams{
		CacheConfig:    cacheConfig,
		Ent:            client,
		ProjectService: projectService,
	})

	authService := &AuthService{
		SystemService: systemService,
		APIKeyService: apiKeyService,
		UserService:   userService,
	}

	cleanup := func() {
		apiKeyService.Stop()
	}

	return authService, client, cleanup
}

func TestAuthService_GenerateJWTToken(t *testing.T) {
	// Test with memory cache
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	authService, client, cleanup := setupTestAuthService(t, cacheConfig)
	defer cleanup()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Generate JWT token
	token, err := authService.GenerateJWTToken(ctx, testUser)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Get the actual secret key for validation
	secretKey, err := authService.SystemService.SecretKey(ctx)
	require.NoError(t, err)

	// Verify token structure
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		return []byte(secretKey), nil
	})
	require.NoError(t, err)

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	require.True(t, ok)

	userID, ok := claims["user_id"].(float64)
	require.True(t, ok)
	require.Equal(t, float64(testUser.ID), userID)

	exp, ok := claims["exp"].(float64)
	require.True(t, ok)
	require.True(t, exp > float64(time.Now().Unix()))
}

func TestAuthService_AuthenticateUser(t *testing.T) {
	// Test with Redis cache using miniredis
	mr := miniredis.RunT(t)

	cacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xredis.Config{
			Addr: mr.Addr(),
		},
	}

	authService, client, cleanup := setupTestAuthService(t, cacheConfig)
	defer cleanup()
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test user
	password := "test-password-123"
	hashedPassword, err := HashPassword(password)
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Test successful authentication
	authenticatedUser, err := authService.AuthenticateUser(ctx, "test@example.com", password)
	require.NoError(t, err)
	require.Equal(t, testUser.ID, authenticatedUser.ID)
	require.Equal(t, testUser.Email, authenticatedUser.Email)

	// Test wrong password
	_, err = authService.AuthenticateUser(ctx, "test@example.com", "wrong-password")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid email or password")

	// Test non-existent user
	_, err = authService.AuthenticateUser(ctx, "nonexistent@example.com", password)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid email or password")

	// Test deactivated user
	_, err = authService.UserService.UpdateUserStatus(ctx, testUser.ID, user.StatusDeactivated)
	require.NoError(t, err)

	_, err = authService.AuthenticateUser(ctx, "test@example.com", password)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid email or password")
}

func TestAuthService_AuthenticateJWTToken(t *testing.T) {
	// Test with two-level cache
	mr := miniredis.RunT(t)

	cacheConfig := xcache.Config{
		Mode: xcache.ModeTwoLevel,
		Redis: xredis.Config{
			Addr: mr.Addr(),
		},
	}

	authService, client, cleanup := setupTestAuthService(t, cacheConfig)
	defer cleanup()
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Generate a valid JWT token
	tokenString, err := authService.GenerateJWTToken(ctx, testUser)
	require.NoError(t, err)

	// Test successful JWT authentication
	authenticatedUser, err := authService.AuthenticateJWTToken(ctx, tokenString)
	require.NoError(t, err)
	require.Equal(t, testUser.ID, authenticatedUser.ID)
	require.Equal(t, testUser.Email, authenticatedUser.Email)

	// Test cache hit - second call should use cache
	authenticatedUser2, err := authService.AuthenticateJWTToken(ctx, tokenString)
	require.NoError(t, err)
	require.Equal(t, testUser.ID, authenticatedUser2.ID)

	// Test invalid token
	_, err = authService.AuthenticateJWTToken(ctx, "invalid-token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse jwt token")

	// Test expired token (create manually)
	expiredClaims := jwt.MapClaims{
		"user_id": float64(testUser.ID),
		"exp":     time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
	}

	// Get secret key for signing
	secretKey, err := authService.SystemService.SecretKey(ctx)
	require.NoError(t, err)

	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenString, err := expiredToken.SignedString([]byte(secretKey))
	require.NoError(t, err)

	_, err = authService.AuthenticateJWTToken(ctx, expiredTokenString)
	require.Error(t, err)

	// Test deactivated user
	_, err = authService.UserService.UpdateUserStatus(ctx, testUser.ID, user.StatusDeactivated)
	require.NoError(t, err)

	// Generate new token for deactivated user
	newTokenString, err := authService.GenerateJWTToken(ctx, testUser)
	require.NoError(t, err)

	_, err = authService.AuthenticateJWTToken(ctx, newTokenString)
	require.Error(t, err)
	require.Contains(t, err.Error(), "user not activated")
}

func TestAuthService_AnthenticateAPIKey(t *testing.T) {
	// Test with noop cache (no cache configured)
	cacheConfig := xcache.Config{} // Empty config = noop cache

	authService, client, cleanup := setupTestAuthService(t, cacheConfig)
	defer cleanup()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	projectName := uuid.NewString()
	testProject, err := client.Project.Create().SetName(
		projectName,
	).SetDescription(
		projectName,
	).SetStatus(
		project.StatusActive,
	).SetCreatedAt(
		time.Now(),
	).SetUpdatedAt(
		time.Now(),
	).Save(
		ctx,
	)
	require.NoError(t, err)

	// Generate API key
	apiKeyString, err := GenerateAPIKey()
	require.NoError(t, err)

	// Create API key in database
	apiKey, err := client.APIKey.Create().
		SetKey(apiKeyString).
		SetName("Test API Key").
		SetUser(testUser).
		SetProject(testProject).
		Save(ctx)
	require.NoError(t, err)

	// Test successful API key authentication
	authenticatedAPIKey, err := authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	require.Equal(t, apiKey.ID, authenticatedAPIKey.ID)
	require.Equal(t, apiKey.Key, authenticatedAPIKey.Key)

	// Test cache behavior - second call should still work (even with noop cache)
	authenticatedAPIKey2, err := authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	require.Equal(t, apiKey.ID, authenticatedAPIKey2.ID)

	// Test invalid API key
	_, err = authService.AnthenticateAPIKey(ctx, "invalid-api-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get api key")

	// Test disabled API key
	_, err = authService.APIKeyService.UpdateAPIKeyStatus(ctx, apiKey.ID, "disabled")
	require.NoError(t, err)

	// Synchronously invalidate the cache for testing (async notification may not complete in time)
	authService.APIKeyService.APIKeyCache.Invalidate(buildAPIKeyCacheKey(apiKeyString))

	_, err = authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.Error(t, err)
	require.Contains(t, err.Error(), "api key not enabled")

	// Test API key with inactive project
	// First, re-enable the API key
	_, err = authService.APIKeyService.UpdateAPIKeyStatus(ctx, apiKey.ID, "enabled")
	require.NoError(t, err)

	// Synchronously invalidate the cache for testing
	authService.APIKeyService.APIKeyCache.Invalidate(buildAPIKeyCacheKey(apiKeyString))

	// Then archive the project (making it inactive)
	_, err = client.Project.UpdateOneID(testProject.ID).
		SetStatus(project.StatusArchived).
		Save(ctx)
	require.NoError(t, err)

	_, err = authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.Error(t, err)
	require.Contains(t, err.Error(), "api key project not valid")
}

func TestAuthService_WithDifferentCacheConfigs(t *testing.T) {
	testCases := []struct {
		name         string
		cacheMode    string
		requireRedis bool
	}{
		{
			name:         "Memory Cache",
			cacheMode:    xcache.ModeMemory,
			requireRedis: false,
		},
		{
			name:         "Redis Cache",
			cacheMode:    xcache.ModeRedis,
			requireRedis: true,
		},
		{
			name:         "Two-Level Cache",
			cacheMode:    xcache.ModeTwoLevel,
			requireRedis: true,
		},
		{
			name:         "Noop Cache",
			cacheMode:    "",
			requireRedis: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var cacheConfig xcache.Config

			if tc.requireRedis {
				mr := miniredis.RunT(t)
				cacheConfig = xcache.Config{
					Mode: tc.cacheMode,
					Redis: xredis.Config{
						Addr: mr.Addr(),
					},
				}
			} else {
				cacheConfig = xcache.Config{Mode: tc.cacheMode}
			}

			authService, client, cleanup := setupTestAuthService(t, cacheConfig)
			defer cleanup()
			defer client.Close()

			ctx := context.Background()
			ctx = ent.NewContext(ctx, client)
			ctx = authz.WithTestBypass(ctx)

			// Create a test user
			hashedPassword, err := HashPassword("test-password")
			require.NoError(t, err)

			testUser, err := client.User.Create().
				SetEmail("test@example.com").
				SetPassword(hashedPassword).
				SetFirstName("Test").
				SetLastName("User").
				SetStatus(user.StatusActivated).
				Save(ctx)
			require.NoError(t, err)

			// Test JWT token generation and authentication
			tokenString, err := authService.GenerateJWTToken(ctx, testUser)
			require.NoError(t, err)

			authenticatedUser, err := authService.AuthenticateJWTToken(ctx, tokenString)
			require.NoError(t, err)
			require.Equal(t, testUser.ID, authenticatedUser.ID)

			// Test user authentication
			authenticatedUser2, err := authService.AuthenticateUser(ctx, "test@example.com", "test-password")
			require.NoError(t, err)
			require.Equal(t, testUser.ID, authenticatedUser2.ID)
		})
	}
}

func TestAuthService_CacheExpiration(t *testing.T) {
	mr := miniredis.RunT(t)

	cacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xredis.Config{
			Addr:       mr.Addr(),
			Expiration: 100 * time.Millisecond, // Very short for testing
		},
	}

	authService, client, cleanup := setupTestAuthService(t, cacheConfig)
	defer cleanup()
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	projectName := uuid.NewString()
	testProject, err := client.Project.Create().SetName(
		projectName,
	).SetDescription(
		projectName,
	).SetStatus(
		project.StatusActive,
	).SetCreatedAt(
		time.Now(),
	).SetUpdatedAt(
		time.Now(),
	).Save(
		ctx,
	)
	require.NoError(t, err)

	// Generate API key
	apiKeyString, err := GenerateAPIKey()
	require.NoError(t, err)

	apiKey, err := client.APIKey.Create().
		SetKey(apiKeyString).
		SetName("Test API Key").
		SetUser(testUser).
		SetProject(testProject).
		Save(ctx)
	require.NoError(t, err)

	// First call - should cache the result
	authenticatedAPIKey, err := authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	require.Equal(t, apiKey.ID, authenticatedAPIKey.ID)

	// Wait for cache expiration
	time.Sleep(150 * time.Millisecond)

	// Second call - cache should be expired, should hit database again
	authenticatedAPIKey2, err := authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	require.Equal(t, apiKey.ID, authenticatedAPIKey2.ID)
}
