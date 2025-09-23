package biz

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestHashPassword(t *testing.T) {
	password := "test-password-123"

	hashedPassword, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hashedPassword)
	assert.NotEqual(t, password, hashedPassword)

	// Test that same password produces different hashes (due to salt)
	hashedPassword2, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEqual(t, hashedPassword, hashedPassword2)
}

func TestVerifyPassword(t *testing.T) {
	password := "test-password-123"
	wrongPassword := "wrong-password"

	hashedPassword, err := HashPassword(password)
	require.NoError(t, err)

	// Test correct password
	err = VerifyPassword(hashedPassword, password)
	assert.NoError(t, err)

	// Test wrong password
	err = VerifyPassword(hashedPassword, wrongPassword)
	assert.Error(t, err)

	// Test invalid hash
	err = VerifyPassword("invalid-hash", password)
	assert.Error(t, err)
}

func TestGenerateSecretKey(t *testing.T) {
	secretKey, err := GenerateSecretKey()
	require.NoError(t, err)
	assert.NotEmpty(t, secretKey)
	assert.Len(t, secretKey, 64) // 32 bytes * 2 (hex encoding)

	// Test that multiple calls produce different keys
	secretKey2, err := GenerateSecretKey()
	require.NoError(t, err)
	assert.NotEqual(t, secretKey, secretKey2)
}

func setupTestDB(t *testing.T) *ent.Client {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")

	return client
}

func setupTestAuthService(t *testing.T, cacheConfig xcache.Config) (*AuthService, *ent.Client) {
	t.Helper()
	client := setupTestDB(t)

	// Create a mock system service
	systemService := &SystemService{
		Cache: xcache.NewFromConfig[ent.System](cacheConfig),
	}

	// Set up a test secret key in the system service
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

	// Create APIKeyService and UserService
	apiKeyService := &APIKeyService{
		UserService: userService,
		APIKeyCache: xcache.NewFromConfig[ent.APIKey](cacheConfig),
	}

	authService := &AuthService{
		SystemService: systemService,
		APIKeyService: apiKeyService,
		UserService:   userService,
	}

	return authService, client
}

func TestAuthService_GenerateJWTToken(t *testing.T) {
	// Test with memory cache
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	authService, client := setupTestAuthService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	assert.NotEmpty(t, token)

	// Get the actual secret key for validation
	secretKey, err := authService.SystemService.SecretKey(ctx)
	require.NoError(t, err)

	// Verify token structure
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	require.NoError(t, err)

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	require.True(t, ok)

	userID, ok := claims["user_id"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(testUser.ID), userID)

	exp, ok := claims["exp"].(float64)
	require.True(t, ok)
	assert.True(t, exp > float64(time.Now().Unix()))
}

func TestAuthService_AuthenticateUser(t *testing.T) {
	// Test with Redis cache using miniredis
	mr := miniredis.RunT(t)
	defer mr.Close()

	cacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xcache.RedisConfig{
			Addr: mr.Addr(),
		},
	}

	authService, client := setupTestAuthService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	assert.Equal(t, testUser.ID, authenticatedUser.ID)
	assert.Equal(t, testUser.Email, authenticatedUser.Email)

	// Test wrong password
	_, err = authService.AuthenticateUser(ctx, "test@example.com", "wrong-password")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email or password")

	// Test non-existent user
	_, err = authService.AuthenticateUser(ctx, "nonexistent@example.com", password)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email or password")

	// Test deactivated user
	_, err = authService.UserService.UpdateUserStatus(ctx, testUser.ID, user.StatusDeactivated)
	require.NoError(t, err)

	_, err = authService.AuthenticateUser(ctx, "test@example.com", password)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email or password")
}

