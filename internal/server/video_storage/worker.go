package video_storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xtime"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/scheduler"
	"github.com/looplj/axonhub/llm"
)

type Params struct {
	fx.In

	Ent                *ent.Client
	SystemService      *biz.SystemService
	DataStorageService *biz.DataStorageService
	VideoService       *biz.VideoService
}

type Worker struct {
	ent                *ent.Client
	systemService      *biz.SystemService
	dataStorageService *biz.DataStorageService
	videoService       *biz.VideoService
}

var ErrUnsafeVideoURL = errors.New("unsafe video download URL")

var sharedAddressPrefix = netip.MustParsePrefix("100.64.0.0/10")

func NewWorker(params Params) *Worker {
	w := &Worker{
		ent:                params.Ent,
		systemService:      params.SystemService,
		dataStorageService: params.DataStorageService,
		videoService:       params.VideoService,
	}

	return w
}

func (w *Worker) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	ctx = authz.WithSystemBypass(ctx, "video-storage-register")
	settings, err := w.systemService.VideoStorageSettings(ctx)
	if err != nil {
		return fmt.Errorf("get video storage settings for registration: %w", err)
	}

	intervalMinutes := settings.ScanIntervalMinutes
	if intervalMinutes <= 0 {
		intervalMinutes = 1
	}

	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "video-storage",
		Description: "Scan and save generated video requests to external storage",
		FixRate:     time.Duration(intervalMinutes) * time.Minute,
	}, w.runScanWithSystemContext)
}

// Reschedule cancels and re-creates the video storage scan task. Call after
// the scan interval setting changes.
func (w *Worker) Reschedule(ctx context.Context, s *scheduler.Scheduler) {
	ctx = authz.WithSystemBypass(ctx, "video-storage-reschedule")
	settings, err := w.systemService.VideoStorageSettings(ctx)
	if err != nil {
		log.Error(ctx, "Failed to get video storage settings for reschedule", log.Cause(err))
		return
	}

	intervalMinutes := settings.ScanIntervalMinutes
	if intervalMinutes <= 0 {
		intervalMinutes = 1
	}

	if err := s.Reschedule(ctx, "video-storage", scheduler.TaskSpec{
		Name:        "video-storage",
		Description: "Scan and save generated video requests to external storage",
		FixRate:     time.Duration(intervalMinutes) * time.Minute,
	}); err != nil {
		log.Error(ctx, "Failed to reschedule video storage worker", log.Cause(err))
	}
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

	if v, err := extractVideoURLFromResponseBody(req.ResponseBody); err == nil && strings.TrimSpace(v) != "" {
		videoURL = v
	}

	if strings.TrimSpace(videoURL) == "" {
		resp, err := w.videoService.GetTask(ctx, req.ID)
		if err != nil {
			return err
		}

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

func openVideoStream(ctx context.Context, videoURL string) (io.ReadCloser, string, error) {
	parsedURL, err := url.Parse(videoURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid video URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, "", fmt.Errorf("%w: invalid URL scheme: %s", ErrUnsafeVideoURL, parsedURL.Scheme)
	}

	if err := validateVideoDownloadURL(ctx, parsedURL); err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, "", fmt.Errorf("failed to configure safe video transport")
	}
	transport = transport.Clone()
	transport.Proxy = nil
	transport.DialContext = safeVideoDialContext

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(redirectReq *http.Request, _ []*http.Request) error {
			return validateVideoDownloadURL(redirectReq.Context(), redirectReq.URL)
		},
	}

	resp, err := client.Do(req)
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

func validateVideoDownloadURL(ctx context.Context, videoURL *url.URL) error {
	if videoURL == nil || (videoURL.Scheme != "http" && videoURL.Scheme != "https") || videoURL.Hostname() == "" || videoURL.User != nil {
		return fmt.Errorf("%w: URL must be an http(s) URL with a public host", ErrUnsafeVideoURL)
	}

	host := videoURL.Hostname()
	addresses, err := resolveVideoHost(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: failed to resolve host %q: %v", ErrUnsafeVideoURL, host, err)
	}

	for _, address := range addresses {
		if isUnsafeVideoAddress(address) {
			return fmt.Errorf("%w: host %q resolves to private or local address %s", ErrUnsafeVideoURL, host, address)
		}
	}

	return nil
}

func resolveVideoHost(ctx context.Context, host string) ([]netip.Addr, error) {
	if address, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{address.Unmap()}, nil
	}

	resolved, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("host has no addresses")
	}

	addresses := make([]netip.Addr, 0, len(resolved))
	for _, address := range resolved {
		addresses = append(addresses, address.Unmap())
	}

	return addresses, nil
}

func isUnsafeVideoAddress(address netip.Addr) bool {
	return !address.IsValid() ||
		address.IsLoopback() ||
		address.IsPrivate() ||
		address.IsLinkLocalUnicast() ||
		address.IsUnspecified() ||
		address.IsMulticast() ||
		sharedAddressPrefix.Contains(address)
}

func safeVideoDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid video download address %q: %w", address, err)
	}

	addresses, err := resolveVideoHost(ctx, host)
	if err != nil {
		return nil, err
	}

	var lastErr error
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	for _, resolved := range addresses {
		if isUnsafeVideoAddress(resolved) {
			return nil, fmt.Errorf("%w: host %q resolved to private or local address %s", ErrUnsafeVideoURL, host, resolved)
		}

		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to connect to video host %q: %w", host, lastErr)
}

func filenameFromResponse(resp *http.Response, fallbackURL string) string {
	if resp != nil {
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			if _, after, ok := strings.Cut(cd, "filename="); ok {
				after = strings.TrimSpace(after)
				after = strings.Trim(after, "\"")
				if after != "" {
					return after
				}
			}
		}
	}

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
