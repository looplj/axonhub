package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type deliverySocketWriter struct {
	http.ResponseWriter

	trace *deliverySSETrace
}

func (w *deliverySocketWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	w.trace.add("socket_header_commit")
}

func (w *deliverySocketWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.trace.add("socket_write_return")
	return n, err
}

func (w *deliverySocketWriter) FlushError() error {
	err := http.NewResponseController(w.ResponseWriter).Flush()
	w.trace.add("socket_flush_return")
	return err
}

func TestStreamDeliverySSELoopback(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, primed := range []bool{false, true} {
			name := map[bool]string{false: "ordinary", true: "heartbeat"}[heartbeat] + "/" + map[bool]string{false: "unprimed", true: "primed"}[primed]
			t.Run(name, func(t *testing.T) {
				// Given: real loopback HTTP, Gin and the production SSE driver.
				trace := &deliverySSETrace{}
				done := make(chan struct{})
				source := &deliverySSEStream{trace: trace, Stream: streams.SliceStream([]*httpclient.StreamEvent{
					{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
				})}
				engine := gin.New()
				engine.GET("/", func(c *gin.Context) {
					defer close(done)
					var stream streams.Stream[*httpclient.StreamEvent] = source
					if primed {
						stream = primeCodexTurnStateHeader(c.Writer.Header(), stream)
					}
					runDeliverySSE(c, stream, heartbeat)
				})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					engine.ServeHTTP(&deliverySocketWriter{ResponseWriter: w, trace: trace}, r)
				}))
				t.Cleanup(server.Close)
				// When
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
				require.NoError(t, err)
				resp, err := server.Client().Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				<-done
				// Then: socket write and flush return precede the shared acknowledgement.
				require.Contains(t, string(body), `"type":"message_stop"`)
				require.True(t, source.DeliveryAcknowledged())
				require.Contains(t, trace.steps, "socket_header_commit")
				require.Less(t, slices.Index(trace.steps, "socket_header_commit"), slices.Index(trace.steps, "socket_flush_return"))
				require.Equal(t, "socket_flush_return", trace.steps[len(trace.steps)-2])
				require.Equal(t, "ack", trace.steps[len(trace.steps)-1])
				t.Logf("real loopback driver order: %v", trace.steps)
			})
		}
	}
}
