package backup

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/studio-b12/gowebdav"
	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

type Worker struct {
	SystemService *biz.SystemService
	BackupService *BackupService
	Ent           *ent.Client
	Executor      executors.ScheduledExecutor
	CancelFunc    context.CancelFunc
	currentCron   string
}

type Params struct {
	fx.In

	SystemService *biz.SystemService
	BackupService *BackupService
	Client        *ent.Client
}

func NewWorker(params Params) *Worker {
	return &Worker{
		SystemService: params.SystemService,
		BackupService: params.BackupService,
		Ent:           params.Client,
		Executor:      executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1)),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	settings, err := w.SystemService.AutoBackupSettings(ctx)
	if err != nil {
		log.Warn(ctx, "Failed to get auto backup settings on start", log.Cause(err))
		return nil
	}

	if !settings.Enabled {
		log.Info(ctx, "Auto backup is disabled")
		return nil
	}

	return w.scheduleBackup(ctx, settings)
}

func (w *Worker) Stop(ctx context.Context) error {
	if w.CancelFunc != nil {
		w.CancelFunc()
	}

	return w.Executor.Shutdown(ctx)
}

func (w *Worker) Reschedule(ctx context.Context) error {
	if w.CancelFunc != nil {
		w.CancelFunc()
		w.CancelFunc = nil
	}

	settings, err := w.SystemService.AutoBackupSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auto backup settings: %w", err)
	}

	if !settings.Enabled {
		log.Info(ctx, "Auto backup disabled, unscheduling")
		return nil
	}

	return w.scheduleBackup(ctx, settings)
}

func (w *Worker) scheduleBackup(ctx context.Context, settings *biz.AutoBackupSettings) error {
	cronExpr := "0 2 * * *" // Always run daily at 2 AM

	cancelFunc, err := w.Executor.ScheduleFuncAtCronRate(
		w.runBackup,
		executors.CRONRule{Expr: cronExpr},
	)
	if err != nil {
		return fmt.Errorf("failed to schedule backup: %w", err)
	}

	w.CancelFunc = cancelFunc
	w.currentCron = cronExpr

	log.Info(ctx, "Auto backup scheduled",
		log.String("cron", cronExpr),
		log.String("frequency", string(settings.Frequency)),
	)

	return nil
}

func (w *Worker) runBackup(ctx context.Context) {
	log.Info(ctx, "Checking if backup is needed")

	ctx = ent.NewContext(ctx, w.Ent)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	settings, err := w.SystemService.AutoBackupSettings(ctx)
	if err != nil {
		log.Error(ctx, "Failed to get auto backup settings", log.Cause(err))
		return
	}

	if !settings.Enabled {
		log.Info(ctx, "Auto backup is disabled, skipping")
		return
	}

	if !w.shouldRunBackup(settings) {
		log.Info(ctx, "Backup not needed based on frequency",
			log.String("frequency", string(settings.Frequency)),
		)

		return
	}

	log.Info(ctx, "Starting automatic backup")

	backupErr := w.performBackup(ctx, settings)

	now := time.Now()

	errMsg := ""
	if backupErr != nil {
		errMsg = backupErr.Error()
		log.Error(ctx, "Auto backup failed", log.Cause(backupErr))
	} else {
		log.Info(ctx, "Auto backup completed successfully")
	}

	if updateErr := w.SystemService.UpdateAutoBackupLastRun(ctx, now, errMsg); updateErr != nil {
		log.Error(ctx, "Failed to update auto backup status", log.Cause(updateErr))
	}
}

func (w *Worker) shouldRunBackup(settings *biz.AutoBackupSettings) bool {
	if settings.LastBackupAt == nil {
		return true
	}

	daysSinceLastBackup := time.Since(*settings.LastBackupAt).Hours() / 24

	switch settings.Frequency {
	case biz.BackupFrequencyDaily:
		return true
	case biz.BackupFrequencyWeekly:
		return daysSinceLastBackup >= 7
	case biz.BackupFrequencyMonthly:
		return daysSinceLastBackup >= 30
	default:
		return true
	}
}

func (w *Worker) performBackup(ctx context.Context, settings *biz.AutoBackupSettings) error {
	if settings.WebDAV == nil {
		return fmt.Errorf("WebDAV configuration is missing")
	}

	opts := BackupOptions{
		IncludeChannels:    settings.IncludeChannels,
		IncludeModels:      settings.IncludeModels,
		IncludeAPIKeys:     settings.IncludeAPIKeys,
		IncludeModelPrices: settings.IncludeModelPrices,
	}

	data, err := w.BackupService.BackupWithoutAuth(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	client := gowebdav.NewClient(settings.WebDAV.URL, settings.WebDAV.Username, settings.WebDAV.Password)
	if settings.WebDAV.InsecureSkipTLS {
		//nolint:gosec // InsecureSkipVerify is used for testing purposes only.
		client.SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("axonhub-backup-%s.json", timestamp)

	remotePath := settings.WebDAV.Path
	if remotePath == "" {
		remotePath = "/"
	}

	if !strings.HasSuffix(remotePath, "/") {
		remotePath += "/"
	}

	fullPath := remotePath + filename

	if err := client.Write(fullPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to upload backup to WebDAV: %w", err)
	}

	log.Info(ctx, "Backup uploaded to WebDAV",
		log.String("path", fullPath),
		log.Int("size", len(data)),
	)

	if settings.RetentionDays > 0 {
		if err := w.cleanupOldBackups(ctx, client, remotePath, settings.RetentionDays); err != nil {
			log.Warn(ctx, "Failed to cleanup old backups", log.Cause(err))
		}
	}

	return nil
}

func (w *Worker) cleanupOldBackups(ctx context.Context, client *gowebdav.Client, remotePath string, retentionDays int) error {
	files, err := client.ReadDir(remotePath)
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
			filePath := remotePath
			if !strings.HasSuffix(filePath, "/") {
				filePath += "/"
			}

			filePath += f.Name()

			if err := client.Remove(filePath); err != nil {
				log.Warn(ctx, "Failed to delete old backup",
					log.String("file", filePath),
					log.Cause(err),
				)
			} else {
				log.Info(ctx, "Deleted old backup",
					log.String("file", filePath),
				)
			}
		}
	}

	return nil
}

// RunBackupNow triggers an immediate backup.
func (w *Worker) RunBackupNow(ctx context.Context) error {
	settings, err := w.SystemService.AutoBackupSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auto backup settings: %w", err)
	}

	if settings.WebDAV == nil {
		return fmt.Errorf("WebDAV configuration is not set")
	}

	return w.performBackup(ctx, settings)
}

// TestConnection tests the WebDAV connection with the provided configuration.
func (w *Worker) TestConnection(ctx context.Context, config *biz.WebDAVConfig) error {
	if config == nil {
		return fmt.Errorf("WebDAV configuration is missing")
	}

	client := gowebdav.NewClient(config.URL, config.Username, config.Password)
	if config.InsecureSkipTLS {
		//nolint:gosec // InsecureSkipVerify is used for testing purposes only.
		client.SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})
	}

	return client.Connect()
}
