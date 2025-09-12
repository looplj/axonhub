package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

type Config struct {
	CRON string `json:"cron" yaml:"cron" conf:"cron" validate:"required"`
}

// Worker handles garbage collection and cleanup operations.
type Worker struct {
	SystemService *biz.SystemService
	Executor      executors.ScheduledExecutor
	Ent           *ent.Client
	Config        Config
	CancelFunc    context.CancelFunc
}

type Params struct {
	fx.In

	Config        Config
	SystemService *biz.SystemService
	Client        *ent.Client
}

// NewWorker creates a new GCService with daily cleanup scheduling.
func NewWorker(params Params) *Worker {
	return &Worker{
		SystemService: params.SystemService,
		Executor:      executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1)),
		Ent:           params.Client,
		Config:        params.Config,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	cancelFunc, err := w.Executor.ScheduleFuncAtCronRate(
		w.runCleanup,
		executors.CRONRule{Expr: w.Config.CRON},
	)
	if err != nil {
		return err
	}

	w.CancelFunc = cancelFunc

	log.Info(ctx, "GC worker started", log.String("cron", w.Config.CRON),
		log.Bool("cancel_func", w.CancelFunc != nil),
		log.Bool("ent", w.Ent != nil),
		log.Bool("executor", w.Executor != nil),
		log.Bool("system_service", w.SystemService != nil),
	)

	return nil
}

func (w *Worker) Stop(ctx context.Context) error {
	if w.CancelFunc != nil {
		w.CancelFunc()
	}

	return w.Executor.Shutdown(ctx)
}

// runCleanup executes the cleanup process based on storage policy.
func (w *Worker) runCleanup(ctx context.Context) {
	log.Info(ctx, "Starting automatic cleanup process")

	ctx = ent.NewContext(ctx, w.Ent)
	ctx = schematype.SkipSoftDelete(ctx)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Get storage policy
	policy, err := w.SystemService.StoragePolicy(ctx)
	if err != nil {
		log.Error(ctx, "Failed to get storage policy for cleanup", log.Cause(err))
		return
	}

	log.Debug(ctx, "Storage policy for cleanup", log.Any("policy", policy))

	// Execute cleanup for each resource type
	for _, option := range policy.CleanupOptions {
		if option.Enabled {
			switch option.ResourceType {
			case "requests":
				err := w.cleanupRequests(ctx, option.CleanupDays)
				if err != nil {
					log.Error(ctx, "Failed to cleanup requests",
						log.String("resource", option.ResourceType),
						log.Cause(err))
				} else {
					log.Info(ctx, "Successfully cleaned up requests",
						log.String("resource", option.ResourceType),
						log.Int("cleanup_days", option.CleanupDays))
				}
			case "usage_logs":
				err := w.cleanupUsageLogs(ctx, option.CleanupDays)
				if err != nil {
					log.Error(ctx, "Failed to cleanup usage logs",
						log.String("resource", option.ResourceType),
						log.Cause(err))
				} else {
					log.Info(ctx, "Successfully cleaned up usage logs",
						log.String("resource", option.ResourceType),
						log.Int("cleanup_days", option.CleanupDays))
				}
			default:
				log.Warn(ctx, "Unknown resource type for cleanup",
					log.String("resource", option.ResourceType))
			}
		}
	}

	log.Info(ctx, "Automatic cleanup process completed")
}

// cleanupRequests deletes requests older than the specified number of days.
func (w *Worker) cleanupRequests(ctx context.Context, cleanupDays int) error {
	if cleanupDays <= 0 {
		log.Debug(ctx, "No cleanup needed for requests")
		return nil // No cleanup needed
	}

	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	cutoffTime := time.Now().AddDate(0, 0, -cleanupDays)

	// Delete requests older than the cutoff time
	reqResult, err := w.Ent.Request.Delete().
		Where(request.CreatedAtLT(cutoffTime)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete old requests: %w", err)
	}

	log.Debug(ctx, "Deleted old requests",
		log.Int("deleted_requests_count", reqResult),
		log.Time("cutoff_time", cutoffTime))

	execResult, err := w.Ent.RequestExecution.Delete().
		Where(requestexecution.CreatedAtLT(cutoffTime)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete old request executions: %w", err)
	}

	log.Debug(ctx, "Deleted old request executions",
		log.Int("deleted_executions_count", execResult),
		log.Time("cutoff_time", cutoffTime),
	)

	return nil
}

// cleanupUsageLogs deletes usage logs older than the specified number of days.
func (w *Worker) cleanupUsageLogs(ctx context.Context, cleanupDays int) error {
	if cleanupDays <= 0 {
		return nil // No cleanup needed
	}

	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	cutoffTime := time.Now().AddDate(0, 0, -cleanupDays)

	// Delete usage logs older than the cutoff time
	result, err := w.Ent.UsageLog.Delete().
		Where(usagelog.CreatedAtLT(cutoffTime)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete old usage logs: %w", err)
	}

	log.Debug(ctx, "Cleaned up usage logs",
		log.Int("deleted_count", result),
		log.Time("cutoff_time", cutoffTime))

	return nil
}

// RunCleanupNow manually triggers the cleanup process.
// This can be useful for testing or manual execution.
func (w *Worker) RunCleanupNow(ctx context.Context) error {
	w.runCleanup(ctx)
	return nil
}
