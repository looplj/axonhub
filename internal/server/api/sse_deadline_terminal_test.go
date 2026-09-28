package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type singleChoiceSSEStream struct {
	streams.Stream[*httpclient.StreamEvent]
}

func (s *singleChoiceSSEStream) ExpectedStreamChoices() int { return 1 }

type triggeredDeadlineContext struct {
	context.Context

	done chan struct{}
}

func (c *triggeredDeadlineContext) Done() <-chan struct{} { return c.done }
func (c *triggeredDeadlineContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

type deadlineAfterFrameWriter struct {
	*httptest.ResponseRecorder

	ctx       *triggeredDeadlineContext
	triggered bool
}

func (w *deadlineAfterFrameWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseRecorder.Write(data)
	if !w.triggered && strings.Contains(string(data), "choices") {
		w.triggered = true
		close(w.ctx.done)
	}
	return n, err
}

func TestWriteSSEStream_DeadlineAfterDeliveredTerminal(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, terminal := range []bool{false, true} {
			name := "ordinary"
			if heartbeat {
				name = "heartbeat"
			}
			if terminal {
				name += "_terminal"
			} else {
				name += "_partial"
			}
			t.Run(name, func(t *testing.T) {
				deadlineCtx := &triggeredDeadlineContext{Context: t.Context(), done: make(chan struct{})}
				writer := &deadlineAfterFrameWriter{ResponseRecorder: httptest.NewRecorder(), ctx: deadlineCtx}
				c, _ := gin.CreateTestContext(writer)
				c.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(deadlineCtx)
				data := `{"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`
				if terminal {
					data = `{"choices":[{"index":0,"finish_reason":"stop"}]}`
				}
				stream := &singleChoiceSSEStream{Stream: &errorAfterStream{
					items: []*httpclient.StreamEvent{{Data: []byte(data)}}, err: errors.New("trailing error"),
				}}
				if heartbeat {
					writeSSEStreamWithHeartbeat(c, stream, FormatStreamError, time.Hour, sseHeartbeatOpenAI)
				} else {
					WriteSSEStream(c, stream)
				}
				body := writer.Body.String()
				require.Contains(t, body, data)
				require.Equal(t, !terminal, strings.Contains(body, "event:error"), body)
			})
		}
	}
}
