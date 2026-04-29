package backup

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/log"
)

func (svc *BackupService) runBackupPeriodically(ctx context.Context) {
	ctx, err := authz.WithSystemBypass(ctx, "run-auto-backup")
	if err != nil {
		log.Error(context.Background(), "failed to create bypass context", log.Cause(err))
		return
	}
	svc.triggerAutoBackup(ctx)
}
