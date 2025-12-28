package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupDataStorageTest(t *testing.T) (*ent.Client, *DataStorageService, context.Context) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	// Setup cache config for testing
	cacheConfig := xcache.Config{
		Mode: xcache.ModeMemory,
		Memory: xcache.MemoryConfig{
			Expiration:      5 * time.Minute,
			CleanupInterval: 10 * time.Minute,
		},
	}

	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: cacheConfig,
	})

	executor := executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1))

	t.Cleanup(func() {
		_ = executor.Shutdown(context.Background())
	})

	service := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   cacheConfig,
		Executor:      executor,
		Client:        client,
	})

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	return client, service, ctx
}

func setupDataStorageTestWithRedis(t *testing.T) (*ent.Client, *DataStorageService, context.Context, *miniredis.Miniredis) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	// Setup miniredis for testing
	mr := miniredis.RunT(t)

	cacheConfig := xcache.Config{
		Mode: xcache.ModeRedis,
		Redis: xcache.RedisConfig{
			Addr:       mr.Addr(),
			Expiration: 5 * time.Minute,
		},
	}

	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: cacheConfig,
	})

	executor := executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1))

	t.Cleanup(func() {
		_ = executor.Shutdown(context.Background())
	})

	service := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   cacheConfig,
		Executor:      executor,
		Client:        client,
	})

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	return client, service, ctx, mr
}

func createTestDataStorage(t *testing.T, client *ent.Client, ctx context.Context, name string, primary bool, dsType datastorage.Type) *ent.DataStorage {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	settings := &objects.DataStorageSettings{}

	if dsType == datastorage.TypeFs {
		dir := "/tmp/test"
		settings.Directory = &dir
	}

	// Add timestamp to make name unique
	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	ds, err := client.DataStorage.Create().
		SetName(uniqueName).
		SetDescription("Test data storage").
		SetPrimary(primary).
		SetType(dsType).
		SetSettings(settings).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	return ds
}

func TestDataStorageService_CreateDataStorage(t *testing.T) {
	client, service, ctx := setupDataStorageTest(t)
	defer client.Close()

	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Seed cache entries to ensure they get cleared after creation.
	require.NoError(t, service.Cache.Set(ctx, "datastorage:123", ent.DataStorage{ID: 123}))
	require.NoError(t, service.Cache.Set(ctx, "datastorage:primary", ent.DataStorage{ID: 999}))

	status := datastorage.StatusActive
	input := ent.CreateDataStorageInput{
		Name:        "create-storage",
		Description: "Test storage",
		Type:        datastorage.TypeDatabase,
		Settings:    &objects.DataStorageSettings{},
		Status:      &status,
	}

	created, err := service.CreateDataStorage(ctx, &input)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, input.Name, created.Name)
	require.Equal(t, input.Description, created.Description)
	require.False(t, created.Primary)
	require.Equal(t, input.Type, created.Type)
	require.Equal(t, datastorage.StatusActive, created.Status)

	// Cache should be cleared by CreateDataStorage.
	_, err = service.Cache.Get(ctx, "datastorage:123")
	require.Error(t, err)
	_, err = service.Cache.Get(ctx, "datastorage:primary")
	require.Error(t, err)
}

