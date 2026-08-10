package backup

import (
	"context"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
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
	// 启动时 fx OnStart 传入的 context 不含用户，读取系统配置会被 ent privacy
	// 规则拒绝（"no user in context"），导致 TimeLocation 兜底返回 UTC、backup
	// 任务时区从配置值退化为 UTC。与 video_storage 注册一致，使用系统级 bypass。
	// The context passed by the fx OnStart hook has no user, so reading system
	// config would be denied by the ent privacy policy ("no user in context"),
	// making TimeLocation fall back to UTC and the backup task lose its
	// configured timezone. Use a system-level bypass, matching video_storage.
	ctx = authz.WithSystemBypass(ctx, "backup-register")

	tz := svc.systemService.TimeLocation(ctx).String()
	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "backup",
		Description: "Auto backup to configured data storage",
		CronExpr:    "0 2 * * *",
		Timezone:    tz,
	}, svc.runBackupPeriodically)
}
