package backup

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
)

func (svc *BackupService) runBackupPeriodically(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "run-auto-backup")
	svc.triggerAutoBackup(ctx)
}