func TestDataStorageService_UpdateDataStorage(t *testing.T) {
	t.Run("updates basic fields and invalidates caches", func(t *testing.T) {
		client, service, ctx := setupDataStorageTest(t)
		defer client.Close()

		ctx = privacy.DecisionContext(ctx, privacy.Allow)

		original := createTestDataStorage(t, client, ctx, "update-storage", true, datastorage.TypeDatabase)

		_, err := service.GetDataStorageByID(ctx, original.ID)
		require.NoError(t, err)
		_, err = service.GetPrimaryDataStorage(ctx)
		require.NoError(t, err)

		newName := "updated-storage"
		status := datastorage.StatusActive
		updateInput := ent.UpdateDataStorageInput{
			Name:   &newName,
			Status: &status,
		}

		updated, err := service.UpdateDataStorage(ctx, original.ID, &updateInput)
		require.NoError(t, err)
		require.NotNil(t, updated)
		require.Equal(t, newName, updated.Name)
		require.True(t, updated.Primary)
		require.Equal(t, status, updated.Status)
		require.NotNil(t, updated.Settings)

		_, err = service.Cache.Get(ctx, fmt.Sprintf("datastorage:%d", original.ID))
		require.Error(t, err)
		_, err = service.Cache.Get(ctx, "datastorage:primary")
		require.Error(t, err)
	})

	t.Run("preserves existing credentials when not provided", func(t *testing.T) {
		client, service, ctx := setupDataStorageTest(t)
		defer client.Close()

		ctx = privacy.DecisionContext(ctx, privacy.Allow)

		existingDirectory := "/existing/path"
		existingDSN := "existing-dsn"
		existingAccess := "existing-access"
		existingSecret := "existing-secret"
		existingGCSCredential := "existing-gcs-cred"

		existingSettings := &objects.DataStorageSettings{
			Directory: stringPtr(existingDirectory),
			DSN:       stringPtr(existingDSN),
			S3: &objects.S3{
				BucketName: "existing-bucket",
				Endpoint:   "existing-endpoint",
				Region:     "existing-region",
				AccessKey:  existingAccess,
				SecretKey:  existingSecret,
			},
			GCS: &objects.GCS{
				BucketName: "existing-gcs-bucket",
				Credential: existingGCSCredential,
			},
		}

		ctxWithDecision := privacy.DecisionContext(ctx, privacy.Allow)
		original, err := client.DataStorage.Create().
			SetName("storage-with-creds").
			SetDescription("Data storage with credentials").
			SetPrimary(false).
			SetType(datastorage.TypeS3).
			SetSettings(existingSettings).
			SetStatus(datastorage.StatusActive).
			Save(ctxWithDecision)
		require.NoError(t, err)

		updateInput := ent.UpdateDataStorageInput{
			Settings: &objects.DataStorageSettings{
				S3: &objects.S3{
					BucketName: "updated-bucket",
					Endpoint:   "",
					Region:     "updated-region",
					AccessKey:  "",
					SecretKey:  "",
				},
				GCS: &objects.GCS{
					BucketName: "updated-gcs-bucket",
					Credential: "",
				},
			},
		}

		updated, err := service.UpdateDataStorage(ctx, original.ID, &updateInput)
		require.NoError(t, err)
		require.NotNil(t, updated)
		require.NotNil(t, updated.Settings)

		require.NotNil(t, updated.Settings.Directory)
		require.Equal(t, existingDirectory, *updated.Settings.Directory)
		require.NotNil(t, updated.Settings.DSN)
		require.Equal(t, existingDSN, *updated.Settings.DSN)

		require.NotNil(t, updated.Settings.S3)
		require.Equal(t, "updated-bucket", updated.Settings.S3.BucketName)
		require.Equal(t, "", updated.Settings.S3.Endpoint)
		require.Equal(t, "updated-region", updated.Settings.S3.Region)
		require.Equal(t, existingAccess, updated.Settings.S3.AccessKey)
		require.Equal(t, existingSecret, updated.Settings.S3.SecretKey)

		require.NotNil(t, updated.Settings.GCS)
		require.Equal(t, "updated-gcs-bucket", updated.Settings.GCS.BucketName)
		require.Equal(t, existingGCSCredential, updated.Settings.GCS.Credential)
	})

	t.Run("overrides credentials when provided", func(t *testing.T) {
		client, service, ctx := setupDataStorageTest(t)
		defer client.Close()

		ctx = privacy.DecisionContext(ctx, privacy.Allow)

		existingSettings := &objects.DataStorageSettings{
			Directory: stringPtr("/existing/path"),
			DSN:       stringPtr("existing-dsn"),
			S3: &objects.S3{
				BucketName: "existing-bucket",
				Endpoint:   "existing-endpoint",
				Region:     "existing-region",
				AccessKey:  "old-access",
				SecretKey:  "old-secret",
			},
			GCS: &objects.GCS{
				BucketName: "existing-gcs-bucket",
				Credential: "old-gcs-cred",
			},
		}

		ctxWithDecision := privacy.DecisionContext(ctx, privacy.Allow)
		original, err := client.DataStorage.Create().
			SetName("storage-with-creds").
			SetDescription("Data storage with credentials").
			SetPrimary(false).
			SetType(datastorage.TypeS3).
			SetSettings(existingSettings).
			SetStatus(datastorage.StatusActive).
			Save(ctxWithDecision)
		require.NoError(t, err)

		updateInput := ent.UpdateDataStorageInput{
			Settings: &objects.DataStorageSettings{
				Directory: stringPtr("/new/path"),
				DSN:       stringPtr("new-dsn"),
				S3: &objects.S3{
					BucketName: "new-bucket",
					Endpoint:   "new-endpoint",
					Region:     "new-region",
					AccessKey:  "new-access",
					SecretKey:  "new-secret",
				},
				GCS: &objects.GCS{
					BucketName: "new-gcs-bucket",
					Credential: "new-gcs-cred",
				},
			},
		}

		updated, err := service.UpdateDataStorage(ctx, original.ID, &updateInput)
		require.NoError(t, err)
		require.NotNil(t, updated)
		require.NotNil(t, updated.Settings)

		require.NotNil(t, updated.Settings.Directory)
		require.Equal(t, "/new/path", *updated.Settings.Directory)
		require.NotNil(t, updated.Settings.DSN)
		require.Equal(t, "new-dsn", *updated.Settings.DSN)

		require.NotNil(t, updated.Settings.S3)
		require.Equal(t, "new-bucket", updated.Settings.S3.BucketName)
		require.Equal(t, "new-endpoint", updated.Settings.S3.Endpoint)
		require.Equal(t, "new-region", updated.Settings.S3.Region)
		require.Equal(t, "new-access", updated.Settings.S3.AccessKey)
		require.Equal(t, "new-secret", updated.Settings.S3.SecretKey)

		require.NotNil(t, updated.Settings.GCS)
		require.Equal(t, "new-gcs-bucket", updated.Settings.GCS.BucketName)
		require.Equal(t, "new-gcs-cred", updated.Settings.GCS.Credential)
	})

	t.Run("returns error when data storage is missing", func(t *testing.T) {
		client, service, ctx := setupDataStorageTest(t)
		defer client.Close()

		ctx = privacy.DecisionContext(ctx, privacy.Allow)

		cacheKey := "datastorage:999"
		require.NoError(t, service.Cache.Set(ctx, cacheKey, ent.DataStorage{ID: 999}))

		_, err := service.UpdateDataStorage(ctx, 999, &ent.UpdateDataStorageInput{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get data storage")

		_, err = service.Cache.Get(ctx, cacheKey)
		require.NoError(t, err)
	})
}

func TestDataStorageService_GetDataStorageByID(t *testing.T) {
	client, service, ctx := setupDataStorageTest(t)
	defer client.Close()

	// Create test data storage
	testDS := createTestDataStorage(t, client, ctx, "test-storage", false, datastorage.TypeDatabase)

	t.Run("get existing data storage", func(t *testing.T) {
		ds, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)
		require.Equal(t, testDS.ID, ds.ID)
		require.Equal(t, testDS.Name, ds.Name)
		require.Equal(t, testDS.Type, ds.Type)
	})

	t.Run("get data storage from cache", func(t *testing.T) {
		// First call should cache the result
		ds1, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)

		// Second call should return from cache
		ds2, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)

		require.Equal(t, ds1.ID, ds2.ID)
		require.Equal(t, ds1.Name, ds2.Name)
	})

	t.Run("get non-existent data storage", func(t *testing.T) {
		_, err := service.GetDataStorageByID(ctx, 99999)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get data storage by ID")
	})
}

