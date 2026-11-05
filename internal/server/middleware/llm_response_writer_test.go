package middleware

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUnlimitedLLMWaitBeyondServerWriteTimeout(t *testing.T) {
	for _, http2 := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP1", true: "HTTP2"}[http2], func(t *testing.T) {
			router := gin.New()
			router.Use(WithLLMTimeout(0))
			router.GET("/", func(c *gin.Context) {
				c.Writer.(*llmResponseWriter).timeout = 25 * time.Millisecond
				_, _ = c.Writer.WriteString("start ")
				c.Writer.Flush()
				// Simulate upstream work beyond the ordinary route's deadline.
				time.Sleep(100 * time.Millisecond)
				c.String(http.StatusOK, "done")
			})
			server := httptest.NewUnstartedServer(router)
			server.Config.WriteTimeout = 25 * time.Millisecond
			server.EnableHTTP2 = http2
			server.StartTLS()
			defer server.Close()
			response, err := server.Client().Get(server.URL)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, map[bool]int{false: 1, true: 2}[http2], response.ProtoMajor)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, "start done", string(body))
		})
	}
}

func TestUnlimitedLLMBlockedDownstreamWrites(t *testing.T) {
	for _, operation := range []string{"binary", "string", "download", "flush"} {
		t.Run(operation, func(t *testing.T) {
			writeErr := make(chan error, 1)
			router := gin.New()
			router.Use(WithLLMTimeout(0))
			router.GET("/", func(c *gin.Context) {
				c.Writer.(*llmResponseWriter).timeout = 25 * time.Millisecond
				payload := strings.Repeat("x", 16<<20)
				var err error
				switch operation {
				case "binary":
					_, err = c.Writer.Write([]byte(payload))
				case "string":
					_, err = c.Writer.WriteString(payload)
				case "download":
					_, err = io.Copy(c.Writer, strings.NewReader(payload))
				case "flush":
					for range 16 << 10 {
						if _, err = c.Writer.Write([]byte(payload[:1024])); err != nil {
							break
						}
						if err = http.NewResponseController(c.Writer).Flush(); err != nil {
							break
						}
					}
				}
				writeErr <- err
			})
			server := httptest.NewServer(router)
			defer server.Close()
			conn, err := net.Dial("tcp", server.Listener.Addr().String())
			require.NoError(t, err)
			defer conn.Close()
			_, err = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n")
			require.NoError(t, err)
			// Leave the socket unread so the real downstream write blocks.
			select {
			case err := <-writeErr:
				require.Error(t, err)
				var netErr net.Error
				require.ErrorAs(t, err, &netErr)
				require.True(t, netErr.Timeout())
			case <-time.After(3 * time.Second):
				t.Fatal("blocked downstream write did not time out")
			}
		})
	}
}

func TestUnlimitedLLMBlockedHTTP2Write(t *testing.T) {
	writeErr := make(chan error, 1)
	router := gin.New()
	router.Use(WithLLMTimeout(0))
	router.GET("/", func(c *gin.Context) {
		c.Writer.(*llmResponseWriter).timeout = 25 * time.Millisecond
		c.Writer.Flush()
		_, err := c.Writer.Write([]byte(strings.Repeat("x", 16<<20)))
		writeErr <- err
	})
	server := httptest.NewUnstartedServer(router)
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 2, response.ProtoMajor)
	// Exhaust the HTTP/2 flow-control window by leaving the body unread.
	select {
	case err := <-writeErr:
		require.ErrorIs(t, err, os.ErrDeadlineExceeded)
	case <-time.After(3 * time.Second):
		t.Fatal("blocked HTTP/2 downstream write did not time out")
	}
}
