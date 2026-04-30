package backup

import (
	"context"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

type BackupServiceParams struct {
	fx.In

	Ent                *ent.Client
	SystemService      *biz.SystemService
	DataStorageService *biz.DataStorageService
}

func NewBackupService(params BackupServiceParams) *BackupService {
	svc := &BackupService{
		db:                 params.Ent,
		systemService:      params.SystemService,
		dataStorageService: params.DataStorageService,
	}

	return svc
}

type BackupService struct {
	db *ent.Client

	systemService      *biz.SystemService
	dataStorageService *biz.DataStorageService
}

func (svc *BackupService) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	tz := svc.systemService.TimeLocation(ctx).String()
	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "backup",
		Description: "Auto backup to configured data storage",
		CronExpr:    "0 2 * * *",
		Timezone:    tz,
	}, svc.runBackupPeriodically)
}
