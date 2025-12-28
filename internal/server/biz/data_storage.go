package biz

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/samber/lo"
	"github.com/spf13/afero"
	"github.com/spf13/afero/gcsfs"
	"github.com/zhenzou/executors"
	"go.uber.org/fx"
	"golang.org/x/oauth2/google"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3fs "github.com/looplj/afero-s3"
	googleoption "google.golang.org/api/option"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

// DataStorageService handles data storage operations.
type DataStorageService struct {
	*AbstractService

	SystemService *SystemService
	Cache         xcache.Cache[ent.DataStorage]
	Executors     executors.ScheduledExecutor

	// fsCache caches afero filesystem instances by data storage ID
	fsCache      map[int]afero.Fs
	fsCacheMu    sync.RWMutex
	latestUpdate time.Time
}

// DataStorageServiceParams holds the dependencies for DataStorageService.
type DataStorageServiceParams struct {
	fx.In

	SystemService *SystemService
	CacheConfig   xcache.Config
	Executor      executors.ScheduledExecutor
	Client        *ent.Client
}

// NewDataStorageService creates a new DataStorageService.
func NewDataStorageService(params DataStorageServiceParams) *DataStorageService {
	svc := &DataStorageService{
		AbstractService: &AbstractService{
			db: params.Client,
		},
		SystemService: params.SystemService,
		Cache:         xcache.NewFromConfig[ent.DataStorage](params.CacheConfig),
		Executors:     params.Executor,
		fsCache:       make(map[int]afero.Fs),
	}

	if err := svc.refreshFileSystems(context.Background()); err != nil {
		log.Error(context.Background(), "failed to preload data storage filesystems", log.Cause(err))
	}

	if _, err := svc.Executors.ScheduleFuncAtCronRate(
		svc.refreshFileSystemsPeriodic,
		executors.CRONRule{Expr: "*/1 * * * *"},
	); err != nil {
		log.Error(context.Background(), "failed to schedule data storage filesystem refresh", log.Cause(err))
	}

	return svc
}

func (s *DataStorageService) refreshFileSystemsPeriodic(ctx context.Context) {
	if err := s.refreshFileSystems(ctx); err != nil {
		log.Error(ctx, "failed to refresh data storage filesystems", log.Cause(err))
	}
}

func (s *DataStorageService) refreshFileSystems(ctx context.Context) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	latestUpdatedStorage, err := s.entFromContext(ctx).DataStorage.Query().
		Order(ent.Desc(datastorage.FieldUpdatedAt)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}

	if latestUpdatedStorage != nil {
		if !latestUpdatedStorage.UpdatedAt.After(s.latestUpdate) {
			log.Debug(ctx, "no data storage updates detected")
			return nil
		}

		s.latestUpdate = latestUpdatedStorage.UpdatedAt
	} else {
		s.latestUpdate = time.Time{}
	}

	storages, err := s.entFromContext(ctx).DataStorage.Query().
		Where(datastorage.StatusEQ(datastorage.StatusActive)).
		All(ctx)
	if err != nil {
		return err
	}

	newCache := make(map[int]afero.Fs, len(storages))

	for _, ds := range storages {
		if ds.Type == datastorage.TypeDatabase {
			continue
		}

		fs, err := s.buildFileSystem(ctx, ds)
		if err != nil {
			log.Warn(ctx, "failed to build data storage filesystem",
				log.Int("data_storage_id", ds.ID),
				log.String("data_storage_name", ds.Name),
				log.String("type", ds.Type.String()),
				log.Cause(err),
			)

			continue
		}

		newCache[ds.ID] = fs
	}

	s.fsCacheMu.Lock()
	s.fsCache = newCache
	s.fsCacheMu.Unlock()

	log.Info(ctx, "refreshed data storage filesystems", log.Int("count", len(newCache)))

	return nil
}

