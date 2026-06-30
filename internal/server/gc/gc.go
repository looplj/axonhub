package gc

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	"go.uber.org/fx"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channelprobe"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/ent/thread"
	"github.com/looplj/axonhub/internal/ent/trace"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

var defaultBatchSize = 500

type TriggerGcCleanupInput struct {
	RequestsCleanupDays  int `json:"requests_cleanup_days"`
	UsageLogsCleanupDays int `json:"usage_logs_cleanup_days"`
}

type GcCleanupPreviewItem struct {
	ResourceType   string    `json:"resource_type"`
	EstimatedCount int       `json:"estimated_count"`
	CutoffTime     time.Time `json:"cutoff_time"`
	RetentionDays  int       `json:"retention_days"`
}

type Config struct {
	CRON          string `json:"cron" yaml:"cron" conf:"cron" validate:"required"`
	VacuumEnabled bool   `json:"vacuum_enabled" yaml:"vacuum_enabled" conf:"vacuum_enabled"`
	VacuumFull    bool   `json:"vacuum_full" yaml:"vacuum_full" conf:"vacuum_full"`
}

type Worker struct {
	SystemService      *biz.SystemService
	DataStorageService *biz.DataStorageService
	Ent                *ent.Client
	Config             Config
}

type Params struct {
	fx.In

	Config             Config
	SystemService      *biz.SystemService
	DataStorageService *biz.DataStorageService
	Client             *ent.Client
}

func NewWorker(params Params) *Worker {
	w := &Worker{
		SystemService:      params.SystemService,
		DataStorageService: params.DataStorageService,
		Ent:                params.Client,
		Config:             params.Config,
	}

	return w
}

func (w *Worker) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "gc",
		Description: "Garbage collection — cleanup old requests, traces, usage logs, and channel probes",
		CronExpr:    w.Config.CRON,
		Timezone:    "UTC",
	}, w.runAutomaticCleanup)
}

// deleteInBatches deletes records in batches to avoid memory issues.
func (w *Worker) deleteInBatches(ctx context.Context, deleteFunc func() (int, error)) (int, error) {
	totalDeleted := 0

	for {
		deleted, err := deleteFunc()
		if err != nil {
			return totalDeleted, fmt.Errorf("failed to delete batch: %w", err)
		}

		if deleted == 0 {
			break
		}

		totalDeleted += deleted
		log.Debug(ctx, "Deleted batch of records", log.Int("batch_size", deleted), log.Int("total_deleted", totalDeleted))
	}

	return totalDeleted, nil
}

// getBatchSize returns the appropriate batch size for cleanup operations.
func (w *Worker) getBatchSize() int {
	return defaultBatchSize
}

