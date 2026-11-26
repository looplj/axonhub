// 职责: 覆盖 _upstream_chunks 元数据帧的筛选、首帧剥离和边界输入处理。
// 关联文件:
//
//	internal/server/biz/upstream_meta.go
package biz

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func streamEvent(data string) *httpclient.StreamEvent {
	return &httpclient.StreamEvent{Data: []byte(data)}
}

func embeddedFrames(t *testing.T, body []byte) []json.RawMessage {
	t.Helper()

	var obj map[string]json.RawMessage

	err := json.Unmarshal(body, &obj)
	require.NoError(t, err)

	raw, ok := obj[upstreamMetaChunksKey]
	require.True(t, ok, "missing %s", upstreamMetaChunksKey)

	var frames []json.RawMessage

	err = json.Unmarshal(raw, &frames)
	require.NoError(t, err)

	return frames
}

func TestEmbedUpstreamMetaChunksSkipsAnswerFrames(t *testing.T) {
	body := []byte(`{"id":"gen-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hello"}}],"usage":{"total_tokens":10}}`)

	chunks := []*httpclient.StreamEvent{
		streamEvent(`{"id":"gen-1","provider":"Baidu","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}`),
		streamEvent(`{"provider":"Baidu","choices":[{"index":0,"delta":{"content":"hel"}}]}`),
		streamEvent(`{"provider":"Baidu","choices":[{"index":0,"delta":{"content":"lo"}}]}`),
		streamEvent(`{"provider":"Baidu","choices":[{"index":0,"delta":{"reasoning":"thinking"}}]}`),
		streamEvent(`{"provider":"Baidu","choices":[{"index":0,"delta":{},"finish_reason":"stop","native_finish_reason":"stop"}]}`),
		streamEvent(`{"provider":"Baidu","usage":{"cost":0.001,"cost_details":{"upstream_inference_cost":0.001}}}`),
		streamEvent(`[DONE]`),
	}

	merged := embedUpstreamMetaChunks(body, chunks)
	frames := embeddedFrames(t, merged)

	require.Len(t, frames, 3, "first frame, finish frame and usage frame must be kept")

	var (
		sawFinish, sawUsage bool
	)

	for _, frame := range frames {
		raw := string(frame)
		require.NotContains(t, raw, `"content":"hel"`)
		require.NotContains(t, raw, `"reasoning":"thinking"`)

		if txt := raw; txt != "" {
			if json.Valid(frame) {
				var obj map[string]any

				require.NoError(t, json.Unmarshal(frame, &obj))

				if _, ok := obj["usage"]; ok {
					sawUsage = true
				}

				if choices, ok := obj["choices"].([]any); ok {
					for _, choice := range choices {
						c, _ := choice.(map[string]any)
						if _, ok := c["native_finish_reason"]; ok {
							sawFinish = true
						}
					}
				}
			}
		}
	}

	require.True(t, sawFinish, "native_finish_reason frame must be kept")
	require.True(t, sawUsage, "usage frame must be kept")

	// The aggregated body itself must stay intact.
	var mergedObj map[string]any

	require.NoError(t, json.Unmarshal(merged, &mergedObj))
	require.Equal(t, "gen-1", mergedObj["id"])
}

func TestEmbedUpstreamMetaChunksKeepsFirstFrameWithoutAnswerText(t *testing.T) {
	body := []byte(`{"id":"gen-1","choices":[]}`)
	chunks := []*httpclient.StreamEvent{
		streamEvent(`{"provider":"Baidu","choices":[{"delta":{"role":"assistant","content":"first-token"}}]}`),
		streamEvent(`{"provider":"Baidu","choices":[{"delta":{"content":"second-token"}}]}`),
	}

	frames := embeddedFrames(t, embedUpstreamMetaChunks(body, chunks))
	require.Len(t, frames, 1)
	require.Contains(t, string(frames[0]), `"provider":"Baidu"`)
	require.Contains(t, string(frames[0]), `"role":"assistant"`)
	require.NotContains(t, string(frames[0]), "first-token")
}

func TestEmbedUpstreamMetaChunksLeavesUnsupportedInputUntouched(t *testing.T) {
	chunks := []*httpclient.StreamEvent{streamEvent(`{"provider":"Baidu"}`)}

	require.Equal(t, []byte(`not-json`), embedUpstreamMetaChunks([]byte(`not-json`), chunks))
	require.Equal(t, []byte(`[1,2]`), embedUpstreamMetaChunks([]byte(`[1,2]`), chunks))
	require.Equal(t, []byte(`{"id":"x"}`), embedUpstreamMetaChunks([]byte(`{"id":"x"}`), nil))
	require.Equal(t, []byte(`{"id":"x"}`), embedUpstreamMetaChunks([]byte(`{"id":"x"}`), []*httpclient.StreamEvent{nil}))
}

func TestChunkCarriesAnswerText(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "content", data: `{"choices":[{"delta":{"content":"hi"}}]}`, want: true},
		{name: "reasoning", data: `{"choices":[{"delta":{"reasoning":"think"}}]}`, want: true},
		{name: "reasoning_content", data: `{"choices":[{"delta":{"reasoning_content":"think"}}]}`, want: true},
		{name: "tool arguments", data: `{"choices":[{"delta":{"tool_calls":[{"function":{"name":"bash","arguments":"{\"a\""}}]}}]}`, want: true},
		{name: "audio data", data: `{"choices":[{"delta":{"audio":{"data":"AAAA"}}}]}`, want: true},
		{name: "empty content", data: `{"choices":[{"delta":{"content":""}}]}`, want: false},
		{name: "null content", data: `{"choices":[{"delta":{"content":null}}]}`, want: false},
		{name: "role only", data: `{"choices":[{"delta":{"role":"assistant"}}]}`, want: false},
		{name: "tool declaration", data: `{"choices":[{"delta":{"tool_calls":[{"function":{"name":"bash","arguments":""}}]}}]}`, want: false},
		{name: "usage only", data: `{"usage":{"cost":0.1},"choices":[]}`, want: false},
		{name: "no choices", data: `{"provider":"Baidu"}`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, chunkCarriesAnswerText([]byte(tt.data)))
		})
	}
}