func (s *DataStorageService) buildFileSystem(ctx context.Context, ds *ent.DataStorage) (afero.Fs, error) {
	switch ds.Type {
	case datastorage.TypeDatabase:
		return nil, fmt.Errorf("database storage does not support file system operations")
	case datastorage.TypeFs:
		if ds.Settings == nil || ds.Settings.Directory == nil {
			return nil, fmt.Errorf("directory not configured for fs storage")
		}

		return afero.NewBasePathFs(afero.NewOsFs(), *ds.Settings.Directory), nil
	case datastorage.TypeS3:
		if ds.Settings == nil || ds.Settings.S3 == nil {
			return nil, fmt.Errorf("s3 settings not configured")
		}

		fs, err := s.createS3Fs(ctx, ds.Settings.S3)
		if err != nil {
			return nil, fmt.Errorf("failed to create s3 filesystem: %w", err)
		}

		return fs, nil
	case datastorage.TypeGcs:
		if ds.Settings == nil || ds.Settings.GCS == nil {
			return nil, fmt.Errorf("gcs settings not configured")
		}

		fs, err := s.createGcsFs(ctx, ds.Settings.GCS)
		if err != nil {
			return nil, fmt.Errorf("failed to create gcs filesystem: %w", err)
		}

		return fs, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", ds.Type)
	}
}

// CreateDataStorage creates a new data storage record and refreshes relevant caches.
func (s *DataStorageService) CreateDataStorage(ctx context.Context, input *ent.CreateDataStorageInput) (*ent.DataStorage, error) {
	dataStorage, err := ent.FromContext(ctx).DataStorage.Create().
		SetName(input.Name).
		SetSettings(input.Settings).
		SetDescription(input.Description).
		SetType(input.Type).
		SetPrimary(false).
		SetStatus(datastorage.StatusActive).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create data storage: %w", err)
	}

	// Clear caches so subsequent reads observe the latest data.
	_ = s.InvalidateAllDataStorageCache(ctx)
	_ = s.InvalidateFsCache(dataStorage.ID)

	return dataStorage, nil
}

// UpdateDataStorage updates an existing data storage record and refreshes relevant caches.
func (s *DataStorageService) UpdateDataStorage(ctx context.Context, id int, input *ent.UpdateDataStorageInput) (*ent.DataStorage, error) {
	// First, get the existing data storage to access current settings
	existing, err := ent.FromContext(ctx).DataStorage.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get data storage: %w", err)
	}

	// Build updated settings by merging with existing settings
	// Sensitive fields (credentials) are only updated if explicitly provided
	var updatedSettings *objects.DataStorageSettings
	if input.Settings != nil {
		updatedSettings = s.mergeSettings(existing.Settings, input.Settings)
	}

	mutation := ent.FromContext(ctx).DataStorage.
		UpdateOneID(id).
		SetNillableName(input.Name).
		SetNillableDescription(input.Description).
		SetNillableStatus(input.Status)

	if input.Settings != nil {
		mutation.SetSettings(updatedSettings)
	}

	dataStorage, err := mutation.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update data storage: %w", err)
	}

	_ = s.InvalidateDataStorageCache(ctx, dataStorage.ID)

	_ = s.InvalidateFsCache(dataStorage.ID)
	if dataStorage.Primary {
		_ = s.InvalidatePrimaryDataStorageCache(ctx)
	}

	return dataStorage, nil
}

// GetDataStorageByID returns a data storage by ID with caching support.
func (s *DataStorageService) GetDataStorageByID(ctx context.Context, id int) (*ent.DataStorage, error) {
	cacheKey := fmt.Sprintf("datastorage:%d", id)

	// Try to get from cache first
	if cached, err := s.Cache.Get(ctx, cacheKey); err == nil {
		return &cached, nil
	}

	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	ds, err := client.DataStorage.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get data storage by ID %d: %w", id, err)
	}

	// Cache the result for 30 minutes
	if err := s.Cache.Set(ctx, cacheKey, *ds, xcache.WithExpiration(30*time.Minute)); err != nil {
		// Log cache error but don't fail the request
		// Could add logging here if needed
	}

	return ds, nil
}

