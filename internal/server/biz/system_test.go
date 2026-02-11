package biz

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
)

func TestSystemService_GetSecretKey_NotInitialized(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)

	// Getting secret key before initialization should return error
	ctx = authz.WithTestBypass(ctx)
	secretKey, err := service.SecretKey(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSystemNotInitialized)
	require.Contains(t, err.Error(), "secret key not found")
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
	ctx = authz.WithTestBypass(ctx)

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
		Redis: xredis.Config{
			Addr: mr.Addr(),
		},
	}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

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
		Redis: xredis.Config{
			Addr: mr.Addr(),
		},
	}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

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
	ctx = authz.WithTestBypass(ctx)

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
	ctx = authz.WithTestBypass(ctx)

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
		Redis: xredis.Config{
			Addr: mr.Addr(),
		},
	}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	// Test system initialization with cache
	params := &InitializeSystemParams{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "securepassword123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	}

	err := service.Initialize(ctx, params)
	require.NoError(t, err)

	// Verify system is initialized
	isInitialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	require.True(t, isInitialized)

	// Verify secret key is cached
	ctx = authz.WithTestBypass(ctx)
	secretKey, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, secretKey)
	require.Len(t, secretKey, 64)

	// Verify brand name is set and cached
	brandName, err := service.BrandName(ctx)
	require.NoError(t, err)
	require.Equal(t, params.BrandName, brandName)

	// Test idempotency with cache
	err = service.Initialize(ctx, params)
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
		Redis: xredis.Config{
			Addr:       mr.Addr(),
			Expiration: 100 * time.Millisecond, // Very short for testing
		},
	}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

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
	ctx = authz.WithTestBypass(ctx)

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
	ctx = authz.WithTestBypass(ctx)

	// Create old-style storage policy without new fields
	oldPolicy := map[string]any{
		"store_chunks": true,
		"cleanup_options": []map[string]any{
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
	ctx = authz.WithTestBypass(ctx)

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
	ctx = authz.WithTestBypass(ctx)

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

func TestSystemService_Version(t *testing.T) {
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Test getting version when not set
	version, err := service.Version(ctx)
	require.NoError(t, err)
	require.Empty(t, version)

	// Test setting version
	testVersion := "v0.4.0"
	err = service.SetVersion(ctx, testVersion)
	require.NoError(t, err)

	// Test getting version after setting
	retrievedVersion, err := service.Version(ctx)
	require.NoError(t, err)
	require.Equal(t, testVersion, retrievedVersion)

	// Test updating version
	newVersion := "v0.5.0"
	err = service.SetVersion(ctx, newVersion)
	require.NoError(t, err)

	retrievedVersion2, err := service.Version(ctx)
	require.NoError(t, err)
	require.Equal(t, newVersion, retrievedVersion2)
}

func TestSystemService_Version_WithCache(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	cacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xredis.Config{
			Addr: mr.Addr(),
		},
	}

	service, client := setupTestSystemService(t, cacheConfig)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Set version
	testVersion := "v0.4.0"
	err := service.SetVersion(ctx, testVersion)
	require.NoError(t, err)

	// First call should hit database and cache the result
	retrievedVersion, err := service.Version(ctx)
	require.NoError(t, err)
	require.Equal(t, testVersion, retrievedVersion)

	// Second call should hit cache
	retrievedVersion2, err := service.Version(ctx)
	require.NoError(t, err)
	require.Equal(t, testVersion, retrievedVersion2)

	// Update version should invalidate cache
	newVersion := "v0.5.0"
	err = service.SetVersion(ctx, newVersion)
	require.NoError(t, err)

	// Should get updated version
	retrievedVersion3, err := service.Version(ctx)
	require.NoError(t, err)
	require.Equal(t, newVersion, retrievedVersion3)
}

func TestSystemService_Initialize_DataMigrationIdempotency(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)

	// First initialization
	err := service.Initialize(ctx, &InitializeSystemParams{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	ctx = authz.WithTestBypass(ctx)

	// Get initial data storage ID
	initialID, err := service.DefaultDataStorageID(ctx)
	require.NoError(t, err)

	// Second initialization (should be idempotent)
	err = service.Initialize(ctx, &InitializeSystemParams{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Verify data storage ID hasn't changed
	currentID, err := service.DefaultDataStorageID(ctx)
	require.NoError(t, err)
	require.Equal(t, initialID, currentID)

	// Verify only one primary data storage exists
	count, err := client.DataStorage.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestSystemService_Initialize_CreatesDefaultProject(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)

	// Initialize system
	err := service.Initialize(ctx, &InitializeSystemParams{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	ctx = authz.WithTestBypass(ctx)

	// Verify default project was created
	project, err := client.Project.Query().First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Default", project.Name)

	// Verify owner is assigned to the project
	userProject, err := client.UserProject.Query().First(ctx)
	require.NoError(t, err)
	require.True(t, userProject.IsOwner)

	// Verify default roles were created
	roles, err := client.Role.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, roles, 3) // Admin, Developer, Viewer
}

func TestSystemService_Initialize_SetsAllSystemKeys(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)

	// Initialize system
	err := service.Initialize(ctx, &InitializeSystemParams{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	ctx = authz.WithTestBypass(ctx)

	// Verify all system keys are set
	systemKeys := []string{
		SystemKeyInitialized,
		SystemKeyVersion,
		SystemKeySecretKey,
		SystemKeyBrandName,
		SystemKeyDefaultDataStorage,
	}

	for _, key := range systemKeys {
		sys, err := client.System.Query().Where().First(ctx)
		require.NoError(t, err, "key %s should exist", key)
		require.NotNil(t, sys)
	}

	// Verify brand name is set correctly
	brandName, err := service.BrandName(ctx)
	require.NoError(t, err)
	require.Equal(t, "Test Brand", brandName)

	// Verify secret key is set and has correct length
	secretKey, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.Len(t, secretKey, 64)
}

func TestSystemService_DefaultDataStorageID(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Test getting default data storage ID when not set (should return 0)
	defaultID, err := service.DefaultDataStorageID(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, defaultID)

	// Create a data storage
	ds, err := client.DataStorage.Create().
		SetName("Test Storage").
		SetDescription("Test storage").
		SetPrimary(true).
		SetType("database").
		SetSettings(&objects.DataStorageSettings{}).
		SetStatus("active").
		Save(ctx)
	require.NoError(t, err)

	// Set default data storage ID
	err = service.SetDefaultDataStorageID(ctx, ds.ID)
	require.NoError(t, err)

	// Get default data storage ID
	retrievedID, err := service.DefaultDataStorageID(ctx)
	require.NoError(t, err)
	require.Equal(t, ds.ID, retrievedID)
}

func TestSystemService_Initialize_TransactionRollback(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// First, create a user with the same email to cause a constraint violation
	_, err := client.User.Create().
		SetEmail("owner@example.com").
		SetPassword("hashedpassword").
		SetFirstName("Existing").
		SetLastName("User").
		SetIsOwner(false).
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)

	// Try to initialize with duplicate email (should fail due to unique constraint)
	err = service.Initialize(ctx, &InitializeSystemParams{
		OwnerEmail:     "owner@example.com", // Duplicate email
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.Error(t, err)

	// Verify system is not initialized (transaction rolled back)
	isInitialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	require.False(t, isInitialized)

	// Verify only the original user exists (no owner created)
	userCount, err := client.User.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, userCount)

	// Verify the existing user is not an owner
	user, err := client.User.Query().First(ctx)
	require.NoError(t, err)
	require.False(t, user.IsOwner)

	// Verify no projects were created
	projectCount, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, projectCount)

	// Verify no data storages were created
	dsCount, err := client.DataStorage.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, dsCount)
}
