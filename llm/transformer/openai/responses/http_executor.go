package responses

import (
	"context"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

type httpTransportExecutor struct {
	inner    pipeline.Executor
	finalize func(*httpclient.Request) *httpclient.Request
}

func (e *httpTransportExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	return e.inner.Do(ctx, e.finalize(request))
}

func (e *httpTransportExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return e.inner.DoStream(ctx, e.finalize(request))
}

// PrepareHTTPTransportRequest removes WebSocket-v2-only request metadata before
// an HTTP/SSE attempt. Generic Responses HTTP requests retain a normal
// previous_response_id unless the WebSocket beta marker proves it came from a
// WebSocket continuation. Codex callers can force removal because their HTTP
// endpoint does not support that field.
func PrepareHTTPTransportRequest(request *httpclient.Request, stripPreviousResponseID bool) *httpclient.Request {
	if request == nil {
		return nil
	}

	headers, webSocketBetaRemoved := withoutWebSocketBeta(request.Headers)
	if webSocketBetaRemoved {
		stripPreviousResponseID = true
	}

	body := request.Body
	bodyChanged := false
	if stripPreviousResponseID {
		body, bodyChanged = withoutPreviousResponseID(body)
	}

	if !webSocketBetaRemoved && !bodyChanged {
		return request
	}

	cloned := *request
	if webSocketBetaRemoved {
		cloned.Headers = headers
	}
	if bodyChanged {
		cloned.Body = body
	}

	return &cloned
}

func withoutPreviousResponseID(body []byte) ([]byte, bool) {
	if !gjson.GetBytes(body, "previous_response_id").Exists() {
		return body, false
	}

	filtered, err := sjson.DeleteBytes(body, "previous_response_id")
	if err != nil {
		return body, false
	}

	return filtered, true
}

func withoutWebSocketBeta(headers http.Header) (http.Header, bool) {
	if headers == nil {
		return nil, false
	}

	values := headers.Values("OpenAI-Beta")
	if len(values) == 0 {
		return headers, false
	}

	filtered := make([]string, 0, len(values))
	changed := false
	for _, value := range values {
		parts := strings.Split(value, ",")
		kept := parts[:0]
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "responses_websockets=") {
				changed = true
				continue
			}
			if part != "" {
				kept = append(kept, part)
			}
		}
		if len(kept) > 0 {
			filtered = append(filtered, strings.Join(kept, ", "))
		}
	}
	if !changed {
		return headers, false
	}

	cloned := headers.Clone()
	cloned.Del("OpenAI-Beta")
	for _, value := range filtered {
		cloned.Add("OpenAI-Beta", value)
	}

	return cloned, true
}
