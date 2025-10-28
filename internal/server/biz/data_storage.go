package biz

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

// DataStorageService handles data storage operations.
type DataStorageService struct {
	SystemService *SystemService
	Cache         xcache.Cache[ent.DataStorage]
}

// DataStorageServiceParams holds the dependencies for DataStorageService.
type DataStorageServiceParams struct {
	fx.In

	SystemService *SystemService
	CacheConfig   xcache.Config
}

// NewDataStorageService creates a new DataStorageService.
func NewDataStorageService(params DataStorageServiceParams) *DataStorageService {
	return &DataStorageService{
		SystemService: params.SystemService,
		Cache:         xcache.NewFromConfig[ent.DataStorage](params.CacheConfig),
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

	return dataStorage, nil
}

// UpdateDataStorage updates an existing data storage record and refreshes relevant caches.
func (s *DataStorageService) UpdateDataStorage(ctx context.Context, id int, input *ent.UpdateDataStorageInput) (*ent.DataStorage, error) {
	mutation := ent.FromContext(ctx).DataStorage.
		UpdateOneID(id).
		SetNillableName(input.Name).
		SetNillableDescription(input.Description).
		SetNillableStatus(input.Status)

	if input.Settings != nil {
		mutation.SetSettings(input.Settings)
	}

	dataStorage, err := mutation.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update data storage: %w", err)
	}

	_ = s.InvalidateDataStorageCache(ctx, dataStorage.ID)
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
	return s.Cache.Clear(ctx)
}

// GetFileSystem returns an afero.Fs for the given data storage.
func (s *DataStorageService) GetFileSystem(ctx context.Context, ds *ent.DataStorage) (afero.Fs, error) {
	switch ds.Type {
	case datastorage.TypeDatabase:
		// For database storage, we don't use afero
		return nil, fmt.Errorf("database storage does not support file system operations")
	case datastorage.TypeFs:
		// Local file system storage
		if ds.Settings.Directory == nil {
			return nil, fmt.Errorf("directory not configured for fs storage")
		}

		return afero.NewBasePathFs(afero.NewOsFs(), *ds.Settings.Directory), nil
	case datastorage.TypeS3:
		// S3 storage - would need to implement S3 afero adapter
		return nil, fmt.Errorf("s3 storage not yet implemented")
	case datastorage.TypeGcs:
		// GCS storage - would need to implement GCS afero adapter
		return nil, fmt.Errorf("gcs storage not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", ds.Type)
	}
}

// SaveData saves data to the specified data storage.
// For database storage, it returns the data as-is to be stored in the database.
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

		err = fs.MkdirAll(filepath.Dir(key), 0o777)
		if err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
		// Write data to file
		if err := afero.WriteFile(fs, key, data, 0o777); err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}

		// Return the file path/key
		return key, nil
	default:
		return "", fmt.Errorf("unsupported storage type: %s", ds.Type)
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

		// Read data from file
		data, err := afero.ReadFile(fs, key)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		return data, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", ds.Type)
	}
}
