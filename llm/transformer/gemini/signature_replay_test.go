// 职责: 验证 Copilot 文字响应尾帧的签名经 Gemini 流式转换、聚合和下一轮请求完整回传。
// 关联文件:
//
//	llm/transformer/gemini/inbound_convert.go
//	llm/transformer/gemini/aggregator.go
//	llm/transformer/gemini/model.go
//	llm/transformer/openai/copilot/outbound.go
//
// 注意事项:
//
//	使用脱敏样本结构；空文本签名 part 必须保留 text 字段和原始位置。
package gemini

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai/copilot"
)

type signatureReplayTokenProvider struct{}

func (signatureReplayTokenProvider) GetToken(context.Context) (string, error) {
	return "test-token", nil
}

func TestCopilotTailSignatureGeminiReplay(t *testing.T) {
	// Mirrors requests #396937–#396939: the signature arrives with stop,
	// after all reasoning and visible text, without a tool call.
	upstream := []*httpclient.StreamEvent{
		{Data: []byte(`{"choices":[{"index":0,"delta":{"role":"assistant","reasoning_text":"thinking"}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"content":"hello"}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"content":" world"}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"role":"assistant","content":null,"reasoning_opaque":"V+l5Rhje"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)},
	}
	outbound, err := copilot.NewOutboundTransformer(copilot.OutboundTransformerParams{
		TokenProvider: signatureReplayTokenProvider{},
	})
	require.NoError(t, err)
	unified, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(upstream))
	require.NoError(t, err)
	inbound := NewInboundTransformer()
	stream, err := inbound.TransformStream(t.Context(), unified)
	require.NoError(t, err)
	defer stream.Close()

	var chunks []*httpclient.StreamEvent
	var streamedParts []*Part
	for stream.Next() {
		chunk := stream.Current()
		chunks = append(chunks, chunk)
		var response GenerateContentResponse
		require.NoError(t, json.Unmarshal(chunk.Data, &response))
		streamedParts = append(streamedParts, response.Candidates[0].Content.Parts...)
	}
	require.NoError(t, stream.Err())
	require.Len(t, chunks, 4)
	require.Contains(t, string(chunks[3].Data), `"text":""`)
	require.Len(t, streamedParts, 4)
	require.Equal(t, "V+l5Rhje", streamedParts[3].ThoughtSignature)

	body, _, err := inbound.AggregateStreamChunks(t.Context(), chunks)
	require.NoError(t, err)
	var response GenerateContentResponse
	require.NoError(t, json.Unmarshal(body, &response))
	require.Equal(t, "STOP", response.Candidates[0].FinishReason)
	require.NotNil(t, response.UsageMetadata)
	require.Equal(t, int64(15), response.UsageMetadata.TotalTokenCount)
	require.Equal(t, streamedParts, response.Candidates[0].Content.Parts)
	require.Contains(t, string(body), `"text":""`)

	for name, content := range map[string]*Content{
		"stream":     {Role: "model", Parts: streamedParts},
		"aggregated": response.Candidates[0].Content,
	} {
		t.Run(name, func(t *testing.T) {
			message, err := convertGeminiContentToLLMMessage(content, nil)
			require.NoError(t, err)
			require.NotNil(t, message.ReasoningSignature)
			require.Equal(t, "V+l5Rhje", *message.ReasoningSignature)
			request, err := outbound.TransformRequest(t.Context(), &llm.Request{
				Model: "gemini-3.8-flash", Messages: []llm.Message{*message},
			})
			require.NoError(t, err)
			var replay struct {
				Messages []struct {
					Opaque string `json:"reasoning_opaque"`
				} `json:"messages"`
			}
			require.NoError(t, json.Unmarshal(request.Body, &replay))
			require.Len(t, replay.Messages, 1)
			require.Equal(t, "V+l5Rhje", replay.Messages[0].Opaque)
		})
	}
}

func TestSignaturePartJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		part Part
		want string
	}{
		{"standalone", Part{ThoughtSignature: "opaque"}, `{"text":"","thoughtSignature":"opaque"}`},
		{"empty", Part{}, `{}`},
		{"text", Part{Text: "answer", ThoughtSignature: "opaque"}, `{"text":"answer","thoughtSignature":"opaque"}`},
		{"tool", Part{FunctionCall: &FunctionCall{Name: "Read"}, ThoughtSignature: "opaque"}, `{"functionCall":{"name":"Read"},"thoughtSignature":"opaque"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.part)
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(data))
			var decoded Part
			require.NoError(t, json.Unmarshal(data, &decoded))
			require.Equal(t, tc.part, decoded)
		})
	}
}