func TestDataStorageService_GetDataStorageByID_WithRedis(t *testing.T) {
	client, service, ctx, mr := setupDataStorageTestWithRedis(t)
	defer client.Close()
	defer mr.Close()

	// Create test data storage
	testDS := createTestDataStorage(t, client, ctx, "test-storage", false, datastorage.TypeDatabase)

	t.Run("get data storage with redis cache", func(t *testing.T) {
		ds, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)
		require.Equal(t, testDS.ID, ds.ID)

		// Verify cache key exists in redis
		cacheKey := fmt.Sprintf("datastorage:%d", testDS.ID)
		exists := mr.Exists(cacheKey)
		require.True(t, exists)
	})
}

func TestDataStorageService_GetPrimaryDataStorage(t *testing.T) {
	client, service, ctx := setupDataStorageTest(t)
	defer client.Close()

	// Create primary data storage
	primaryDS := createTestDataStorage(t, client, ctx, "primary-storage", true, datastorage.TypeDatabase)

	t.Run("get primary data storage", func(t *testing.T) {
		ds, err := service.GetPrimaryDataStorage(ctx)
		require.NoError(t, err)
		require.Equal(t, primaryDS.ID, ds.ID)
		require.True(t, ds.Primary)
	})

	t.Run("get primary data storage from cache", func(t *testing.T) {
		// First call should cache the result
		ds1, err := service.GetPrimaryDataStorage(ctx)
		require.NoError(t, err)

		// Second call should return from cache
		ds2, err := service.GetPrimaryDataStorage(ctx)
		require.NoError(t, err)

		require.Equal(t, ds1.ID, ds2.ID)
		require.Equal(t, ds1.Name, ds2.Name)
	})

	t.Run("no primary data storage", func(t *testing.T) {
		// Delete the primary data storage
		ctx := privacy.DecisionContext(ctx, privacy.Allow)
		err := client.DataStorage.DeleteOneID(primaryDS.ID).Exec(ctx)
		require.NoError(t, err)

		// Clear cache
		err = service.InvalidateAllDataStorageCache(ctx)
		require.NoError(t, err)

		_, err = service.GetPrimaryDataStorage(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get primary data storage")
	})
}

func TestDataStorageService_GetDefaultDataStorage(t *testing.T) {
	client, service, ctx := setupDataStorageTest(t)
	defer client.Close()

	// Create primary and default data storages
	primaryDS := createTestDataStorage(t, client, ctx, "primary-storage", true, datastorage.TypeDatabase)
	defaultDS := createTestDataStorage(t, client, ctx, "default-storage", false, datastorage.TypeDatabase)

	t.Run("get default data storage when configured", func(t *testing.T) {
		// Mock system service to return default storage ID
		// This would require setting up system configuration
		// For now, test fallback to primary
		ds, err := service.GetDefaultDataStorage(ctx)
		require.NoError(t, err)
		require.Equal(t, primaryDS.ID, ds.ID)
	})

	t.Run("fallback to primary when no default configured", func(t *testing.T) {
		ds, err := service.GetDefaultDataStorage(ctx)
		require.NoError(t, err)
		require.Equal(t, primaryDS.ID, ds.ID)
		require.True(t, ds.Primary)
	})

	_ = defaultDS // Use the variable to avoid unused warning
}

func TestDataStorageService_CacheInvalidation(t *testing.T) {
	client, service, ctx := setupDataStorageTest(t)
	defer client.Close()

	// Create test data storage
	testDS := createTestDataStorage(t, client, ctx, "test-storage", false, datastorage.TypeDatabase)

	t.Run("invalidate specific data storage cache", func(t *testing.T) {
		// First, cache the data storage
		ds1, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)

		// Invalidate cache
		err = service.InvalidateDataStorageCache(ctx, testDS.ID)
		require.NoError(t, err)

		// Next call should fetch from database again
		ds2, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)

		require.Equal(t, ds1.ID, ds2.ID)
	})

	t.Run("invalidate primary data storage cache", func(t *testing.T) {
		// Create primary data storage
		primaryDS := createTestDataStorage(t, client, ctx, "primary-storage", true, datastorage.TypeDatabase)

		// Cache the primary data storage
		_, err := service.GetPrimaryDataStorage(ctx)
		require.NoError(t, err)

		// Invalidate primary cache
		err = service.InvalidatePrimaryDataStorageCache(ctx)
		require.NoError(t, err)

		// Next call should work normally
		ds, err := service.GetPrimaryDataStorage(ctx)
		require.NoError(t, err)
		require.Equal(t, primaryDS.ID, ds.ID)
	})

	t.Run("invalidate all cache", func(t *testing.T) {
		// Cache some data
		_, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)

		// Clear all cache
		err = service.InvalidateAllDataStorageCache(ctx)
		require.NoError(t, err)

		// Next calls should work normally
		ds, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)
		require.Equal(t, testDS.ID, ds.ID)
	})
}

