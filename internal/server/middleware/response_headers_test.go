package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
)

type responseHeadersStoreForTest struct {
	requestID int
	headers   http.Header
}

func (s *responseHeadersStoreForTest) UpdateRequestResponseHeaders(_ context.Context, requestID int, headers http.Header) error {
	s.requestID = requestID
	s.headers = headers.Clone()
	return nil
}

func TestWithResponseHeadersPersistsAfterRequestCreation(t *testing.T) {
	store := &responseHeadersStoreForTest{}
	router := gin.New()
	router.Use(WithResponseHeaders(store))
	router.POST("/test", func(c *gin.Context) {
		contexts.NotifyRequestRecord(c.Request.Context(), 42)
		c.Header("X-Request-Header", "captured")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/test", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 42, store.requestID)
	require.Equal(t, "captured", store.headers.Get("X-Request-Header"))
}