// GetPrimaryDataStorage returns the primary data storage.
func (s *DataStorageService) GetPrimaryDataStorage(ctx context.Context) (*ent.DataStorage, error) {
	cacheKey := "datastorage:primary"

	// Try to get from cache first
	if cached, err := s.Cache.Get(ctx, cacheKey); err == nil {
		return &cached, nil
	}

	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	ds, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		First(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary data storage: %w", err)
	}

	// Cache the result for 30 minutes
	if err := s.Cache.Set(ctx, cacheKey, *ds, xcache.WithExpiration(30*time.Minute)); err != nil {
		// Log cache error but don't fail the request
	}

	return ds, nil
}

// GetDefaultDataStorage returns the default data storage configured in system settings.
// If no default is configured, it returns the primary data storage.
func (s *DataStorageService) GetDefaultDataStorage(ctx context.Context) (*ent.DataStorage, error) {
	// Try to get default data storage ID from system settings
	defaultID, err := s.SystemService.DefaultDataStorageID(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// No default configured, use primary
			return s.GetPrimaryDataStorage(ctx)
		}

		return nil, fmt.Errorf("failed to get default data storage ID: %w", err)
	}

	// Get the data storage by ID using cached method
	ds, err := s.GetDataStorageByID(ctx, defaultID)
	if err != nil {
		if ent.IsNotFound(err) {
			// Configured storage not found, fall back to primary
			return s.GetPrimaryDataStorage(ctx)
		}

		return nil, fmt.Errorf("failed to get data storage: %w", err)
	}

	return ds, nil
}

// InvalidateDataStorageCache invalidates the cache for a specific data storage.
func (s *DataStorageService) InvalidateDataStorageCache(ctx context.Context, id int) error {
	cacheKey := fmt.Sprintf("datastorage:%d", id)
	return s.Cache.Delete(ctx, cacheKey)
}

// InvalidatePrimaryDataStorageCache invalidates the primary data storage cache.
func (s *DataStorageService) InvalidatePrimaryDataStorageCache(ctx context.Context) error {
	return s.Cache.Delete(ctx, "datastorage:primary")
}

// InvalidateAllDataStorageCache clears all data storage related cache entries.
func (s *DataStorageService) InvalidateAllDataStorageCache(ctx context.Context) error {
	// Also clear filesystem cache
	s.fsCacheMu.Lock()
	s.fsCache = make(map[int]afero.Fs)
	s.fsCacheMu.Unlock()

	s.latestUpdate = time.Time{}

	return s.Cache.Clear(ctx)
}

// InvalidateFsCache invalidates the cached filesystem for a specific data storage.
func (s *DataStorageService) InvalidateFsCache(id int) error {
	s.fsCacheMu.Lock()
	delete(s.fsCache, id)
	s.fsCacheMu.Unlock()

	return nil
}

// createS3Fs creates an S3 filesystem using the afero-s3 adapter.
func (s *DataStorageService) createS3Fs(ctx context.Context, s3Config *objects.S3) (afero.Fs, error) {
	credProvider := awscredentials.NewStaticCredentialsProvider(
		s3Config.AccessKey,
		s3Config.SecretKey,
		"",
	)

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(s3Config.Region),
		awsconfig.WithCredentialsProvider(credProvider),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if s3Config.Endpoint != "" {
			o.BaseEndpoint = lo.ToPtr(s3Config.Endpoint)
		}
	})

	baseFs := s3fs.NewFsFromClient(s3Config.BucketName, client)
	cachedFs := afero.NewCacheOnReadFs(baseFs, afero.NewMemMapFs(), 5*time.Minute)

	return cachedFs, nil
}

