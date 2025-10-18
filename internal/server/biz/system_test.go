package biz

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestSystemService_Initialize(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)

	// Test system initialization with auto-generated secret key
	err := service.Initialize(ctx, &InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Verify system is initialized
	isInitialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	require.True(t, isInitialized)

	// Verify secret key is set
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	secretKey, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, secretKey)
	require.Len(t, secretKey, 64) // Should be 64 hex characters (32 bytes)

	// Verify owner user is created
	owner, err := client.User.Query().Where().First(ctx)
	require.NoError(t, err)
	require.Equal(t, "owner@example.com", owner.Email)
	require.True(t, owner.IsOwner)

	// Verify default project is created
	project, err := client.Project.Query().Where().First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Default", project.Name)

	// Verify owner is assigned to the project
	userProject, err := client.UserProject.Query().Where().First(ctx)
	require.NoError(t, err)
	require.Equal(t, owner.ID, userProject.UserID)
	require.Equal(t, project.ID, userProject.ProjectID)
	require.True(t, userProject.IsOwner)

	// Verify default roles are created (admin, developer, viewer)
	roles, err := client.Role.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, roles, 3)

	// Test idempotency - calling Initialize again should not error
	// but should not change the existing secret key or create duplicate projects
	originalKey := secretKey
	err = service.Initialize(ctx, &InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Secret key should remain the same after second initialization
	secretKey2, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.Equal(t, originalKey, secretKey2)

	// Should still have only one project
	projectCount, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, projectCount)
}

func TestSystemService_GetSecretKey_NotInitialized(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)

	// Getting secret key before initialization should return error
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	secretKey, err := service.SecretKey(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "secret key not found, system may not be initialized")
	require.Empty(t, secretKey) // Should be empty when error occurs
}

func setupTestSystemService(t *testing.T, cacheConfig xcache.Config) (*SystemService, *ent.Client) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	systemService := &SystemService{
		Cache: xcache.NewFromConfig[ent.System](cacheConfig),
	}

	return systemService, client
}

func TestSystemService_WithMemoryCache(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Test setting and getting system values with cache
	testKey := "test_key"
	testValue := "test_value"

	err := service.setSystemValue(ctx, testKey, testValue)
	require.NoError(t, err)

	// First call should hit database and cache the result
	retrievedValue, err := service.getSystemValue(ctx, testKey)
	require.NoError(t, err)
	require.Equal(t, testValue, retrievedValue)

	// Second call should hit cache
	retrievedValue2, err := service.getSystemValue(ctx, testKey)
	require.NoError(t, err)
	require.Equal(t, testValue, retrievedValue2)

	// Update value should invalidate cache
	newValue := "new_test_value"
	err = service.setSystemValue(ctx, testKey, newValue)
	require.NoError(t, err)

	// Should get updated value
	retrievedValue3, err := service.getSystemValue(ctx, testKey)
	require.NoError(t, err)
	require.Equal(t, newValue, retrievedValue3)
}

func TestSystemService_WithRedisCache(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	cacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xcache.RedisConfig{
			Addr: mr.Addr(),
		},
	}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Test brand name functionality with Redis cache
	brandName := "Test Brand"
	err := service.SetBrandName(ctx, brandName)
	require.NoError(t, err)

	retrievedBrandName, err := service.BrandName(ctx)
	require.NoError(t, err)
	require.Equal(t, brandName, retrievedBrandName)

	// Test brand logo functionality
	brandLogo := "base64encodedlogo"
	err = service.SetBrandLogo(ctx, brandLogo)
	require.NoError(t, err)

	retrievedBrandLogo, err := service.BrandLogo(ctx)
	require.NoError(t, err)
	require.Equal(t, brandLogo, retrievedBrandLogo)
}

func TestSystemService_WithTwoLevelCache(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	cacheConfig := xcache.Config{
		Mode: xcache.ModeTwoLevel,
		Redis: xcache.RedisConfig{
			Addr: mr.Addr(),
		},
	}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Test secret key functionality with two-level cache
	secretKey := "test-secret-key-123456789012345678901234567890123456789012345678901234567890123456789012"
	err := service.SetSecretKey(ctx, secretKey)
	require.NoError(t, err)

	retrievedSecretKey, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.Equal(t, secretKey, retrievedSecretKey)
}

func TestSystemService_WithNoopCache(t *testing.T) {
	cacheConfig := xcache.Config{} // Empty config = noop cache

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Test that system service works even with noop cache
	testKey := "noop_test_key"
	testValue := "noop_test_value"

	err := service.setSystemValue(ctx, testKey, testValue)
	require.NoError(t, err)

	retrievedValue, err := service.getSystemValue(ctx, testKey)
	require.NoError(t, err)
	require.Equal(t, testValue, retrievedValue)

	// Cache should be noop, so every call hits database
	require.Equal(t, "noop", service.Cache.GetType())
}

