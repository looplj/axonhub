package responses

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestResponsesReasoningGenerateSummaryKeepsDeprecatedIdentity(t *testing.T) {
	body, err := os.ReadFile("testdata/reasoning-generate-summary.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	require.Equal(t, "medium", llmReq.ReasoningEffort)
	require.NotNil(t, llmReq.ReasoningSummary)
	require.Equal(t, "detailed", *llmReq.ReasoningSummary)
	origin, ok := llmReq.TransformerMetadata[responsesReasoningGenerateSummaryOriginTransformerMetadataKey].(bool)
	require.True(t, ok)
	require.True(t, origin)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	var reasoning map[string]any
	require.NoError(t, json.Unmarshal(outboundBody["reasoning"], &reasoning))
	require.Equal(t, "medium", reasoning["effort"])
	require.Equal(t, "detailed", reasoning["generate_summary"])
	_, hasSummary := reasoning["summary"]
	require.False(t, hasSummary, "generate_summary-only request must not rewrite to summary")
}

func TestResponsesReasoningSummaryAndGenerateSummaryBothPreserved(t *testing.T) {
	body, err := os.ReadFile("testdata/reasoning-summary-and-generate-summary.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	require.NotNil(t, llmReq.ReasoningSummary)
	require.Equal(t, "auto", *llmReq.ReasoningSummary)
	gen, ok := llmReq.TransformerMetadata[responsesReasoningGenerateSummaryValueTransformerMetadataKey].(string)
	require.True(t, ok)
	require.Equal(t, "concise", gen)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	var reasoning map[string]any
	require.NoError(t, json.Unmarshal(outboundBody["reasoning"], &reasoning))
	require.Equal(t, "auto", reasoning["summary"])
	require.Equal(t, "concise", reasoning["generate_summary"])
}

func TestResponsesReasoningOutputContentPreserved(t *testing.T) {
	body, err := os.ReadFile("testdata/reasoning-output-content.response.json")
	require.NoError(t, err)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	})
	require.NoError(t, err)
	require.NotEmpty(t, llmResp.Choices)
	msg := llmResp.Choices[0].Message
	require.NotNil(t, msg)
	require.NotNil(t, msg.ReasoningContent)
	// Prefer reasoning_text content over summary when both exist, or combine without loss.
	require.Contains(t, *msg.ReasoningContent, "raw reasoning text body")
	require.NotNil(t, msg.ReasoningSignature)
	require.Equal(t, "enc_abc", *msg.ReasoningSignature)

	// Same-protocol Responses client response should re-emit content[] reasoning_text.
	inbound := NewInboundTransformer()
	httpResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)
	var clientBody map[string]any
	require.NoError(t, json.Unmarshal(httpResp.Body, &clientBody))
	output := clientBody["output"].([]any)
	require.NotEmpty(t, output)
	first := output[0].(map[string]any)
	require.Equal(t, "reasoning", first["type"])
	content, ok := first["content"].([]any)
	require.True(t, ok, "reasoning content[] must survive: %v", first)
	require.NotEmpty(t, content)
	part := content[0].(map[string]any)
	require.Equal(t, "reasoning_text", part["type"])
	require.Equal(t, "raw reasoning text body", part["text"])
}

func TestResponsesReasoningUnknownNestedPreserved(t *testing.T) {
	body, err := os.ReadFile("testdata/reasoning-unknown-nested.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	raw, ok := llmReq.TransformerMetadata[responsesReasoningRawObjectTransformerMetadataKey].(json.RawMessage)
	require.True(t, ok, "raw reasoning object must be preserved for unknown nested keys")
	require.Contains(t, string(raw), "future_nested")
	require.Contains(t, string(raw), "another_future")

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	var reasoning map[string]any
	require.NoError(t, json.Unmarshal(outboundBody["reasoning"], &reasoning))
	require.Equal(t, "low", reasoning["effort"])
	require.Equal(t, "auto", reasoning["context"])
	require.Equal(t, "auto", reasoning["summary"])
	require.Contains(t, reasoning, "future_nested")
	require.Equal(t, "keep-me", reasoning["another_future"])
}

func TestResponsesReasoningTextStreamEvents(t *testing.T) {
	// Common stream chunks carrying reasoning content should emit reasoning_text
	// stream events on the Responses inbound path, not only summary events.
	meta := map[string]any{responsesReasoningPreferTextStreamTransformerMetadataKey: true}
	chunks := []*llm.Response{
		{
			ID:     "resp_stream_1",
			Object: "response",
			Model:  "gpt-5.1",
			TransformerMetadata: meta,
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role:             "assistant",
					ReasoningContent: loPtr("think-1"),
				},
			}},
		},
		{
			ID:     "resp_stream_1",
			Object: "response",
			Model:  "gpt-5.1",
			TransformerMetadata: meta,
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ReasoningContent: loPtr("think-2"),
				},
			}},
		},
		{
			ID:     "resp_stream_1",
			Object: "response",
			Model:  "gpt-5.1",
			TransformerMetadata: meta,
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: loPtr("stop"),
				Message: &llm.Message{
					Role:    "assistant",
					Content: llm.MessageContent{Content: loPtr("done")},
				},
			}},
		},
	}

	inbound := NewInboundTransformer()
	stream, err := inbound.TransformStream(t.Context(), streams.SliceStream(chunks))
	require.NoError(t, err)

	var eventTypes []string
	var sawReasoningTextDelta bool
	var sawReasoningTextDone bool
	for stream.Next() {
		ev := stream.Current()
		require.NotNil(t, ev)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(ev.Data, &payload))
		typ, _ := payload["type"].(string)
		eventTypes = append(eventTypes, typ)
		if typ == string(StreamEventTypeReasoningTextDelta) {
			sawReasoningTextDelta = true
			require.Contains(t, payload["delta"], "think")
		}
		if typ == string(StreamEventTypeReasoningTextDone) {
			sawReasoningTextDone = true
		}
	}
	require.NoError(t, stream.Err())
	require.True(t, sawReasoningTextDelta, "expected reasoning_text.delta events, got %v", eventTypes)
	require.True(t, sawReasoningTextDone, "expected reasoning_text.done event, got %v", eventTypes)
}

func loPtr[T any](v T) *T { return &v }
