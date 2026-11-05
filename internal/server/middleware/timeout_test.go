package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestWithLLMTimeoutUnlimitedPreservesCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	router := gin.New()
	router.Use(WithLLMTimeout(0))
	router.GET("/", func(c *gin.Context) {
		handlerContext := c.Request.Context()
		_, hasDeadline := handlerContext.Deadline()
		require.False(t, hasDeadline)
		require.NoError(t, handlerContext.Err())
		cancelParent()
		require.ErrorIs(t, handlerContext.Err(), context.Canceled)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil).WithContext(parent))
}

func TestWithLLMTimeoutFiniteDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(WithLLMTimeout(10 * time.Millisecond))
	router.GET("/", func(c *gin.Context) {
		_, hasDeadline := c.Request.Context().Deadline()
		require.True(t, hasDeadline)
		<-c.Request.Context().Done()
		require.ErrorIs(t, c.Request.Context().Err(), context.DeadlineExceeded)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestWithLLMTimeoutCancelsContextAfterHandlerReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var handlerErr func() error
	router := gin.New()
	router.Use(WithLLMTimeout(0))
	router.GET("/", func(c *gin.Context) { handlerErr = c.Request.Context().Err })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	require.ErrorIs(t, handlerErr(), context.Canceled)
}

func TestLLMTimeoutAndStreamIdleTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name        string
		total, idle time.Duration
		cause       error
	}{
		{"idle with unlimited total", 0, 10 * time.Millisecond, httpclient.ErrStreamIdleTimeout},
		{"total expires first", 10 * time.Millisecond, time.Second, context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(WithLLMTimeout(tt.total), WithStreamIdleTimeout(tt.idle))
			router.GET("/", func(c *gin.Context) {
				upstreamContext, _, stop := httpclient.NewStreamIdleContext(c.Request.Context())
				defer stop()
				select {
				case <-upstreamContext.Done():
				case <-time.After(2 * time.Second):
					t.Fatal("upstream timeout did not fire")
				}
				require.ErrorIs(t, context.Cause(upstreamContext), tt.cause)
				if tt.total == 0 {
					require.NoError(t, c.Request.Context().Err(), "upstream idle must not cancel the downstream request")
				}
			})
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		})
	}
}
