package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/server/biz"
)

type RequestContentHandlersParams struct {
	fx.In

	DataStorageService *biz.DataStorageService
}

type RequestContentHandlers struct {
	DataStorageService *biz.DataStorageService
}

func NewRequestContentHandlers(params RequestContentHandlersParams) *RequestContentHandlers {
	return &RequestContentHandlers{
		DataStorageService: params.DataStorageService,
	}
}

type DownloadContentRequest struct {
	RequestID int `uri:"request_id"`
}

func (h *RequestContentHandlers) DownloadRequestContent(c *gin.Context) {
	ctx := c.Request.Context()

	projectID, ok := contexts.GetProjectID(ctx)
	if !ok || projectID <= 0 {
		JSONError(c, http.StatusBadRequest, errors.New("Project ID not found in context"))
		return
	}

	var reqquest DownloadContentRequest
	if err := c.ShouldBindUri(&reqquest); err != nil {
		JSONError(c, http.StatusBadRequest, fmt.Errorf("Invalid request body: %w", err))
		return
	}

	req, err := ent.FromContext(ctx).Request.Get(ctx, reqquest.RequestID)
	if err != nil {
		if ent.IsNotFound(err) {
			JSONError(c, http.StatusNotFound, errors.New("Request not found"))
			return
		}
		JSONError(c, http.StatusInternalServerError, errors.New("Failed to load request"))
		return
	}

	if projectID != req.ProjectID {
		JSONError(c, http.StatusNotFound, errors.New("Request not found"))
		return
	}

	if !req.ContentSaved || req.ContentStorageID == nil || req.ContentStorageKey == nil || strings.TrimSpace(*req.ContentStorageKey) == "" {
		JSONError(c, http.StatusNotFound, errors.New("Content not found"))
		return
	}

	rawKey := strings.TrimSpace(*req.ContentStorageKey)
	key := rawKey
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}
	expectedPrefix := fmt.Sprintf("/%d/requests/%d/", req.ProjectID, req.ID)
	if !strings.HasPrefix(key, expectedPrefix) {
		JSONError(c, http.StatusNotFound, errors.New("Content not found"))
		return
	}

	ds, err := h.DataStorageService.GetDataStorageByID(ctx, *req.ContentStorageID)
	if err != nil {
		if ent.IsNotFound(err) {
			JSONError(c, http.StatusNotFound, errors.New("Content storage not found"))
			return
		}
		JSONError(c, http.StatusInternalServerError, errors.New("Failed to load content storage"))
		return
	}

	if ds.Primary || ds.Type == datastorage.TypeDatabase {
		JSONError(c, http.StatusBadRequest, errors.New("Content storage is not file-based"))
		return
	}

	if ds.Type == datastorage.TypeFs && ds.Settings != nil && ds.Settings.Directory != nil {
		rel, ok := safeRelativePath(key)
		if !ok {
			JSONError(c, http.StatusNotFound, errors.New("Content not found"))
			return
		}

		fullPath := filepath.Join(*ds.Settings.Directory, rel)
		c.FileAttachment(fullPath, filenameFromKey(key, req.ID))
		return
	}

	fs, err := h.DataStorageService.GetFileSystem(ctx, ds)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, errors.New("Failed to open content storage"))
		return
	}

	if ds.Type == datastorage.TypeFs {
		key = filepath.FromSlash(key)
	} else if ds.Type == datastorage.TypeS3 && ds.Settings != nil && ds.Settings.S3 != nil && ds.Settings.S3.PathStyle {
		key = strings.TrimPrefix(key, "/")
	}

	f, err := fs.Open(key)
	if err != nil {
		JSONError(c, http.StatusNotFound, errors.New("Content not found"))
		return
	}
	defer f.Close()

	filename := filenameFromKey(key, req.ID)

	if stat, err := f.Stat(); err == nil && stat != nil {
		c.Header("Content-Length", fmt.Sprintf("%d", stat.Size()))
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Cache-Control", "private, max-age=0, no-cache")

	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}

func safeRelativePath(key string) (string, bool) {
	trimmed := strings.TrimPrefix(filepath.FromSlash(key), "/")
	trimmed = filepath.Clean(trimmed)

	if trimmed == "." || trimmed == "" {
		return "", false
	}

	if trimmed == ".." || strings.HasPrefix(trimmed, ".."+string(filepath.Separator)) {
		return "", false
	}

	return trimmed, true
}

func filenameFromKey(key string, requestID int) string {
	filename := filepath.Base(key)
	if filename == "." || filename == "/" || filename == "" {
		return fmt.Sprintf("request-%d-content", requestID)
	}
	return filename
}
