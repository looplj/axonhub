package video_storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xtime"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

type Params struct {
	fx.In

	Ent                *ent.Client
	SystemService      *biz.SystemService
	DataStorageService *biz.DataStorageService
	VideoService       *biz.VideoService
	Executor           executors.ScheduledExecutor
}

type Worker struct {
	ent                *ent.Client
	systemService      *biz.SystemService
	dataStorageService *biz.DataStorageService
	videoService       *biz.VideoService
	executor           executors.ScheduledExecutor

	cancelFunc context.CancelFunc
}

func NewWorker(params Params) *Worker {
	return &Worker{
		ent:                params.Ent,
		systemService:      params.SystemService,
		dataStorageService: params.DataStorageService,
		videoService:       params.VideoService,
		executor:           params.Executor,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	if w.cancelFunc != nil {
		return nil
	}

	ctx = authz.WithSystemBypass(ctx, "video-storage-start")
	settings, err := w.systemService.VideoStorageSettings(ctx)
	if err != nil {
		return err
	}

	intervalMinutes := settings.ScanIntervalMinutes
	if intervalMinutes <= 0 {
		intervalMinutes = 1
	}

	cancelFunc, err := w.executor.ScheduleFuncAtFixRate(
		w.runScanWithSystemContext,
		time.Duration(intervalMinutes)*time.Minute,
	)
	if err != nil {
		return fmt.Errorf("failed to schedule video storage worker: %w", err)
	}

	w.cancelFunc = cancelFunc

	log.Info(ctx, "Video storage worker scheduled", log.Int("interval_minutes", intervalMinutes))

	return nil
}

func (w *Worker) Stop(ctx context.Context) error {
	if w.cancelFunc != nil {
		w.cancelFunc()
		w.cancelFunc = nil
	}

	return nil
}

func (w *Worker) runScanWithSystemContext(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "video-storage-scan")
	ctx = ent.NewContext(ctx, w.ent)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := w.scanAndSave(ctx); err != nil {
		log.Error(ctx, "Video storage worker failed", log.Cause(err))
	}
}

func (w *Worker) scanAndSave(ctx context.Context) error {
	settings, err := w.systemService.VideoStorageSettings(ctx)
	if err != nil {
		return err
	}

	if !settings.Enabled {
		return nil
	}

	if settings.DataStorageID == 0 {
		return fmt.Errorf("video storage enabled but data_storage_id is not set")
	}

	ds, err := w.dataStorageService.GetDataStorageByID(ctx, settings.DataStorageID)
	if err != nil {
		return fmt.Errorf("failed to get data storage: %w", err)
	}

	if ds.Primary || ds.Type == datastorage.TypeDatabase {
		return fmt.Errorf("video storage must be non-database storage")
	}

	limit := settings.ScanLimit
	if limit <= 0 {
		limit = 50
	}

	// Find video requests (completed or in-progress) not saved yet.
	reqs, err := w.ent.Request.Query().
		Where(
			request.StatusIn(request.StatusProcessing, request.StatusCompleted),
			request.FormatIn(string(llm.APIFormatOpenAIVideo), string(llm.APIFormatSeedanceVideo)),
			request.ContentSaved(false),
		).
		Order(ent.Asc(request.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query video requests: %w", err)
	}

	for _, req := range reqs {
		if err := w.processOne(ctx, ds, req); err != nil {
			log.Warn(ctx, "Failed to save video request", log.Cause(err), log.Int("request_id", req.ID))
			continue
		}
	}

	return nil
}

func (w *Worker) processOne(ctx context.Context, ds *ent.DataStorage, req *ent.Request) error {
	var videoURL string

	// First try to parse cached snapshot (unified VideoResponse format stored by outbound transformer).
	if v, err := extractVideoURLFromResponseBody(req.ResponseBody); err == nil && strings.TrimSpace(v) != "" {
		videoURL = v
	}

	// If missing, poll provider once to refresh snapshot via outbound transformer.
	if strings.TrimSpace(videoURL) == "" {
		resp, err := w.videoService.GetTask(ctx, req.ID)
		if err != nil {
			return err
		}

		// Only save when provider confirms it succeeded.
		if resp.Video == nil || strings.ToLower(strings.TrimSpace(resp.Video.Status)) != "succeeded" {
			return nil
		}

		videoURL = resp.Video.VideoURL
	}

	if strings.TrimSpace(videoURL) == "" {
		return nil
	}

	downloadCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	resp, filename, err := openVideoStream(downloadCtx, videoURL)
	if err != nil {
		return err
	}
	defer resp.Close()

	// Limit to 512MB to avoid OOM for pathological URLs.
	const maxBytes = 512 * 1024 * 1024
	reader := io.LimitReader(resp, maxBytes)

	storageKey := GenerateVideoKey(req.ProjectID, req.ID, filename)

	_, n, err := w.dataStorageService.SaveDataFromReader(ctx, ds, storageKey, reader)
	if err != nil {
		return fmt.Errorf("failed to save video to storage: %w", err)
	}

	now := xtime.UTCNow()
	_, err = w.ent.Request.UpdateOneID(req.ID).
		SetContentSaved(true).
		SetContentStorageID(ds.ID).
		SetContentStorageKey(storageKey).
		SetContentSavedAt(now).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request video saved status: %w", err)
	}

	log.Info(ctx, "Saved video to storage",
		log.Int("request_id", req.ID),
		log.Int("data_storage_id", ds.ID),
		log.String("key", storageKey),
		log.Int64("size", n),
	)

	return nil
}

func GenerateVideoKey(projectID, requestID int, filename string) string {
	name := strings.TrimSpace(filename)
	if name == "" {
		name = "video.mp4"
	}
	name = filepath.Base(name)
	return fmt.Sprintf("/%d/requests/%d/video/%s", projectID, requestID, name)
}

// extractVideoURLFromResponseBody parses the unified VideoResponse format
// stored by the outbound transformer to extract the video URL.
func extractVideoURLFromResponseBody(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var v llm.VideoResponse
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}

	return v.VideoURL, nil
}

// openVideoStream opens an HTTP GET to the video URL and returns the response body
// as an io.ReadCloser for streaming. The caller must close the returned reader.
func openVideoStream(ctx context.Context, videoURL string) (io.ReadCloser, string, error) {
	// Validate URL to prevent SSRF attacks
	parsedURL, err := url.Parse(videoURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid video URL: %w", err)
	}

	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, "", fmt.Errorf("invalid URL scheme: %s", parsedURL.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// nolint:gosec // URL has been validated above
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download video: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("failed to download video: HTTP %d", resp.StatusCode)
	}

	filename := filenameFromResponse(resp, videoURL)
	return resp.Body, filename, nil
}

func filenameFromResponse(resp *http.Response, fallbackURL string) string {
	if resp != nil {
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			// Best-effort: look for filename=
			if _, after, ok := strings.Cut(cd, "filename="); ok {
				after = strings.TrimSpace(after)
				after = strings.Trim(after, "\"")
				if after != "" {
					return after
				}
			}
		}
	}

	// Try URL path segment
	u := fallbackURL
	if idx := strings.Index(u, "?"); idx >= 0 {
		u = u[:idx]
	}
	base := filepath.Base(u)
	if base == "." || base == "/" || base == "" {
		return fmt.Sprintf("video-%d.mp4", time.Now().Unix())
	}
	return base
}
