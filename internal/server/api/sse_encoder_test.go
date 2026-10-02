package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestSSEEncoder_ExplicitProtocol_when_PathDiffers(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		t.Run(fmtResponsesCase(heartbeat, nil), func(t *testing.T) {
			// Given a protocol registered on an arbitrary route.
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/custom/stream", nil)
			stream := &errorAfterStream{err: io.ErrUnexpectedEOF}
			h := &ChatCompletionHandlers{
				sseKeepAlive:       SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Hour},
				sseHeartbeatFormat: sseHeartbeatOpenAI,
				streamAdapterFactory: func(_ context.Context, stream streams.Stream[*httpclient.StreamEvent], _ *biz.SystemService) (streams.Stream[*httpclient.StreamEvent], StreamErrorEncoder) {
					return stream, func(_ context.Context, err error) (*httpclient.StreamEvent, error) {
						require.ErrorIs(t, err, io.ErrUnexpectedEOF)
						return &httpclient.StreamEvent{Type: "protocol.failure", Data: []byte(`{"failure":true}`)}, nil
					}
				},
			}
			// When the shared writer finalizes the stream.
			h.writeSSEStream(c, stream)
			// Then the encoder owns the event name and the original error survives.
			require.Equal(t, 1, strings.Count(w.Body.String(), "event:protocol.failure\n"))
			require.NotContains(t, w.Body.String(), "event:error")
			require.ErrorIs(t, stream.Err(), io.ErrUnexpectedEOF)
		})
	}
}

type expectedChoiceStream struct {
	*errorAfterStream
}

func (s *expectedChoiceStream) ExpectedStreamChoices() int { return 1 }

func TestSSEEncoder_TerminalEvidence_when_PolicyWrapsStream(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		t.Run(fmtResponsesCase(heartbeat, nil), func(t *testing.T) {
			// Given a completed choice followed by a transport error through policy and header wrappers.
			ctx, svc := setupUpstreamErrorPolicyTest(t, biz.UpstreamErrorPolicy{Mode: biz.UpstreamErrorModeHidden})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
			source := &expectedChoiceStream{&errorAfterStream{items: []*httpclient.StreamEvent{
				{Data: []byte(`{"choices":[{"index":0,"finish_reason":"stop"}]}`)},
			}, err: io.ErrUnexpectedEOF}}
			stream := primeCodexTurnStateHeader(w.Header(), newUpstreamErrorStream(ctx, source, svc))
			h := &ChatCompletionHandlers{sseKeepAlive: SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Hour}, sseHeartbeatFormat: sseHeartbeatOpenAI}
			// When the common SSE writer drains the stream.
			h.writeSSEStream(c, stream)
			// Then completion evidence suppresses the trailing error without losing its cause.
			require.NotContains(t, w.Body.String(), "event:error")
			require.ErrorIs(t, stream.Err(), io.ErrUnexpectedEOF)
		})
	}
}