// runCleanup executes the cleanup process based on storage policy.
// When manual is true and manualDays is provided, those days override the policy values.
func (w *Worker) runCleanup(ctx context.Context, manual bool, manualDays map[string]int) {
	log.Info(ctx, "Starting cleanup process", log.Bool("manual", manual))

	ctx = ent.NewContext(ctx, w.Ent)
	ctx = schematype.SkipSoftDelete(ctx)

	policy, err := w.SystemService.StoragePolicy(ctx)
	if err != nil {
		log.Error(ctx, "Failed to get storage policy for cleanup", log.Cause(err))
		return
	}

	log.Debug(ctx, "Storage policy for cleanup", log.Any("policy", policy))

	for _, option := range policy.CleanupOptions {
		if option.Enabled || manual {
			if manual && manualDays != nil {
				if _, ok := manualDays[option.ResourceType]; !ok {
					continue
				}
			}
			days := option.CleanupDays
			if manual && manualDays != nil {
				if d, ok := manualDays[option.ResourceType]; ok {
					days = d
				}
			}
			switch option.ResourceType {
			case "requests":
				err := w.cleanupRequests(ctx, days, manual)
				if err != nil {
					log.Error(ctx, "Failed to cleanup requests",
						log.String("resource", option.ResourceType),
						log.Cause(err))
				} else {
					log.Info(ctx, "Successfully cleaned up requests",
						log.String("resource", option.ResourceType),
						log.Int("cleanup_days", days))
				}

				err = w.cleanupThreads(ctx, days, manual)
				if err != nil {
					log.Error(ctx, "Failed to cleanup threads",
						log.String("resource", "threads"),
						log.Cause(err))
				} else {
					log.Info(ctx, "Successfully cleaned up threads",
						log.String("resource", "threads"),
						log.Int("cleanup_days", days))
				}

				err = w.cleanupTraces(ctx, days, manual)
				if err != nil {
					log.Error(ctx, "Failed to cleanup traces",
						log.String("resource", "traces"),
						log.Cause(err))
				} else {
					log.Info(ctx, "Successfully cleaned up traces",
						log.String("resource", "traces"),
						log.Int("cleanup_days", days))
				}
			case "usage_logs":
				err := w.cleanupUsageLogs(ctx, days, manual)
				if err != nil {
					log.Error(ctx, "Failed to cleanup usage logs",
						log.String("resource", option.ResourceType),
						log.Cause(err))
				} else {
					log.Info(ctx, "Successfully cleaned up usage logs",
						log.String("resource", option.ResourceType),
						log.Int("cleanup_days", days))
				}
			default:
				log.Warn(ctx, "Unknown resource type for cleanup",
					log.String("resource", option.ResourceType))
			}
		}
	}

	err = w.cleanupChannelProbes(ctx, 3, manual)
	if err != nil {
		log.Error(ctx, "Failed to cleanup channel probes",
			log.Cause(err))
	} else {
		log.Info(ctx, "Successfully cleaned up channel probes",
			log.Int("cleanup_days", 3))
	}

	if w.Config.VacuumEnabled {
		if err := w.runVacuum(ctx); err != nil {
			log.Error(ctx, "Failed to run VACUUM after cleanup",
				log.Cause(err))
		}
	}

	log.Info(ctx, "Cleanup process completed")
}

// cleanupRequests deletes requests older than the specified number of days.
func (w *Worker) cleanupRequests(ctx context.Context, cleanupDays int, manual bool) error {
	if cleanupDays <= 0 {
		log.Debug(ctx, "No cleanup needed for requests")
		return nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -cleanupDays)

	execResult, err := w.cleanupOldRequestExecutions(ctx, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to cleanup request executions: %w", err)
	}

	log.Debug(ctx, "Deleted old request executions",
		log.Int("deleted_executions_count", execResult),
		log.Time("cutoff_time", cutoffTime),
	)

	reqResult, err := w.cleanupOldRequestsRecords(ctx, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to cleanup requests: %w", err)
	}

	log.Debug(ctx, "Deleted old requests",
		log.Int("deleted_requests_count", reqResult),
		log.Time("cutoff_time", cutoffTime))

	return nil
}

func (w *Worker) cleanupOldRequestExecutions(ctx context.Context, cutoffTime time.Time) (int, error) {
	batchSize := w.getBatchSize()
	totalDeleted := 0
	cache := make(map[int]*ent.DataStorage)

	for {
		executions, err := w.Ent.RequestExecution.Query().
			Select(
				requestexecution.FieldID,
				requestexecution.FieldProjectID,
				requestexecution.FieldDataStorageID,
				requestexecution.FieldRequestID,
			).
			Where(requestexecution.CreatedAtLT(cutoffTime)).
			Order(ent.Asc(requestexecution.FieldID)).
			Limit(batchSize).
			All(ctx)
		if err != nil {
			return totalDeleted, fmt.Errorf("failed to query old request executions: %w", err)
		}

		if len(executions) == 0 {
			break
		}

		ids := make([]int, len(executions))

		for i, exec := range executions {
			ids[i] = exec.ID
			w.cleanupExecutionExternalStorage(ctx, exec, cache)
		}

		if _, err := w.Ent.RequestExecution.Delete().
			Where(requestexecution.IDIn(ids...)).
			Exec(ctx); err != nil {
			return totalDeleted, fmt.Errorf("failed to delete request executions batch: %w", err)
		}

		log.Debug(ctx, "Deleted old request executions batch",
			log.Int("deleted_executions_count", len(ids)),
			log.Time("cutoff_time", cutoffTime),
		)

		totalDeleted += len(ids)
	}

	return totalDeleted, nil
}

