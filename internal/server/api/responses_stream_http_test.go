package api

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestResponsesStream_HTTPReset_when_ContentDelivered(t *testing.T) {
	for _, conversion := range []bool{false, true} {
		for _, heartbeat := range []bool{false, true} {
			name := fmt.Sprintf("conversion_%t/heartbeat_%t", conversion, heartbeat)
			t.Run(name, func(t *testing.T) {
				// Given a TCP upstream that resets only after the client receives content.
				consumed := make(chan struct{})
				var releaseOnce sync.Once
				release := func() { releaseOnce.Do(func() { close(consumed) }) }
				defer release()
				var attempts atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					w.Header().Set("Content-Type", "text/event-stream")
					payload := "event: response.created\ndata: {\"type\":\"response.created\",\"sequence_number\":10,\"response\":{\"id\":\"resp_test\",\"object\":\"response\",\"model\":\"test-model\",\"created_at\":1,\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"sequence_number\":11,\"item_id\":\"msg_test\",\"output_index\":0,\"content_index\":0,\"delta\":\"hello\"}\n\n"
					if conversion {
						payload = "data: {\"id\":\"resp_test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"}}]}\n\n"
					}
					if _, err := io.WriteString(w, payload); err != nil {
						return
					}
					w.(http.Flusher).Flush()
					select {
					case <-consumed:
					case <-r.Context().Done():
						return
					}
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						return
					}
					defer conn.Close()
					if tcp, ok := conn.(*net.TCPConn); ok {
						if err := tcp.SetLinger(0); err != nil {
							return
						}
					}
				}))
				t.Cleanup(upstream.Close)
				h := &ChatCompletionHandlers{streamAdapterFactory: newResponsesStreamAdapter, sseKeepAlive: SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Millisecond}, sseHeartbeatFormat: sseHeartbeatOpenAI}
				gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c, _ := gin.CreateTestContext(w)
					c.Request = r
					request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream.URL, nil)
					if err != nil {
						t.Error(err)
						return
					}
					resp, err := upstream.Client().Do(request)
					if err != nil {
						t.Error(err)
						return
					}
					defer resp.Body.Close()
					var stream streams.Stream[*httpclient.StreamEvent] = httpclient.NewDefaultSSEDecoder(r.Context(), resp.Body)
					if conversion {
						outbound, err := openai.NewOutboundTransformer(upstream.URL, "test")
						if err != nil {
							t.Error(err)
							return
						}
						unified, err := outbound.TransformStream(r.Context(), &httpclient.Request{}, stream)
						if err != nil {
							t.Error(err)
							return
						}
						stream, err = responses.NewInboundTransformer().TransformStream(r.Context(), unified)
						if err != nil {
							t.Error(err)
							return
						}
					}
					h.writeSSEStream(c, stream)
				}))
				t.Cleanup(gateway.Close)
				client := &http.Client{Timeout: 10 * time.Second}
				// When the real HTTP client consumes the gateway stream.
				resp, err := client.Get(gateway.URL + "/v1/responses")
				require.NoError(t, err)
				defer resp.Body.Close()
				reader := bufio.NewReader(resp.Body)
				var prefix strings.Builder
				contentSeen, heartbeatSeen := false, !heartbeat
				for !contentSeen || !heartbeatSeen {
					line, err := reader.ReadString('\n')
					require.NoError(t, err)
					prefix.WriteString(line)
					contentSeen = contentSeen || strings.Contains(line, `"delta":"hello"`)
					heartbeatSeen = heartbeatSeen || strings.HasPrefix(line, ": keep-alive")
				}
				release()
				suffix, err := io.ReadAll(reader)
				require.NoError(t, err)
				body := []byte(prefix.String() + string(suffix))
				// Then content is preserved, there is one failure and no replay.
				require.Contains(t, string(body), "hello")
				require.Equal(t, int32(1), attempts.Load())
				terminal := "error"
				if conversion {
					terminal = "response.failed"
				}
				require.Equal(t, 1, strings.Count(string(body), "event:"+terminal+"\n"))
				decoder := httpclient.NewDefaultSSEDecoder(context.Background(), io.NopCloser(strings.NewReader(string(body))))
				sequence := int64(-1)
				for decoder.Next() {
					event := decoder.Current()
					seq := gjson.GetBytes(event.Data, "sequence_number").Int()
					require.Greater(t, seq, sequence)
					sequence = seq
					if event.Type == "response.failed" {
						require.Equal(t, "resp_test", gjson.GetBytes(event.Data, "response.id").String())
						require.Equal(t, "failed", gjson.GetBytes(event.Data, "response.status").String())
					}
				}
				require.NoError(t, decoder.Err())
				require.NoError(t, decoder.Close())
				t.Logf("wire %s:\n%s", name, body)
				if dir := os.Getenv("AXONHUB_RESPONSES_QA_DIR"); dir != "" {
					require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("conversion_%t_heartbeat_%t.sse", conversion, heartbeat)), body, 0o600))
				}
			})
		}
	}
}

func TestResponsesStream_Policy_when_ConversionFailed(t *testing.T) {
	for _, mode := range []string{biz.UpstreamErrorModeHidden, biz.UpstreamErrorModeCustom} {
		t.Run(mode, func(t *testing.T) {
			// Given a conversion failure whose message contains upstream detail.
			ctx, svc := setupUpstreamErrorPolicyTest(t, biz.UpstreamErrorPolicy{Mode: mode, CustomMessage: "custom safe message"})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
			h := &ChatCompletionHandlers{streamAdapterFactory: newResponsesStreamAdapter, ChatCompletionOrchestrator: &orchestrator.ChatCompletionOrchestrator{SystemService: svc}}
			stream := &errorAfterStream{items: []*httpclient.StreamEvent{{Type: "response.failed", Data: []byte(`{"type":"response.failed","sequence_number":8,"response":{"id":"resp_policy","status":"failed","output":[],"error":{"code":"stream_error","message":"private upstream detail"}}}`)}}}
			// When
			h.writeSSEStream(c, stream)
			// Then the configured safe message replaces only the upstream message.
			require.NotContains(t, w.Body.String(), "private upstream detail")
			require.Contains(t, w.Body.String(), "resp_policy")
			require.Contains(t, w.Body.String(), `"sequence_number":8`)
			expected := biz.DefaultUpstreamErrorMessage
			if mode == biz.UpstreamErrorModeCustom {
				expected = "custom safe message"
			}
			require.Contains(t, w.Body.String(), expected)
		})
	}
}
