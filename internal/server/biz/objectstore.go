package biz

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
)

// ObjectStore is a minimal object-oriented blob API used natively for backends
// with real Class-A pricing (currently S3; GCS planned). Filesystem and WebDAV
// backends keep the afero path; Database is a no-op above this layer.
//
// Keys passed to an ObjectStore are already normalized via normalizeObjectKey
// (leading slash stripped), matching how the afero-s3 adapter sanitized keys so
// that objects written before this refactor remain readable/deletable.
type ObjectStore interface {
	// PutObject writes data in a single operation (S3 PutObject for in-memory
	// payloads), never Create+Close+Write.
	PutObject(ctx context.Context, key string, data []byte) error

	// PutObjectStream streams from r and returns the number of bytes written.
	// size < 0 means the length is unknown. The implementation switches to
	// multipart only when actually required (with a deliberate part size).
	PutObjectStream(ctx context.Context, key string, r io.Reader, size int64) (written int64, err error)

	// GetObject returns the object bytes. A missing key maps to an error
	// satisfying errors.Is(err, os.ErrNotExist) with no List fallback.
	GetObject(ctx context.Context, key string) ([]byte, error)

	// OpenObject returns a streaming reader and size for download paths.
	// A missing key maps to os.ErrNotExist with no List fallback.
	OpenObject(ctx context.Context, key string) (body io.ReadCloser, size int64, err error)

	// DeleteObject deletes one key. Deleting a non-existent key (including the
	// directory-marker keys that were never created, or chunk keys when
	// StoreChunks is off) is an idempotent no-op: a single DeleteObject with no
	// HeadObject and no ListObjectsV2.
	DeleteObject(ctx context.Context, key string) error
}

// normalizeObjectKey strips the leading slash from a storage key so native
// object-store operations target the same keys the afero-s3 adapter wrote to
// (its sanitize() stripped the leading slash regardless of PathStyle).
func normalizeObjectKey(key string) string {
	return strings.TrimPrefix(key, "/")
}

// objectStoreFor returns a native ObjectStore for backends that have one
// (currently S3 only). For all other backends it returns (nil, false, nil) so
// the caller falls back to the afero filesystem path or the Database no-op.
//
// Clients are cached by data storage ID alongside fsCache and invalidated in
// lockstep (see InvalidateFsCache / InvalidateAllDataStorageCache /
// refreshFileSystems). Building a client performs no S3 network call.
func (s *DataStorageService) objectStoreFor(ctx context.Context, ds *ent.DataStorage) (ObjectStore, bool, error) {
	if ds == nil || ds.Type != datastorage.TypeS3 {
		return nil, false, nil
	}

	if ds.Settings == nil || ds.Settings.S3 == nil {
		return nil, false, fmt.Errorf("s3 settings not configured")
	}

	s.fsCacheMu.RLock()
	if store, ok := s.objectStoreCache[ds.ID]; ok {
		s.fsCacheMu.RUnlock()
		return store, true, nil
	}
	s.fsCacheMu.RUnlock()

	store, err := newS3ObjectStore(ctx, ds.Settings.S3)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create s3 object store: %w", err)
	}

	s.fsCacheMu.Lock()
	s.objectStoreCache[ds.ID] = store
	s.fsCacheMu.Unlock()

	return store, true, nil
}
