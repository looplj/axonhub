package backup

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/afero"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

func (svc *BackupService) Start(ctx context.Context) error {
	return svc.runBackupPeriodic(ctx)
}

func (svc *BackupService) Stop(ctx context.Context) error {
	if svc.cancelFunc != nil {
		svc.cancelFunc()
	}

	if svc.executor == nil {
		return nil
	}

	return svc.executor.Shutdown(ctx)
}

func (svc *BackupService) runBackupPeriodic(ctx context.Context) error {
	if svc.cancelFunc != nil {
		return nil
	}

	cronExpr := "0 2 * * *" // Always run daily at 2 AM

	cancelFunc, err := svc.executor.ScheduleFuncAtCronRate(
		svc.runBackup,
		executors.CRONRule{Expr: cronExpr},
	)
	if err != nil {
		return fmt.Errorf("failed to schedule backup: %w", err)
	}

	svc.cancelFunc = cancelFunc

	log.Info(ctx, "Auto backup scheduled", log.String("cron", cronExpr))

	return nil
}

func (svc *BackupService) runBackup(ctx context.Context) {
	log.Info(ctx, "Checking if backup is needed")

	ctx = ent.NewContext(ctx, svc.db)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	settings, err := svc.systemService.AutoBackupSettings(ctx)
	if err != nil {
		log.Error(ctx, "Failed to get auto backup settings", log.Cause(err))
		return
	}

	if !settings.Enabled {
		log.Info(ctx, "Auto backup is disabled, skipping")
		return
	}

	if !svc.shouldRunBackup(time.Now(), settings) {
		log.Info(ctx, "Backup not needed based on frequency",
			log.String("frequency", string(settings.Frequency)))

		return
	}

	log.Info(ctx, "Starting automatic backup")

	startAt := time.Now()
	err = svc.performBackup(ctx, settings)

	var errMsg string
	if err != nil {
		errMsg = err.Error()
		log.Error(ctx, "Auto backup failed", log.Cause(err))
	} else {
		log.Info(ctx, "Auto backup completed successfully",
			log.String("cost", time.Since(startAt).String()))
	}

	if err := svc.systemService.UpdateAutoBackupLastRun(ctx, errMsg); err != nil {
		log.Error(ctx, "Failed to update auto backup status", log.Cause(err))
	}
}

func (svc *BackupService) shouldRunBackup(now time.Time, settings *biz.AutoBackupSettings) bool {
	switch settings.Frequency {
	case biz.BackupFrequencyDaily:
		return true
	case biz.BackupFrequencyWeekly:
		return now.Weekday() == time.Sunday
	case biz.BackupFrequencyMonthly:
		return now.Day() == 1
	default:
		// Unknown frequency, default to daily to be safe.
		return true
	}
}

func (svc *BackupService) performBackup(ctx context.Context, settings *biz.AutoBackupSettings) error {
	ds, err := svc.dataStorageService.GetDataStorageByID(ctx, settings.DataStorageID)
	if err != nil {
		return fmt.Errorf("failed to get data storage: %w", err)
	}

	opts := BackupOptions{
		IncludeChannels:    settings.IncludeChannels,
		IncludeModels:      settings.IncludeModels,
		IncludeAPIKeys:     settings.IncludeAPIKeys,
		IncludeModelPrices: settings.IncludeModelPrices,
	}

	data, err := svc.BackupWithoutAuth(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("axonhub-backup-%s.json", timestamp)

	if _, err := svc.dataStorageService.SaveData(ctx, ds, filename, data); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	log.Info(ctx, "Backup uploaded to storage",
		log.String("path", filename),
		log.Int("size", len(data)),
	)

	if settings.RetentionDays > 0 {
		if err := svc.cleanupOldBackups(ctx, ds, settings.RetentionDays); err != nil {
			log.Warn(ctx, "Failed to cleanup old backups", log.Cause(err))
		}
	}

	return nil
}

func (svc *BackupService) cleanupOldBackups(ctx context.Context, ds *ent.DataStorage, retentionDays int) error {
	fs, err := svc.dataStorageService.GetFileSystem(ctx, ds)
	if err != nil {
		return fmt.Errorf("failed to get data storage filesystem: %w", err)
	}

	files, err := afero.ReadDir(fs, "/")
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	var backupFiles []os.FileInfo

	for _, f := range files {
		if strings.HasPrefix(f.Name(), "axonhub-backup-") && strings.HasSuffix(f.Name(), ".json") {
			backupFiles = append(backupFiles, f)
		}
	}

	sort.Slice(backupFiles, func(i, j int) bool {
		return backupFiles[i].ModTime().Before(backupFiles[j].ModTime())
	})

	for _, f := range backupFiles {
		if f.ModTime().Before(cutoff) {
			if err := fs.Remove(f.Name()); err != nil {
				log.Warn(ctx, "Failed to delete old backup",
					log.String("file", f.Name()),
					log.Cause(err),
				)
			} else {
				log.Info(ctx, "Deleted old backup",
					log.String("file", f.Name()),
				)
			}
		}
	}

	return nil
}

// RunBackupNow triggers an immediate backup.
func (svc *BackupService) RunBackupNow(ctx context.Context) error {
	settings, err := svc.systemService.AutoBackupSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auto backup settings: %w", err)
	}

	if settings.DataStorageID == 0 {
		return fmt.Errorf("data storage not configured for backup")
	}

	return svc.performBackup(ctx, settings)
}
