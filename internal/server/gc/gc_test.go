package gc

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestWorker_getBatchSize(t *testing.T) {
	worker := &Worker{
		Ent:    nil,
		Config: Config{CRON: "0 0 * * *"},
	}

	// Test default batch size
	batchSize := worker.getBatchSize()
	if batchSize != defaultBatchSize {
		t.Errorf("Expected batch size %d, got %d", defaultBatchSize, batchSize)
	}

	// Test with overridden batch size
	originalBatchSize := defaultBatchSize
	defaultBatchSize = 20

	defer func() { defaultBatchSize = originalBatchSize }()

	batchSize = worker.getBatchSize()
	if batchSize != 20 {
		t.Errorf("Expected batch size 20, got %d", batchSize)
	}
}

func TestWorker_cleanupRequestExternalStorageDeletesFsArtifacts(t *testing.T) {
	worker, ctx, dataStorage, baseDir := setupWorkerWithFSStorage(t)

	req := &ent.Request{
		ID:            101,
		ProjectID:     202,
		DataStorageID: dataStorage.ID,
	}

	fileKeys := []string{
		biz.GenerateRequestBodyKey(req.ProjectID, req.ID),
		biz.GenerateResponseBodyKey(req.ProjectID, req.ID),
		biz.GenerateResponseChunksKey(req.ProjectID, req.ID),
	}

	dirKeys := []string{
		biz.GenerateRequestExecutionsDirKey(req.ProjectID, req.ID),
		biz.GenerateRequestDirKey(req.ProjectID, req.ID),
	}

	for _, key := range fileKeys {
		createFileForKey(t, baseDir, key)
	}

	for _, key := range dirKeys {
		createDirForKey(t, baseDir, key)
	}

	worker.cleanupRequestExternalStorage(ctx, req, make(map[int]*ent.DataStorage))

	for _, key := range append(fileKeys, dirKeys...) {
		assertRemoved(t, baseDir, key)
	}
}

func TestWorker_cleanupExecutionExternalStorageDeletesFsArtifacts(t *testing.T) {
	worker, ctx, dataStorage, baseDir := setupWorkerWithFSStorage(t)

	req := &ent.Request{
		ID:            303,
		ProjectID:     404,
		DataStorageID: dataStorage.ID,
	}

	exec := &ent.RequestExecution{
		ID:            505,
		RequestID:     req.ID,
		ProjectID:     req.ProjectID,
		DataStorageID: dataStorage.ID,
	}

	fileKeys := []string{
		biz.GenerateExecutionRequestBodyKey(exec.ProjectID, exec.RequestID, exec.ID),
		biz.GenerateExecutionResponseBodyKey(exec.ProjectID, exec.RequestID, exec.ID),
		biz.GenerateExecutionResponseChunksKey(exec.ProjectID, exec.RequestID, exec.ID),
	}

	dirKeys := []string{
		biz.GenerateExecutionRequestDirKey(exec.ProjectID, exec.RequestID, exec.ID),
	}

	for _, key := range fileKeys {
		createFileForKey(t, baseDir, key)
	}

	for _, key := range dirKeys {
		createDirForKey(t, baseDir, key)
	}

	worker.cleanupExecutionExternalStorage(ctx, exec, make(map[int]*ent.DataStorage))

	for _, key := range append(fileKeys, dirKeys...) {
		assertRemoved(t, baseDir, key)
	}
}

func setupWorkerWithFSStorage(t *testing.T) (*Worker, context.Context, *ent.DataStorage, string) {
	t.Helper()

	cacheConfig := xcache.Config{
		Mode: xcache.ModeMemory,
		Memory: xcache.MemoryConfig{
			Expiration:      5 * time.Minute,
			CleanupInterval: 10 * time.Minute,
		},
	}

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	executor := executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1))

	t.Cleanup(func() {
		_ = executor.Shutdown(context.Background())

		client.Close()
	})

	systemService := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cacheConfig})
	dataStorageService := biz.NewDataStorageService(biz.DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   cacheConfig,
		Executor:      executor,
		Client:        client,
	})

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	dir := t.TempDir()
	dirCopy := dir
	settings := &objects.DataStorageSettings{Directory: &dirCopy}

	dataStorage, err := client.DataStorage.Create().
		SetName("fs-storage").
		SetDescription("test fs storage").
		SetPrimary(false).
		SetType(datastorage.TypeFs).
		SetSettings(settings).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	worker := &Worker{
		DataStorageService: dataStorageService,
		Ent:                client,
	}

	return worker, ctx, dataStorage, dir
}

func createFileForKey(t *testing.T, baseDir, key string) {
	t.Helper()

	path := pathForKey(baseDir, key)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("test"), 0o644))
}

func createDirForKey(t *testing.T, baseDir, key string) {
	t.Helper()

	path := pathForKey(baseDir, key)
	require.NoError(t, os.MkdirAll(path, 0o755))
}

func assertRemoved(t *testing.T, baseDir, key string) {
	t.Helper()

	path := pathForKey(baseDir, key)
	_, err := os.Stat(path)
	require.ErrorIs(t, err, fs.ErrNotExist, "expected %s to be removed", key)
}

func pathForKey(baseDir, key string) string {
	rel := strings.TrimPrefix(key, "/")
	return filepath.Join(baseDir, filepath.FromSlash(rel))
}

func TestWorker_deleteInBatches(t *testing.T) {
	// Test that the deleteInBatches method works correctly
	// This test verifies the loop logic without needing a real database
	worker := &Worker{
		Ent:    nil,
		Config: Config{CRON: "0 0 * * *"},
	}

	// Simulate batch deletion - delete 3 times, with decreasing counts
	callCount := 0
	deleteFunc := func() (int, error) {
		callCount++
		if callCount == 1 {
			return 30, nil
		} else if callCount == 2 {
			return 15, nil
		} else {
			return 0, nil
		}
	}

	deleted, err := worker.deleteInBatches(context.Background(), deleteFunc)
	if err != nil {
		t.Fatalf("deleteInBatches failed: %v", err)
	}

	// Verify total deleted
	if deleted != 45 {
		t.Errorf("Expected to delete 45 records total, got %d", deleted)
	}

	// Verify it stopped after third call (when 0 was returned)
	if callCount != 3 {
		t.Errorf("Expected 3 delete calls, got %d", callCount)
	}
}

func TestWorker_cleanupWithZeroDays(t *testing.T) {
	worker := &Worker{
		Ent:    nil,
		Config: Config{CRON: "0 0 * * *"},
	}

	ctx := context.Background()

	// Test with 0 days - should not error
	err := worker.cleanupRequests(ctx, 0)
	if err != nil {
		t.Fatalf("cleanupRequests with 0 days failed: %v", err)
	}

	// Test with negative days - should not error
	err = worker.cleanupUsageLogs(ctx, -1)
	if err != nil {
		t.Fatalf("cleanupUsageLogs with negative days failed: %v", err)
	}
}
