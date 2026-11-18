// 职责: 守住 Ollama 流式响应按出站协议（而非 Content-Type）选解码器，以及上游错误行的透出。
// 关联文件:
//   llm/transformer/ollama/decoder.go
//   llm/transformer/ollama/outbound.go
//   llm/httpclient/client.go
//   llm/httpclient/decoder.go
// 注意事项:
//   ollama.com 的 /api/chat 流是 NDJSON，但响应头写 application/json；
//   按 Content-Type 回退到 SSE 会得到 0 个事件，表现为 stream ended without terminal event。
package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// ollamaCloudStream is a streaming /api/chat body as ollama.com sends it:
// NDJSON, with a final done:true chunk.
const ollamaCloudStream = `{"model":"deepseek-v4.1-flash","created_at":"2026-11-17T00:59:41Z","message":{"role":"assistant","content":"","thinking":"We"},"done":false}
{"model":"deepseek-v4.1-flash","created_at":"2026-11-17T00:59:41Z","message":{"role":"assistant","content":"Hi"},"done":false}
{"model":"deepseek-v4.1-flash","created_at":"2026-11-17T00:59:41Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":31,"eval_count":2}
`

// TestDoStreamDecodesOllamaCloudStream covers ollama.com labeling its NDJSON
// stream as application/json. The decoder must be chosen by the outbound API
// format, not by that header, or the SSE fallback yields no events and the
// pipeline fails with "stream ended without terminal event".
func TestDoStreamDecodesOllamaCloudStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ollamaCloudStream)
	}))
	t.Cleanup(server.Close)

	stream, err := httpclient.NewHttpClient().DoStream(context.Background(), &httpclient.Request{
		Method:    http.MethodPost,
		URL:       server.URL,
		Body:      []byte(`{"model":"deepseek-v4.1-flash","stream":true}`),
		APIFormat: llm.APIFormatOllamaChat.String(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	var events []*httpclient.StreamEvent
	for stream.Next() {
		events = append(events, stream.Current())
	}
	require.NoError(t, stream.Err())
	require.Len(t, events, 3)

	transformer, err := NewOutboundTransformerWithConfig(&Config{BaseURL: server.URL})
	require.NoError(t, err)

	responses, err := transformer.TransformStream(context.Background(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	var thinking, content string
	var finishReason string
	for responses.Next() {
		resp := responses.Current()
		require.Len(t, resp.Choices, 1)
		if delta := resp.Choices[0].Delta; delta != nil {
			if delta.ReasoningContent != nil {
				thinking += *delta.ReasoningContent
			}
			if delta.Content.Content != nil {
				content += *delta.Content.Content
			}
		}
		if reason := resp.Choices[0].FinishReason; reason != nil {
			finishReason = *reason
		}
	}
	require.NoError(t, responses.Err())
	require.Equal(t, "We", thinking)
	require.Equal(t, "Hi", content)
	require.Equal(t, "stop", finishReason, "the last chunk's done:true must be a terminal event")
}

func TestTransformResponseSurfacesOllamaError(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{BaseURL: "http://localhost:11434"})
	require.NoError(t, err)

	_, err = transformer.TransformResponse(context.Background(), &httpclient.Response{
		Body: []byte(`{"error":"model requires a subscription"}`),
	})
	require.ErrorContains(t, err, "model requires a subscription")
}

func TestTransformStreamChunkSurfacesOllamaError(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{BaseURL: "http://localhost:11434"})
	require.NoError(t, err)

	ollamaTransformer, ok := transformer.(*OutboundTransformer)
	require.True(t, ok)

	_, err = ollamaTransformer.TransformStreamChunk(context.Background(), &httpclient.StreamEvent{
		Data: []byte(`{"error":"invalid api key"}`),
	})
	require.ErrorContains(t, err, "invalid api key")
}
