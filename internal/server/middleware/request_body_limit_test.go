package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/tracing"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestRequestBodyLimitMiddleware 验证已知长度提前拒绝、未知长度实际读限流和 trace 读取。
func TestRequestBodyLimitMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name          string
		size          int
		unknownLength bool
		trace         tracing.Config
		status        int
	}{
		{name: "below", size: (1 << 20) - 1, status: http.StatusNoContent},
		{name: "exact", size: 1 << 20, status: http.StatusNoContent},
		{name: "known oversized", size: (1 << 20) + 1, status: http.StatusRequestEntityTooLarge},
		{name: "chunked exact", size: 1 << 20, unknownLength: true, status: http.StatusNoContent},
		{name: "chunked oversized", size: (1 << 20) + 1, unknownLength: true, status: http.StatusRequestEntityTooLarge},
		{name: "trace body oversized", size: (1 << 20) + 1, unknownLength: true, trace: tracing.Config{ExtraTraceBodyFields: []string{"trace_id"}}, status: http.StatusRequestEntityTooLarge},
		{name: "claude trace oversized", size: (1 << 20) + 1, unknownLength: true, trace: tracing.Config{ClaudeCodeTraceEnabled: true}, status: http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(WithRequestBodyLimit(), WithTrace(tc.trace, nil))
			reachedHandler := false
			router.POST("/v1/messages", func(c *gin.Context) {
				reachedHandler = true
				require.Equal(t, int64(1<<20), httpclient.RequestBodyLimit(c.Request.Context()))
				_, err := io.ReadAll(c.Request.Body)
				if err != nil {
					abortTraceBodyReadError(c, err)
					return
				}
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(bytes.Repeat([]byte(" "), tc.size)))
			req.Header.Set("User-Agent", "claude-cli/1.0.0")
			req = req.WithContext(httpclient.WithRequestBodyLimit(req.Context(), 1<<20))
			if tc.unknownLength {
				req.ContentLength = -1
				req.TransferEncoding = []string{"chunked"}
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			if tc.status == http.StatusRequestEntityTooLarge {
				require.Contains(t, recorder.Body.String(), "request body too large")
				if !tc.unknownLength || tc.trace.ClaudeCodeTraceEnabled || len(tc.trace.ExtraTraceBodyFields) > 0 {
					require.False(t, reachedHandler)
				}
			}
		})
	}
}