func TestSystemService_StoragePolicy(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// First set a default storage policy to avoid JSON unmarshaling error
	defaultPolicy := &StoragePolicy{
		StoreChunks:       false,
		StoreRequestBody:  true,
		StoreResponseBody: true,
		CleanupOptions: []CleanupOption{
			{
				ResourceType: "requests",
				Enabled:      false,
				CleanupDays:  3,
			},
			{
				ResourceType: "usage_logs",
				Enabled:      false,
				CleanupDays:  30,
			},
		},
	}

	err := service.SetStoragePolicy(ctx, defaultPolicy)
	require.NoError(t, err)

	// Test getting the storage policy
	policy, err := service.StoragePolicy(ctx)
	require.NoError(t, err)
	require.False(t, policy.StoreChunks)
	require.True(t, policy.StoreRequestBody)
	require.True(t, policy.StoreResponseBody)
	require.Len(t, policy.CleanupOptions, 2)

	// Test setting custom storage policy
	customPolicy := &StoragePolicy{
		StoreChunks:       true,
		StoreRequestBody:  false,
		StoreResponseBody: true,
		CleanupOptions: []CleanupOption{
			{
				ResourceType: "custom_resource",
				Enabled:      true,
				CleanupDays:  7,
			},
		},
	}

	err = service.SetStoragePolicy(ctx, customPolicy)
	require.NoError(t, err)

	retrievedPolicy, err := service.StoragePolicy(ctx)
	require.NoError(t, err)
	require.Equal(t, customPolicy.StoreChunks, retrievedPolicy.StoreChunks)
	require.Equal(t, customPolicy.StoreRequestBody, retrievedPolicy.StoreRequestBody)
	require.Equal(t, customPolicy.StoreResponseBody, retrievedPolicy.StoreResponseBody)
	require.Len(t, retrievedPolicy.CleanupOptions, 1)
	require.Equal(t, "custom_resource", retrievedPolicy.CleanupOptions[0].ResourceType)

	// Test StoreChunks convenience method
	storeChunks, err := service.StoreChunks(ctx)
	require.NoError(t, err)
	require.True(t, storeChunks)
}

func TestSystemService_Initialize_WithCache(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	cacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xcache.RedisConfig{
			Addr: mr.Addr(),
		},
	}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	// Test system initialization with cache
	args := &InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "securepassword123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	}

	err := service.Initialize(ctx, args)
	require.NoError(t, err)

	// Verify system is initialized
	isInitialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	require.True(t, isInitialized)

	// Verify secret key is cached
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	secretKey, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, secretKey)
	require.Len(t, secretKey, 64)

	// Verify brand name is set and cached
	brandName, err := service.BrandName(ctx)
	require.NoError(t, err)
	require.Equal(t, args.BrandName, brandName)

	// Test idempotency with cache
	err = service.Initialize(ctx, args)
	require.NoError(t, err)

	// Values should remain the same
	secretKey2, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.Equal(t, secretKey, secretKey2)
}

func TestSystemService_CacheExpiration(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	cacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xcache.RedisConfig{
			Addr:       mr.Addr(),
			Expiration: 100 * time.Millisecond, // Very short for testing
		},
	}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Set a test value
	testKey := "expiration_test"
	testValue := "expiration_value"

	err := service.setSystemValue(ctx, testKey, testValue)
	require.NoError(t, err)

	// First call should cache the result
	retrievedValue, err := service.getSystemValue(ctx, testKey)
	require.NoError(t, err)
	require.Equal(t, testValue, retrievedValue)

	// Wait for cache expiration
	time.Sleep(150 * time.Millisecond)

	// Should still work (will hit database again)
	retrievedValue2, err := service.getSystemValue(ctx, testKey)
	require.NoError(t, err)
	require.Equal(t, testValue, retrievedValue2)
}

func TestSystemService_InvalidStoragePolicyJSON(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Manually insert invalid JSON for storage policy
	_, err := client.System.Create().
		SetKey(SystemKeyStoragePolicy).
		SetValue("invalid-json").
		Save(ctx)
	require.NoError(t, err)

	// Should return error when trying to parse invalid JSON
	_, err = service.StoragePolicy(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal storage policy")
}

func TestSystemService_BackwardCompatibility(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create old-style storage policy without new fields
	oldPolicy := map[string]interface{}{
		"store_chunks": true,
		"cleanup_options": []map[string]interface{}{
			{
				"resource_type": "requests",
				"enabled":       true,
				"cleanup_days":  5,
			},
		},
	}

	oldPolicyJSON, err := json.Marshal(oldPolicy)
	require.NoError(t, err)

	_, err = client.System.Create().
		SetKey(SystemKeyStoragePolicy).
		SetValue(string(oldPolicyJSON)).
		Save(ctx)
	require.NoError(t, err)

	// Should handle backward compatibility
	policy, err := service.StoragePolicy(ctx)
	require.NoError(t, err)
	require.True(t, policy.StoreChunks)
	require.True(t, policy.StoreRequestBody)  // Should default to true
	require.True(t, policy.StoreResponseBody) // Should default to true
	require.Len(t, policy.CleanupOptions, 1)
}

func TestSystemService_GetSystemValue_NotFound(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Try to get non-existent key
	value, err := service.getSystemValue(ctx, "non-existent-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get system value")
	require.Empty(t, value) // Should return empty string when error occurs
}

func TestSystemService_BrandName_NotSet(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Brand name not set should return empty string
	brandName, err := service.BrandName(ctx)
	require.NoError(t, err)
	require.Empty(t, brandName)
}

func TestSystemService_BrandLogo_NotSet(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	// Brand logo not set should return empty string
	brandLogo, err := service.BrandLogo(ctx)
	require.NoError(t, err)
	require.Empty(t, brandLogo)
}