func (w *Worker) cleanupOldRequestsRecords(ctx context.Context, cutoffTime time.Time) (int, error) {
	batchSize := w.getBatchSize()
	totalDeleted := 0
	cache := make(map[int]*ent.DataStorage)

	for {
		reqs, err := w.Ent.Request.Query().
			Select(
				request.FieldID,
				request.FieldProjectID,
				request.FieldDataStorageID,
			).
			Where(request.CreatedAtLT(cutoffTime)).
			Order(ent.Asc(request.FieldID)).
			Limit(batchSize).
			All(ctx)
		if err != nil {
			return totalDeleted, fmt.Errorf("failed to query old requests: %w", err)
		}

		if len(reqs) == 0 {
			break
		}

		ids := make([]int, len(reqs))
		for i, req := range reqs {
			ids[i] = req.ID
			w.cleanupRequestExternalStorage(ctx, req, cache)
		}

		if _, err := w.Ent.Request.Delete().
			Where(request.IDIn(ids...)).
			Exec(ctx); err != nil {
			return totalDeleted, fmt.Errorf("failed to delete requests batch: %w", err)
		}

		totalDeleted += len(ids)
	}

	return totalDeleted, nil
}

func (w *Worker) cleanupExecutionExternalStorage(ctx context.Context, exec *ent.RequestExecution, cache map[int]*ent.DataStorage) {
	if exec == nil || exec.DataStorageID == 0 || w.DataStorageService == nil {
		return
	}

	ds, err := w.getDataStorageCached(ctx, exec.DataStorageID, cache)
	if err != nil {
		log.Warn(ctx, "Failed to load data storage for execution cleanup",
			log.Cause(err),
			log.Int("execution_id", exec.ID),
		)

		return
	}

	if ds == nil || ds.Primary {
		return
	}

	keys := []string{
		biz.GenerateExecutionRequestBodyKey(exec.ProjectID, exec.RequestID, exec.ID),
		biz.GenerateExecutionResponseBodyKey(exec.ProjectID, exec.RequestID, exec.ID),
		biz.GenerateExecutionResponseChunksKey(exec.ProjectID, exec.RequestID, exec.ID),
	}

	// Directory-marker keys only exist as real directories on filesystem-like
	// backends. On object stores (S3/GCS) they were never created, so deleting
	// them only wastes a ListObjectsV2 (Class A); skip them there.
	if hasRealDirectories(ds.Type) {
		keys = append(keys, biz.GenerateExecutionRequestDirKey(exec.ProjectID, exec.RequestID, exec.ID))
	}

	for _, key := range keys {
		if err := w.DataStorageService.DeleteData(ctx, ds, key); err != nil {
			log.Warn(ctx, "Failed to delete execution external data",
				log.Cause(err),
				log.Int("execution_id", exec.ID),
				log.String("key", key),
			)
		}
	}
}

func (w *Worker) cleanupRequestExternalStorage(ctx context.Context, req *ent.Request, cache map[int]*ent.DataStorage) {
	if req == nil || req.DataStorageID == 0 || w.DataStorageService == nil {
		return
	}

	ds, err := w.getDataStorageCached(ctx, req.DataStorageID, cache)
	if err != nil {
		log.Warn(ctx, "Failed to load data storage for request cleanup",
			log.Cause(err),
			log.Int("request_id", req.ID),
		)

		return
	}

	if ds == nil || ds.Primary {
		return
	}

	keys := []string{
		biz.GenerateRequestBodyKey(req.ProjectID, req.ID),
		biz.GenerateResponseBodyKey(req.ProjectID, req.ID),
		biz.GenerateResponseChunksKey(req.ProjectID, req.ID),
	}

	// See cleanupExecutionExternalStorage: object stores have no real
	// directories, so only attempt directory-marker deletes on FS/WebDAV.
	if hasRealDirectories(ds.Type) {
		keys = append(keys,
			biz.GenerateRequestExecutionsDirKey(req.ProjectID, req.ID),
			biz.GenerateRequestDirKey(req.ProjectID, req.ID),
		)
	}

	for _, key := range keys {
		if err := w.DataStorageService.DeleteData(ctx, ds, key); err != nil {
			log.Warn(ctx, "Failed to delete request external data",
				log.Cause(err),
				log.Int("request_id", req.ID),
				log.String("key", key),
			)
		}
	}
}