func TestAuthService_AuthenticateJWTToken(t *testing.T) {
	// Test with two-level cache
	mr := miniredis.RunT(t)
	defer mr.Close()

	cacheConfig := xcache.Config{
		Mode: xcache.ModeTwoLevel,
		Redis: xcache.RedisConfig{
			Addr: mr.Addr(),
		},
	}

	authService, client := setupTestAuthService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	assert.Equal(t, testUser.ID, authenticatedUser.ID)
	assert.Equal(t, testUser.Email, authenticatedUser.Email)

	// Test cache hit - second call should use cache
	authenticatedUser2, err := authService.AuthenticateJWTToken(ctx, tokenString)
	require.NoError(t, err)
	assert.Equal(t, testUser.ID, authenticatedUser2.ID)

	// Test invalid token
	_, err = authService.AuthenticateJWTToken(ctx, "invalid-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse jwt token")

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
	assert.Error(t, err)

	// Test deactivated user
	_, err = authService.UserService.UpdateUserStatus(ctx, testUser.ID, user.StatusDeactivated)
	require.NoError(t, err)

	// Generate new token for deactivated user
	newTokenString, err := authService.GenerateJWTToken(ctx, testUser)
	require.NoError(t, err)

	_, err = authService.AuthenticateJWTToken(ctx, newTokenString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not activated")
}

func TestAuthService_AnthenticateAPIKey(t *testing.T) {
	// Test with noop cache (no cache configured)
	cacheConfig := xcache.Config{} // Empty config = noop cache

	authService, client := setupTestAuthService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

	// Generate API key
	apiKeyString, err := GenerateAPIKey()
	require.NoError(t, err)

	// Create API key in database
	apiKey, err := client.APIKey.Create().
		SetKey(apiKeyString).
		SetName("Test API Key").
		SetUser(testUser).
		Save(ctx)
	require.NoError(t, err)

	// Test successful API key authentication
	authenticatedAPIKey, err := authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	assert.Equal(t, apiKey.ID, authenticatedAPIKey.ID)
	assert.Equal(t, apiKey.Key, authenticatedAPIKey.Key)

	// Test cache behavior - second call should still work (even with noop cache)
	authenticatedAPIKey2, err := authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	assert.Equal(t, apiKey.ID, authenticatedAPIKey2.ID)

	// Test invalid API key
	_, err = authService.AnthenticateAPIKey(ctx, "invalid-api-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get api key")

	// Test disabled API key
	_, err = authService.APIKeyService.UpdateAPIKeyStatus(ctx, apiKey.ID, "disabled")
	require.NoError(t, err)

	_, err = authService.AnthenticateAPIKey(ctx, apiKeyString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api key not enabled")

	// Test API key with deactivated user
	// First, re-enable the API key
	_, err = authService.APIKeyService.UpdateAPIKeyStatus(ctx, apiKey.ID, "enabled")
	require.NoError(t, err)

	// Then deactivate the user
	_, err = authService.UserService.UpdateUserStatus(ctx, testUser.ID, user.StatusDeactivated)
	require.NoError(t, err)

	_, err = authService.AnthenticateAPIKey(ctx, apiKeyString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api key owner not valid")
}

func TestAuthService_WithDifferentCacheConfigs(t *testing.T) {
	testCases := []struct {
		name        string
		cacheConfig xcache.Config
	}{
		{
			name:        "Memory Cache",
			cacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		},
		{
			name: "Redis Cache",
			cacheConfig: xcache.Config{
				Mode: xcache.ModeRedis,
				Redis: xcache.RedisConfig{
					Addr: miniredis.RunT(t).Addr(),
				},
			},
		},
		{
			name: "Two-Level Cache",
			cacheConfig: xcache.Config{
				Mode: xcache.ModeTwoLevel,
				Redis: xcache.RedisConfig{
					Addr: miniredis.RunT(t).Addr(),
				},
			},
		},
		{
			name:        "Noop Cache",
			cacheConfig: xcache.Config{}, // Empty = noop
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authService, client := setupTestAuthService(t, tc.cacheConfig)
			defer client.Close()

			ctx := context.Background()
			ctx = ent.NewContext(ctx, client)
			ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
			assert.Equal(t, testUser.ID, authenticatedUser.ID)

			// Test user authentication
			authenticatedUser2, err := authService.AuthenticateUser(ctx, "test@example.com", "test-password")
			require.NoError(t, err)
			assert.Equal(t, testUser.ID, authenticatedUser2.ID)
		})
	}
}

func TestAuthService_CacheExpiration(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	cacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xcache.RedisConfig{
			Addr:       mr.Addr(),
			Expiration: 100 * time.Millisecond, // Very short for testing
		},
	}

	authService, client := setupTestAuthService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

	// Generate API key
	apiKeyString, err := GenerateAPIKey()
	require.NoError(t, err)

	apiKey, err := client.APIKey.Create().
		SetKey(apiKeyString).
		SetName("Test API Key").
		SetUser(testUser).
		Save(ctx)
	require.NoError(t, err)

	// First call - should cache the result
	authenticatedAPIKey, err := authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	assert.Equal(t, apiKey.ID, authenticatedAPIKey.ID)

	// Wait for cache expiration
	time.Sleep(150 * time.Millisecond)

	// Second call - cache should be expired, should hit database again
	authenticatedAPIKey2, err := authService.AnthenticateAPIKey(ctx, apiKeyString)
	require.NoError(t, err)
	assert.Equal(t, apiKey.ID, authenticatedAPIKey2.ID)
}
