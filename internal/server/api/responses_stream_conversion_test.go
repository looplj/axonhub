package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestResponsesStream_ConversionOutcome_when_SourceEnds(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, sourceErr := range []error{nil, io.ErrUnexpectedEOF, context.Canceled, context.DeadlineExceeded} {
			for _, mode := range []string{biz.UpstreamErrorModePassthrough, biz.UpstreamErrorModeHidden, biz.UpstreamErrorModeCustom} {
				t.Run(mode+"/"+fmtResponsesCase(heartbeat, sourceErr), func(t *testing.T) {
					// Given a real Responses converter with partial content and a source failure.
					ctx, svc := setupUpstreamErrorPolicyTest(t, biz.UpstreamErrorPolicy{Mode: mode, CustomMessage: "safe conversion failure"})
					partial := &llm.Response{ID: "resp_conversion", Model: "test", Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{Role: "assistant", Content: llm.MessageContent{Content: new("hello")}}}}}
					source := streams.MapErr(streams.SliceStream([]*llm.Response{partial, nil}), func(chunk *llm.Response) (*llm.Response, error) {
						if chunk == nil {
							return nil, sourceErr
						}
						return chunk, nil
					})
					converted, err := responses.NewInboundTransformer().TransformStream(ctx, source)
					require.NoError(t, err)
					w := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(w)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
					h := &ChatCompletionHandlers{streamAdapterFactory: newResponsesStreamAdapter, ChatCompletionOrchestrator: &orchestrator.ChatCompletionOrchestrator{SystemService: svc}, sseKeepAlive: SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Hour}, sseHeartbeatFormat: sseHeartbeatOpenAI}
					// When
					h.writeSSEStream(c, newUpstreamErrorStream(ctx, converted, svc))
					// Then partial content is retained and cancellation produces no error.
					body := w.Body.String()
					require.Contains(t, body, `"delta":"hello"`)
					failures := strings.Count(body, "event:error\n") + strings.Count(body, "event:response.failed\n")
					if errors.Is(sourceErr, context.Canceled) {
						require.Zero(t, failures)
						return
					}
					require.Equal(t, 1, failures)
					require.NotContains(t, body, "event:response.completed")
					if mode == biz.UpstreamErrorModeCustom && !errors.Is(sourceErr, context.DeadlineExceeded) {
						require.Contains(t, body, "safe conversion failure")
					}
					if mode == biz.UpstreamErrorModeHidden && !errors.Is(sourceErr, context.DeadlineExceeded) {
						require.Contains(t, body, biz.DefaultUpstreamErrorMessage)
					}
					decoder := httpclient.NewDefaultSSEDecoder(ctx, io.NopCloser(strings.NewReader(body)))
					var lastSequence int64 = -1
					for decoder.Next() {
						event := decoder.Current()
						sequence := gjson.GetBytes(event.Data, "sequence_number").Int()
						require.Greater(t, sequence, lastSequence)
						lastSequence = sequence
						if event.Type == "response.failed" {
							require.Equal(t, "resp_conversion", gjson.GetBytes(event.Data, "response.id").String())
						}
					}
					require.NoError(t, decoder.Err())
					require.NoError(t, decoder.Close())
				})
			}
		}
	}
}
