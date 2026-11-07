package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const llmWriteTimeout = 30 * time.Second

// llmResponseWriter bounds socket writes without timing upstream computation or
// reads. The server-wide deadline remains in force for ordinary routes.
type llmResponseWriter struct {
	gin.ResponseWriter

	timeout time.Duration
}

func (w *llmResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *llmResponseWriter) setDeadline(deadline time.Time) {
	// ResponseController unwraps Gin to the HTTP/1 connection or HTTP/2 stream.
	// In-memory response recorders do not support deadlines.
	_ = http.NewResponseController(w.ResponseWriter).SetWriteDeadline(deadline)
}

func (w *llmResponseWriter) boundWrite() func() {
	w.setDeadline(time.Now().Add(w.timeout))
	return func() { w.setDeadline(time.Time{}) }
}

func (w *llmResponseWriter) Write(data []byte) (int, error) {
	defer w.boundWrite()()
	return w.ResponseWriter.Write(data)
}

func (w *llmResponseWriter) WriteString(data string) (int, error) {
	defer w.boundWrite()()
	return w.ResponseWriter.WriteString(data)
}

func (w *llmResponseWriter) WriteHeaderNow() {
	defer w.boundWrite()()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *llmResponseWriter) Flush() { _ = w.FlushError() }

func (w *llmResponseWriter) FlushError() error {
	defer w.boundWrite()()
	w.ResponseWriter.WriteHeaderNow()
	// Gin's Flush discards errors; unwrap it after committing its headers.
	var writer http.ResponseWriter = w.ResponseWriter
	for {
		if flusher, ok := writer.(interface{ FlushError() error }); ok {
			return flusher.FlushError()
		}
		if unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter }); ok {
			writer = unwrapper.Unwrap()
			continue
		}
		return http.NewResponseController(writer).Flush()
	}
}
