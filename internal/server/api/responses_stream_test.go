package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResponsesStream_ProtocolError_when_Interrupted(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, streamErr := range []error{io.ErrUnexpectedEOF, nil, context.DeadlineExceeded} {
			t.Run(fmtResponsesCase(heartbeat, streamErr), func(t *testing.T) {
				// Given partial output with a provider sequence that does not start at zero.
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				stream := &errorAfterStream{items: []*httpclient.StreamEvent{
					{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","sequence_number":41,"delta":"hello"}`)},
				}, err: streamErr}
				h := &ChatCompletionHandlers{streamAdapterFactory: newResponsesStreamAdapter, sseKeepAlive: SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Hour}, sseHeartbeatFormat: sseHeartbeatOpenAI}
				// When
				h.writeSSEStream(c, stream)
				// Then the partial output is followed by exactly one Responses error.
				require.Equal(t, 2, strings.Count(w.Body.String(), "data:"))
				event := parseSSEErrorEvent(t, w.Body.String())
				require.Equal(t, "error", event["type"])
				require.Equal(t, float64(42), event["sequence_number"])
				require.NotEmpty(t, event["code"])
				require.NotEmpty(t, event["message"])
				require.Contains(t, event, "param")
				require.Nil(t, event["param"])
			})
		}
	}
}

func TestResponsesStream_ContextOutcome_when_Interrupted(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, deadline := range []bool{false, true} {
			t.Run(fmtResponsesCase(heartbeat, nil)+fmt.Sprint(deadline), func(t *testing.T) {
				// Given partial output and a canceled or expired request context.
				ctx, cancel := context.WithCancel(context.Background())
				if deadline {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 0)
				}
				cancel()
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
				stream := &errorAfterStream{items: []*httpclient.StreamEvent{{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","sequence_number":4,"delta":"hello"}`)}}, err: ctx.Err()}
				h := &ChatCompletionHandlers{streamAdapterFactory: newResponsesStreamAdapter, sseKeepAlive: SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Hour}, sseHeartbeatFormat: sseHeartbeatOpenAI}
				// When
				h.writeSSEStream(c, stream)
				// Then cancellation is silent and a server deadline is a protocol error.
				if deadline {
					event := parseSSEErrorEvent(t, w.Body.String())
					require.Equal(t, "error", event["type"])
					require.Equal(t, float64(5), event["sequence_number"])
				} else {
					require.NotContains(t, w.Body.String(), "event:error")
				}
			})
		}
	}
}

func TestResponsesStream_Policy_when_Interrupted(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, mode := range []string{biz.UpstreamErrorModeHidden, biz.UpstreamErrorModeCustom} {
			t.Run(mode+fmtResponsesCase(heartbeat, nil), func(t *testing.T) {
				// Given an interruption after content under the configured policy.
				ctx, svc := setupUpstreamErrorPolicyTest(t, biz.UpstreamErrorPolicy{Mode: mode, CustomMessage: "safe custom failure"})
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
				stream := newUpstreamErrorStream(ctx, &errorAfterStream{err: syscall.ECONNRESET}, svc)
				h := &ChatCompletionHandlers{streamAdapterFactory: newResponsesStreamAdapter, ChatCompletionOrchestrator: &orchestrator.ChatCompletionOrchestrator{SystemService: svc}, sseKeepAlive: SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Hour}, sseHeartbeatFormat: sseHeartbeatOpenAI}
				// When
				h.writeSSEStream(c, stream)
				// Then redaction preserves the transport classification.
				event := parseSSEErrorEvent(t, w.Body.String())
				require.Equal(t, "error", event["type"])
				require.Equal(t, orchestrator.ErrCodeUpstreamStreamInterrupted, event["code"])
				expected := biz.DefaultUpstreamErrorMessage
				if mode == biz.UpstreamErrorModeCustom {
					expected = "safe custom failure"
				}
				require.Equal(t, expected, event["message"])
				require.ErrorIs(t, stream.Err(), syscall.ECONNRESET)
			})
		}
	}
}

func TestResponsesStream_NoTrailingError_when_TerminalPrecedesDeadline(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, terminal := range []string{"response.completed", "response.failed", "response.incomplete", "error"} {
			t.Run(terminal+fmtResponsesCase(heartbeat, nil), func(t *testing.T) {
				// Given a terminal response and an expired server deadline.
				ctx, cancel := context.WithTimeout(context.Background(), 0)
				defer cancel()
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
				data, err := json.Marshal(map[string]string{"type": terminal})
				require.NoError(t, err)
				stream := &errorAfterStream{items: []*httpclient.StreamEvent{{Type: terminal, Data: data}}, err: context.DeadlineExceeded}
				h := &ChatCompletionHandlers{streamAdapterFactory: newResponsesStreamAdapter, sseKeepAlive: SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Hour}, sseHeartbeatFormat: sseHeartbeatOpenAI}
				// When
				h.writeSSEStream(c, stream)
				// Then
				require.Equal(t, 1, strings.Count(w.Body.String(), "data:"))
			})
		}
	}
}

func fmtResponsesCase(heartbeat bool, err error) string {
	name := "without_heartbeat"
	if heartbeat {
		name = "with_heartbeat"
	}
	if err != nil {
		return name + "/" + err.Error()
	}
	return name + "/eof"
}