// createGcsFs creates a GCS filesystem using the afero gcsfs adapter.
func (s *DataStorageService) createGcsFs(ctx context.Context, gcsConfig *objects.GCS) (afero.Fs, error) {
	// Parse GCP credentials
	creds, err := google.CredentialsFromJSON(ctx, []byte(gcsConfig.Credential), storage.ScopeFullControl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GCP credentials: %w", err)
	}

	// Create GCS client
	client, err := storage.NewClient(context.Background(), googleoption.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	// Create GCS filesystem
	fs, err := gcsfs.NewGcsFSFromClient(context.Background(), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS filesystem: %w", err)
	}

	basePathFs := afero.NewBasePathFs(fs, gcsConfig.BucketName)

	cachedFs := afero.NewCacheOnReadFs(basePathFs, afero.NewMemMapFs(), 5*time.Minute)

	return cachedFs, nil
}

// GetFileSystem returns an afero.Fs for the given data storage.
// Filesystem instances are cached to avoid recreating them on every call.
func (s *DataStorageService) GetFileSystem(ctx context.Context, ds *ent.DataStorage) (afero.Fs, error) {
	// Check cache first
	s.fsCacheMu.RLock()

	if fs, ok := s.fsCache[ds.ID]; ok {
		s.fsCacheMu.RUnlock()
		return fs, nil
	}

	s.fsCacheMu.RUnlock()

	fs, err := s.buildFileSystem(ctx, ds)
	if err != nil {
		return nil, err
	}

	// Cache the filesystem instance
	s.fsCacheMu.Lock()
	s.fsCache[ds.ID] = fs
	s.fsCacheMu.Unlock()

	return fs, nil
}

// SaveData saves data to the specified data storage.
// For file system storage, it writes the data to a file and returns the file path.
func (s *DataStorageService) SaveData(ctx context.Context, ds *ent.DataStorage, key string, data []byte) (string, error) {
	switch ds.Type {
	case datastorage.TypeDatabase:
		// For database storage, we just return the data as a string
		// The caller will store it in the database
		return string(data), nil
	case datastorage.TypeFs, datastorage.TypeS3, datastorage.TypeGcs:
		// For file-based storage, write to file system
		fs, err := s.GetFileSystem(ctx, ds)
		if err != nil {
			return "", fmt.Errorf("failed to get file system: %w", err)
		}

		if ds.Type == datastorage.TypeFs {
			key = filepath.FromSlash(key)

			err = fs.MkdirAll(filepath.Dir(key), 0o777)
			if err != nil {
				return "", fmt.Errorf("failed to create directory: %w, key: %s", err, key)
			}
		} else {
			f, err := fs.Create(key)
			if err != nil {
				return "", fmt.Errorf("failed to create file: %w, key: %s", err, key)
			}

			_ = f.Close()
		}

		// Write data to file
		if err := afero.WriteFile(fs, key, data, 0o777); err != nil {
			return "", fmt.Errorf("failed to write file: %w, key: %s", err, key)
		}

		return key, nil
	default:
		return "", fmt.Errorf("unsupported storage type: %s", ds.Type)
	}
}

// DeleteData removes data stored under the provided key for the given data storage.
// It is a no-op for database storage because the data is kept in the database itself.
func (s *DataStorageService) DeleteData(ctx context.Context, ds *ent.DataStorage, key string) error {
	log.Debug(ctx, "Deleting data", log.String("key", key))

	switch ds.Type {
	case datastorage.TypeDatabase:
		return nil
	case datastorage.TypeFs, datastorage.TypeS3, datastorage.TypeGcs:
		fs, err := s.GetFileSystem(ctx, ds)
		if err != nil {
			return fmt.Errorf("failed to get file system: %w", err)
		}

		if ds.Type == datastorage.TypeFs {
			key = filepath.FromSlash(key)
		}

		if err := fs.Remove(key); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}

			return fmt.Errorf("failed to remove file: %w, key: %s", err, key)
		}

		return nil
	default:
		return fmt.Errorf("unsupported storage type: %s", ds.Type)
	}
}

