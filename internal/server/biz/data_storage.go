package biz

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// DataStorageService handles data storage operations.
type DataStorageService struct {
	SystemService *SystemService
}

// NewDataStorageService creates a new DataStorageService.
func NewDataStorageService(systemService *SystemService) *DataStorageService {
	return &DataStorageService{
		SystemService: systemService,
	}
}

// GetPrimaryDataStorage returns the primary data storage.
func (s *DataStorageService) GetPrimaryDataStorage(ctx context.Context) (*ent.DataStorage, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	ds, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		First(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary data storage: %w", err)
	}

	return ds, nil
}

// GetDefaultDataStorage returns the default data storage configured in system settings.
// If no default is configured, it returns the primary data storage.
func (s *DataStorageService) GetDefaultDataStorage(ctx context.Context) (*ent.DataStorage, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	// Try to get default data storage ID from system settings
	defaultID, err := s.SystemService.DefaultDataStorageID(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// No default configured, use primary
			return s.GetPrimaryDataStorage(ctx)
		}

		return nil, fmt.Errorf("failed to get default data storage ID: %w", err)
	}

	// Get the data storage by ID
	ds, err := client.DataStorage.Get(ctx, defaultID)
	if err != nil {
		if ent.IsNotFound(err) {
			// Configured storage not found, fall back to primary
			return s.GetPrimaryDataStorage(ctx)
		}

		return nil, fmt.Errorf("failed to get data storage: %w", err)
	}

	return ds, nil
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
