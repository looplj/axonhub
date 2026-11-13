package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
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
// an HTTP/SSE attempt. It also downgrades function outputs without a call ID to
// user messages because Responses HTTP rejects them. Generic Responses HTTP
// requests retain a normal previous_response_id. Codex callers can force removal
// because their HTTP endpoint does not support that field.
func PrepareHTTPTransportRequest(request *httpclient.Request, stripPreviousResponseID bool) *httpclient.Request {
	if request == nil {
		return nil
	}

	headers, webSocketBetaRemoved := withoutWebSocketBeta(request.Headers)
	body, bodyChanged := downgradeCalllessFunctionOutputs(request.Body)
	if stripPreviousResponseID {
		var previousResponseIDRemoved bool
		body, previousResponseIDRemoved = withoutPreviousResponseID(body)
		bodyChanged = bodyChanged || previousResponseIDRemoved
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

// Codex Desktop emits call-ID-less function_call_output items for delegation
// and automation bootstrap data. Responses WebSocket accepts them, but
// Responses HTTP rejects them, so preserve their payload as user content.
func downgradeCalllessFunctionOutputs(body []byte) ([]byte, bool) {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return body, false
	}

	original := body
	changed := false
	for index, item := range input.Array() {
		if item.Get("type").String() != "function_call_output" {
			continue
		}
		callID := item.Get("call_id")
		if callID.Type == gjson.String && strings.TrimSpace(callID.String()) != "" {
			continue
		}

		output := item.Get("output")
		var content any
		switch {
		case output.Type == gjson.String:
			content = []map[string]string{{"type": "input_text", "text": output.String()}}
		case output.IsArray() && len(output.Array()) > 0:
			content = json.RawMessage(output.Raw)
		default:
			text := ""
			if output.Exists() && output.Type != gjson.Null && !output.IsArray() {
				text = output.Raw
			}
			content = []map[string]string{{"type": "input_text", "text": text}}
		}

		replacement, err := json.Marshal(map[string]any{
			"type": "message", "role": "user", "content": content,
		})
		if err != nil {
			return original, false
		}
		body, err = sjson.SetRawBytes(body, "input."+strconv.Itoa(index), replacement)
		if err != nil {
			return original, false
		}
		changed = true
	}

	return body, changed
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
