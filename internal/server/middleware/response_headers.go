package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
)

type responseHeadersStore interface {
	UpdateRequestResponseHeaders(context.Context, int, http.Header) error
}

// WithResponseHeaders captures application headers at the downstream HTTP boundary.
// WebSocket messages have no per-response HTTP headers and are left unset.
func WithResponseHeaders(store responseHeadersStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.IsWebsocket() {
			c.Next()
			return
		}
		var requestID int
		ctx := contexts.WithRequestRecordObserver(c.Request.Context(), func(id int) { requestID = id })
		c.Request = c.Request.WithContext(ctx)
		writer := &responseHeadersWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()
		if requestID == 0 {
			return
		}
		writer.capture()
		persistCtx, cancel := xcontext.DetachWithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		if err := store.UpdateRequestResponseHeaders(persistCtx, requestID, writer.headers); err != nil {
			log.Warn(persistCtx, "Failed to save downstream response headers", log.Cause(err), log.Int("request_id", requestID))
		}
	}
}

type responseHeadersWriter struct {
	gin.ResponseWriter

	headers http.Header
}

// Unwrap preserves http.ResponseController support (notably SSE write
// deadlines) through this capture wrapper.
func (w *responseHeadersWriter) Unwrap() http.ResponseWriter {
	if unwrapper, ok := w.ResponseWriter.(interface{ Unwrap() http.ResponseWriter }); ok {
		return unwrapper.Unwrap()
	}

	return w.ResponseWriter
}

func (w *responseHeadersWriter) capture() {
	if w.headers == nil {
		w.headers = w.Header().Clone()
	}
}

func (w *responseHeadersWriter) WriteHeaderNow() {
	w.capture()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *responseHeadersWriter) Write(body []byte) (int, error) {
	w.capture()
	return w.ResponseWriter.Write(body)
}

func (w *responseHeadersWriter) WriteString(body string) (int, error) {
	w.capture()
	return w.ResponseWriter.WriteString(body)
}

func (w *responseHeadersWriter) Flush() {
	w.capture()
	w.ResponseWriter.Flush()
}

// FlushError preserves Gin/custom writer flush errors for SSE disconnect
// detection while still capturing the headers before the first flush.
func (w *responseHeadersWriter) FlushError() error {
	w.capture()
	if flusher, ok := w.ResponseWriter.(interface{ FlushError() error }); ok {
		return flusher.FlushError()
	}

	w.ResponseWriter.Flush()
	return nil
}
