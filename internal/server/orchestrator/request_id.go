package orchestrator

import (
	"context"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
)

const upstreamRequestIDHeader = "X-Request-Id"

// applyUpstreamRequestID replaces any client-provided request ID with AxonHub's
// per-request ID. Trace IDs may intentionally span multiple agent calls, while
// X-Request-Id identifies one HTTP request and must remain unique downstream.
func applyUpstreamRequestID() pipeline.Middleware {
	return pipeline.OnRawRequest("upstream-request-id", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		requestID, ok := contexts.GetRequestID(ctx)
		if !ok || strings.TrimSpace(requestID) == "" {
			return request, nil
		}

		if request.Headers == nil {
			request.Headers = make(http.Header)
		}
		request.Headers.Set(upstreamRequestIDHeader, requestID)

		return request, nil
	})
}
