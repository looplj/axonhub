package backup

import (
	"context"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

type BackupServiceParams struct {
	fx.In

	Ent                *ent.Client
	SystemService      *biz.SystemService
	DataStorageService *biz.DataStorageService
}

func NewBackupService(params BackupServiceParams) *BackupService {
	return &BackupService{
		db:                 params.Ent,
		systemService:      params.SystemService,
		dataStorageService: params.DataStorageService,
		executor:           executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1)),
	}
}

type BackupService struct {
	db *ent.Client

	systemService      *biz.SystemService
	dataStorageService *biz.DataStorageService

	executor   executors.ScheduledExecutor
	cancelFunc context.CancelFunc
}
