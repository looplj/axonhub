package responses

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestAnthropicWebSearchTwoTurnHistory(t *testing.T) {
	for _, status := range []string{"completed", "failed"} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			out, err := NewOutboundTransformer("https://api.openai.com", "key")
			require.NoError(t, err)
			in := anthropic.NewInboundTransformer()
			first, err := out.TransformResponse(ctx, &httpclient.Response{Body: []byte(fmt.Sprintf(`{"id":"resp_1","model":"test","status":"completed","output":[{"id":"ws_1","type":"web_search_call","status":%q,"action":{"type":"search","query":"Debian","sources":[{"type":"url","url":"https://debian.org","title":"Debian"}]}},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Answer"}]}]}`, status))})
			require.NoError(t, err)
			reply, err := in.TransformResponse(ctx, first)
			require.NoError(t, err)
			content := gjson.GetBytes(reply.Body, "content").Raw
			// Echo the actual response alongside an ordinary function with the same name.
			content = content[:len(content)-1] + `,{"type":"tool_use","id":"client_search","name":"web_search","input":{"query":"local"}}]`
			followup := []byte(fmt.Sprintf(`{"model":"test","max_tokens":100,"messages":[{"role":"user","content":"Search Debian"},{"role":"assistant","content":%s},{"role":"user","content":[{"type":"tool_result","tool_use_id":"client_search","content":"local result"},{"type":"text","text":"Summarize those sources"}]}]}`, content))
			unified, err := in.TransformRequest(ctx, &httpclient.Request{Body: followup, Headers: http.Header{"Content-Type": []string{"application/json"}}})
			require.NoError(t, err)
			converted, err := out.TransformRequest(ctx, unified)
			require.NoError(t, err)
			calls, results := 0, 0
			var history string
			for _, item := range gjson.GetBytes(converted.Body, "input").Array() {
				switch item.Get("type").String() {
				case "function_call":
					calls++
					require.Equal(t, "client_search", item.Get("call_id").String())
				case "function_call_output":
					results++
					require.Equal(t, "client_search", item.Get("call_id").String())
				case "message":
					for _, part := range item.Get("content").Array() {
						history += part.Get("text").String()
					}
				}
			}
			require.Equal(t, 1, calls)
			require.Equal(t, 1, results)
			require.Contains(t, history, "Debian")
			require.Contains(t, history, "srvtoolu_ws_1")
			require.Contains(t, history, "Answer")
			if status == "completed" {
				require.Contains(t, history, "https://debian.org")
			} else {
				require.Contains(t, history, "unavailable")
			}
		})
	}
}

func TestAnthropicWebSearchTerminalSnapshots(t *testing.T) {
	for _, status := range []string{"completed", "incomplete", "failed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			out, err := NewOutboundTransformer("https://api.openai.com", "key")
			require.NoError(t, err)
			terminal := fmt.Sprintf(`{"type":"response.%s","response":{"id":"resp_1","model":"test","status":%q,"output":[{"id":"ws_1","type":"web_search_call","status":"completed","action":{"type":"search","query":"Debian","sources":[{"type":"url","url":"https://debian.org","title":"Debian"}]}}]}}`, status, status)
			events := []*httpclient.StreamEvent{
				{Data: []byte(`{"type":"response.created","response":{"id":"resp_1","model":"test"}}`)},
				{Data: []byte(`{"type":"response.output_item.done","item":{"id":"ws_1","type":"web_search_call","status":"completed"}}`)},
				{Data: []byte(`{"type":"response.output_text.delta","delta":"Partial answer"}`)},
				{Data: []byte(terminal)},
			}
			unified, err := out.TransformStream(ctx, nil, streams.SliceStream(events))
			require.NoError(t, err)
			converted, err := anthropic.NewInboundTransformer().TransformStream(ctx, unified)
			require.NoError(t, err)
			wire, err := streams.All(converted)
			require.NoError(t, err)
			results := 0
			for _, event := range wire {
				if gjson.GetBytes(event.Data, "content_block.type").String() == "web_search_tool_result" {
					results++
					require.Equal(t, "https://debian.org", gjson.GetBytes(event.Data, "content_block.content.0.url").String())
				}
			}
			require.Equal(t, 1, results)
		})
	}
}
