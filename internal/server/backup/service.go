package backup

import (
	"context"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

// Config holds backup service configuration.
type Config struct {
	// CronExpr is the cron expression for automatic backup scheduling.
	// Defaults to "0 2 * * *" (daily at 2 AM) if empty.
	CronExpr string `json:"cron_expr" yaml:"cron_expr" conf:"cron_expr"`
}

type BackupServiceParams struct {
	fx.In

	Config             Config `optional:"true"`
	Ent                *ent.Client
	SystemService      *biz.SystemService
	DataStorageService *biz.DataStorageService
}

func NewBackupService(params BackupServiceParams) *BackupService {
	cronExpr := params.Config.CronExpr
	if cronExpr == "" {
		cronExpr = "0 2 * * *"
	}

	return &BackupService{
		db:                 params.Ent,
		systemService:      params.SystemService,
		dataStorageService: params.DataStorageService,
		executor:           executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1)),
		cronExpr:           cronExpr,
	}
}

type BackupService struct {
	db *ent.Client

	systemService      *biz.SystemService
	dataStorageService *biz.DataStorageService

	executor   executors.ScheduledExecutor
	cancelFunc context.CancelFunc
	cronExpr   string
}