func TestDataStorageService_GetFileSystem(t *testing.T) {
	client, service, ctx := setupDataStorageTest(t)
	defer client.Close()

	t.Run("database storage should not support file system", func(t *testing.T) {
		dbDS := createTestDataStorage(t, client, ctx, "db-storage", false, datastorage.TypeDatabase)

		_, err := service.GetFileSystem(ctx, dbDS)
		require.Error(t, err)
		require.Contains(t, err.Error(), "database storage does not support file system operations")
	})

	t.Run("fs storage should return afero filesystem", func(t *testing.T) {
		fsDS := createTestDataStorage(t, client, ctx, "fs-storage", false, datastorage.TypeFs)

		fs, err := service.GetFileSystem(ctx, fsDS)
		require.NoError(t, err)
		require.NotNil(t, fs)

		// Verify it's a BasePathFs
		_, ok := fs.(*afero.BasePathFs)
		require.True(t, ok)
	})

	t.Run("fs storage without directory should fail", func(t *testing.T) {
		ctx := privacy.DecisionContext(ctx, privacy.Allow)
		fsDS, err := client.DataStorage.Create().
			SetName("fs-no-dir").
			SetDescription("FS storage without directory").
			SetPrimary(false).
			SetType(datastorage.TypeFs).
			SetSettings(&objects.DataStorageSettings{}).
			SetStatus(datastorage.StatusActive).
			Save(ctx)
		require.NoError(t, err)

		_, err = service.GetFileSystem(ctx, fsDS)
		require.Error(t, err)
		require.Contains(t, err.Error(), "directory not configured for fs storage")
	})

	t.Run("storage types without settings", func(t *testing.T) {
		s3DS := createTestDataStorage(t, client, ctx, "s3-storage", false, datastorage.TypeS3)
		gcsDS := createTestDataStorage(t, client, ctx, "gcs-storage", false, datastorage.TypeGcs)

		_, err := service.GetFileSystem(ctx, s3DS)
		require.Error(t, err)
		require.Contains(t, err.Error(), "s3 settings not configured")

		_, err = service.GetFileSystem(ctx, gcsDS)
		require.Error(t, err)
		require.Contains(t, err.Error(), "gcs settings not configured")
	})
}