// LoadData loads data from the specified data storage.
// For database storage, it expects the data to be passed directly.
// For file system storage, it reads the data from the file.
func (s *DataStorageService) LoadData(ctx context.Context, ds *ent.DataStorage, key string) ([]byte, error) {
	switch ds.Type {
	case datastorage.TypeDatabase:
		// For database storage, the key is the data itself
		return []byte(key), nil
	case datastorage.TypeFs, datastorage.TypeS3, datastorage.TypeGcs:
		// For file-based storage, read from file system
		fs, err := s.GetFileSystem(ctx, ds)
		if err != nil {
			return nil, fmt.Errorf("failed to get file system: %w", err)
		}

		if ds.Type == datastorage.TypeFs {
			key = filepath.FromSlash(key)
		}

		data, err := afero.ReadFile(fs, key)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		return data, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", ds.Type)
	}
}

// isS3Provided checks if any S3 field is provided in the input (non-empty).
func isS3Provided(s3 *objects.S3) bool {
	if s3 == nil {
		return false
	}

	return s3.BucketName != "" || s3.Endpoint != "" || s3.Region != "" ||
		s3.AccessKey != "" || s3.SecretKey != ""
}

// isGCSProvided checks if any GCS field is provided in the input (non-empty).
func isGCSProvided(gcs *objects.GCS) bool {
	if gcs == nil {
		return false
	}

	return gcs.BucketName != "" || gcs.Credential != ""
}

// mergeSettings merges existing and new settings, preserving sensitive fields
// that are empty in the new settings. This allows partial updates without
// accidentally clearing credentials.
func (s *DataStorageService) mergeSettings(existing, input *objects.DataStorageSettings) *objects.DataStorageSettings {
	// Start with the input settings as the base
	merged := &objects.DataStorageSettings{}

	// If no input, return existing
	if input == nil {
		return existing
	}

	// Merge Directory (non-sensitive)
	if input.Directory != nil {
		merged.Directory = input.Directory
	} else if existing != nil && existing.Directory != nil {
		merged.Directory = existing.Directory
	}

	// Merge DSN (sensitive for database)
	if input.DSN != nil {
		merged.DSN = input.DSN
	} else if existing != nil && existing.DSN != nil {
		merged.DSN = existing.DSN
	}

	// Merge S3 settings
	if isS3Provided(input.S3) {
		// Input S3 is provided, merge fields
		merged.S3 = &objects.S3{
			BucketName: input.S3.BucketName,
			Endpoint:   input.S3.Endpoint,
			Region:     input.S3.Region,
		}

		// Only update sensitive fields if they are non-empty
		if input.S3.AccessKey != "" {
			merged.S3.AccessKey = input.S3.AccessKey
		} else if existing.S3 != nil {
			merged.S3.AccessKey = existing.S3.AccessKey
		}

		if input.S3.SecretKey != "" {
			merged.S3.SecretKey = input.S3.SecretKey
		} else if existing.S3 != nil {
			merged.S3.SecretKey = existing.S3.SecretKey
		}
	} else if existing.S3 != nil {
		// No S3 input provided, preserve existing S3 config
		merged.S3 = &objects.S3{}
		*merged.S3 = *existing.S3
	}

	// Merge GCS settings
	if isGCSProvided(input.GCS) {
		// Input GCS is provided, merge fields
		merged.GCS = &objects.GCS{
			BucketName: input.GCS.BucketName,
		}

		// Only update sensitive field if it is non-empty
		if input.GCS.Credential != "" {
			merged.GCS.Credential = input.GCS.Credential
		} else if existing.GCS != nil {
			merged.GCS.Credential = existing.GCS.Credential
		}
	} else if existing.GCS != nil {
		// No GCS input provided, preserve existing GCS config
		merged.GCS = &objects.GCS{}
		*merged.GCS = *existing.GCS
	}

	return merged
}