// hasRealDirectories reports whether the storage backend materializes
// directories as real entries that must be explicitly removed during cleanup.
// Object stores (S3/GCS) have no real directories — the "*DirKey" paths are
// never created, so attempting to delete them only costs a wasted
// ListObjectsV2 (Class A). Filesystem and WebDAV backends do create real
// directories that should be removed.
func hasRealDirectories(t datastorage.Type) bool {
	return t == datastorage.TypeFs || t == datastorage.TypeWebdav
}

func (w *Worker) getDataStorageCached(ctx context.Context, id int, cache map[int]*ent.DataStorage) (*ent.DataStorage, error) {
	if ds, ok := cache[id]; ok {
		return ds, nil
	}

	ds, err := w.DataStorageService.GetDataStorageByID(ctx, id)
	if err != nil {
		return nil, err
	}

	cache[id] = ds

	return ds, nil
}

// cleanupUsageLogs deletes usage logs older than the specified number of days.
func (w *Worker) cleanupUsageLogs(ctx context.Context, cleanupDays int, manual bool) error {
	if cleanupDays <= 0 {
		return nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -cleanupDays)
	batchSize := w.getBatchSize()

	result, err := w.deleteInBatches(ctx, func() (int, error) {
		ids, err := w.Ent.UsageLog.Query().
			Where(usagelog.CreatedAtLT(cutoffTime)).
			Order(ent.Asc(usagelog.FieldID)).
			Limit(batchSize).
			IDs(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to query old usage logs: %w", err)
		}
		if len(ids) == 0 {
			return 0, nil
		}

		return w.Ent.UsageLog.Delete().Where(usagelog.IDIn(ids...)).Exec(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to delete old usage logs: %w", err)
	}

	log.Debug(ctx, "Cleaned up usage logs",
		log.Int("deleted_count", result),
		log.Time("cutoff_time", cutoffTime))

	return nil
}

// cleanupThreads deletes threads older than the specified number of days.
func (w *Worker) cleanupThreads(ctx context.Context, cleanupDays int, manual bool) error {
	if cleanupDays <= 0 {
		log.Debug(ctx, "No cleanup needed for threads")
		return nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -cleanupDays)
	batchSize := w.getBatchSize()

	result, err := w.deleteInBatches(ctx, func() (int, error) {
		ids, err := w.Ent.Thread.Query().
			Where(thread.CreatedAtLT(cutoffTime)).
			Order(ent.Asc(thread.FieldID)).
			Limit(batchSize).
			IDs(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to query old threads: %w", err)
		}
		if len(ids) == 0 {
			return 0, nil
		}

		return w.Ent.Thread.Delete().Where(thread.IDIn(ids...)).Exec(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to delete old threads: %w", err)
	}

	log.Debug(ctx, "Cleaned up threads",
		log.Int("deleted_count", result),
		log.Time("cutoff_time", cutoffTime))

	return nil
}

// cleanupTraces deletes traces older than the specified number of days.
func (w *Worker) cleanupTraces(ctx context.Context, cleanupDays int, manual bool) error {
	if cleanupDays <= 0 {
		log.Debug(ctx, "No cleanup needed for traces")
		return nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -cleanupDays)
	batchSize := w.getBatchSize()

	result, err := w.deleteInBatches(ctx, func() (int, error) {
		ids, err := w.Ent.Trace.Query().
			Where(trace.CreatedAtLT(cutoffTime)).
			Order(ent.Asc(trace.FieldID)).
			Limit(batchSize).
			IDs(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to query old traces: %w", err)
		}
		if len(ids) == 0 {
			return 0, nil
		}

		return w.Ent.Trace.Delete().Where(trace.IDIn(ids...)).Exec(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to delete old traces: %w", err)
	}

	log.Debug(ctx, "Cleaned up traces",
		log.Int("deleted_count", result),
		log.Time("cutoff_time", cutoffTime))

	return nil
}

// cleanupChannelProbes deletes channel probes older than the specified number of days.
func (w *Worker) cleanupChannelProbes(ctx context.Context, cleanupDays int, manual bool) error {
	if cleanupDays <= 0 {
		log.Debug(ctx, "No cleanup needed for channel probes")
		return nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -cleanupDays)
	batchSize := w.getBatchSize()

	result, err := w.deleteInBatches(ctx, func() (int, error) {
		ids, err := w.Ent.ChannelProbe.Query().
			Where(channelprobe.TimestampLT(cutoffTime.Unix())).
			Order(ent.Asc(channelprobe.FieldID)).
			Limit(batchSize).
			IDs(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to query old channel probes: %w", err)
		}
		if len(ids) == 0 {
			return 0, nil
		}

		return w.Ent.ChannelProbe.Delete().Where(channelprobe.IDIn(ids...)).Exec(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to delete old channel probes: %w", err)
	}

	log.Debug(ctx, "Cleaned up channel probes",
		log.Int("deleted_count", result),
		log.Time("cutoff_time", cutoffTime))

	return nil
}

// runVacuum executes VACUUM command on SQLite/PostgreSQL database.
func (w *Worker) runVacuum(ctx context.Context) error {
	if !w.Config.VacuumEnabled {
		log.Debug(ctx, "VACUUM is disabled, skipping")
		return nil
	}

	dbDriver := w.Ent.Driver()
	if dbDriver == nil {
		return fmt.Errorf("failed to get database driver")
	}

	sqlDriver, ok := dbDriver.(*entsql.Driver)
	if !ok {
		log.Debug(ctx, "Database driver is not *entsql.Driver, skipping VACUUM")
		return nil
	}

	if sqlDriver.Dialect() != dialect.SQLite && sqlDriver.Dialect() != dialect.Postgres {
		log.Debug(ctx, "Database does not support VACUUM, skipping",
			log.String("dialect", sqlDriver.Dialect()))

		return nil
	}

	log.Info(ctx, "Starting database VACUUM operation",
		log.String("dialect", sqlDriver.Dialect()),
		log.Bool("vacuum_full", w.Config.VacuumFull))

	startTime := time.Now()

	var vacuumSQL string
	if sqlDriver.Dialect() == dialect.Postgres && w.Config.VacuumFull {
		vacuumSQL = "VACUUM FULL"
	} else {
		vacuumSQL = "VACUUM"
	}

	if _, err := sqlDriver.ExecContext(ctx, vacuumSQL); err != nil {
		return fmt.Errorf("failed to execute %s: %w", vacuumSQL, err)
	}

	duration := time.Since(startTime)
	log.Info(ctx, "Database VACUUM completed successfully",
		log.Duration("duration", duration),
		log.String("command", vacuumSQL))

	return nil
}

// RunVacuumNow manually triggers the VACUUM operation.
func (w *Worker) RunVacuumNow(ctx context.Context) error {
	return w.runVacuum(ctx)
}

// RunCleanupNow manually triggers the cleanup process with the specified days.
func (w *Worker) RunCleanupNow(ctx context.Context, input TriggerGcCleanupInput) error {
	manualDays := make(map[string]int)
	if input.RequestsCleanupDays > 0 {
		manualDays["requests"] = input.RequestsCleanupDays
	}
	if input.UsageLogsCleanupDays > 0 {
		manualDays["usage_logs"] = input.UsageLogsCleanupDays
	}
	w.runCleanup(ctx, true, manualDays)
	return nil
}

// PreviewCleanup estimates how many records would be deleted without actually deleting them.
func (w *Worker) PreviewCleanup(ctx context.Context, input TriggerGcCleanupInput) ([]GcCleanupPreviewItem, error) {
	ctx = ent.NewContext(ctx, w.Ent)
	ctx = schematype.SkipSoftDelete(ctx)

	var items []GcCleanupPreviewItem

	if input.RequestsCleanupDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -input.RequestsCleanupDays)
		count, err := w.Ent.Request.Query().Where(request.CreatedAtLT(cutoff)).Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to count requests for preview: %w", err)
		}
		items = append(items, GcCleanupPreviewItem{
			ResourceType:   "requests",
			EstimatedCount: count,
			CutoffTime:     cutoff,
			RetentionDays:  input.RequestsCleanupDays,
		})
	}

	if input.UsageLogsCleanupDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -input.UsageLogsCleanupDays)
		count, err := w.Ent.UsageLog.Query().Where(usagelog.CreatedAtLT(cutoff)).Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to count usage logs for preview: %w", err)
		}
		items = append(items, GcCleanupPreviewItem{
			ResourceType:   "usage_logs",
			EstimatedCount: count,
			CutoffTime:     cutoff,
			RetentionDays:  input.UsageLogsCleanupDays,
		})
	}

	return items, nil
}
