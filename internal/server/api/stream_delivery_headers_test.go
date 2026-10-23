package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type deliveryRawFlushWriter struct {
	*httptest.ResponseRecorder

	flushes int
}

func (w *deliveryRawFlushWriter) FlushError() error {
	w.flushes++
	if w.flushes > 1 {
		return io.ErrClosedPipe
	}
	w.ResponseRecorder.Flush()
	return nil
}

func TestStreamDeliverySSEHeadersWrapperFlushError(t *testing.T) {
	// Given: middleware wraps Gin, which wraps an error-returning transport.
	w := &deliveryRawFlushWriter{ResponseRecorder: httptest.NewRecorder()}
	s := &deliverySSEStream{trace: &deliverySSETrace{}, Stream: streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
	})}
	engine := gin.New()
	engine.Use(middleware.WithResponseHeaders(nil))
	engine.GET("/", func(c *gin.Context) { WriteSSEStream(c, s) })
	// When
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	// Then: the event flush error survives both wrappers and cannot acknowledge.
	require.Equal(t, 2, w.flushes)
	require.False(t, s.DeliveryAcknowledged())
}