func TestDataStorageService_SaveData(t *testing.T) {
	client, service, ctx := setupDataStorageTest(t)
	defer client.Close()

	testData := []byte("test data content")
	testKey := "test/key.txt"

	t.Run("save data to database storage", func(t *testing.T) {
		dbDS := createTestDataStorage(t, client, ctx, "db-storage", false, datastorage.TypeDatabase)

		result, err := service.SaveData(ctx, dbDS, testKey, testData)
		require.NoError(t, err)
		require.Equal(t, string(testData), result)
	})

	t.Run("save data to fs storage", func(t *testing.T) {
		fsDS := createTestDataStorage(t, client, ctx, "fs-storage", false, datastorage.TypeFs)

		result, err := service.SaveData(ctx, fsDS, testKey, testData)
		require.NoError(t, err)
		require.Equal(t, testKey, result)

		// Verify file was created
		fs, err := service.GetFileSystem(ctx, fsDS)
		require.NoError(t, err)

		exists, err := afero.Exists(fs, testKey)
		require.NoError(t, err)
		require.True(t, exists)

		// Verify content
		content, err := afero.ReadFile(fs, testKey)
		require.NoError(t, err)
		require.Equal(t, testData, content)
	})
}

func TestDataStorageService_LoadData(t *testing.T) {
	client, service, ctx := setupDataStorageTest(t)
	defer client.Close()

	testData := []byte("test data content")
	testKey := "test/key.txt"

	t.Run("load data from database storage", func(t *testing.T) {
		dbDS := createTestDataStorage(t, client, ctx, "db-storage", false, datastorage.TypeDatabase)

		// For database storage, the key is the data itself
		dataKey := string(testData)

		result, err := service.LoadData(ctx, dbDS, dataKey)
		require.NoError(t, err)
		require.Equal(t, testData, result)
	})

	t.Run("load data from fs storage", func(t *testing.T) {
		fsDS := createTestDataStorage(t, client, ctx, "fs-storage", false, datastorage.TypeFs)

		// First save data
		_, err := service.SaveData(ctx, fsDS, testKey, testData)
		require.NoError(t, err)

		// Then load it
		result, err := service.LoadData(ctx, fsDS, testKey)
		require.NoError(t, err)
		require.Equal(t, testData, result)
	})

	t.Run("load non-existent file from fs storage", func(t *testing.T) {
		fsDS := createTestDataStorage(t, client, ctx, "fs-storage-load-test", false, datastorage.TypeFs)

		_, err := service.LoadData(ctx, fsDS, "non-existent.txt")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read file")
	})
}

func TestDataStorageService_CacheExpiration(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	// Setup cache with very short expiration for testing
	cacheConfig := xcache.Config{
		Mode: xcache.ModeMemory,
		Memory: xcache.MemoryConfig{
			Expiration:      100 * time.Millisecond, // Very short for testing
			CleanupInterval: 50 * time.Millisecond,
		},
	}

	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: cacheConfig,
	})

	service := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   cacheConfig,
		Executor:      executors.NewPoolScheduleExecutor(),
		Client:        client,
	})

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	// Create test data storage
	testDS := createTestDataStorage(t, client, ctx, "test-storage", false, datastorage.TypeDatabase)

	t.Run("cache should expire after timeout", func(t *testing.T) {
		// First call should cache the result
		ds1, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)

		// Wait for cache to expire
		time.Sleep(200 * time.Millisecond)

		// Second call should fetch from database again (cache expired)
		ds2, err := service.GetDataStorageByID(ctx, testDS.ID)
		require.NoError(t, err)

		require.Equal(t, ds1.ID, ds2.ID)
	})
}
