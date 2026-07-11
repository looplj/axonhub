package responses

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
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

func TestResponsesOutboundStreamReasoningTextToCommon(t *testing.T) {
	events := []*httpclient.StreamEvent{
		{Type: string(StreamEventTypeResponseCreated), Data: []byte(`{"type":"response.created","response":{"id":"resp_rt","object":"response","created_at":1700000000,"model":"gpt-5.1","status":"in_progress","output":[]}}`)},
		{Type: string(StreamEventTypeOutputItemAdded), Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","status":"in_progress"}}`)},
		{Type: string(StreamEventTypeReasoningTextDelta), Data: []byte(`{"type":"response.reasoning_text.delta","item_id":"rs_1","output_index":0,"content_index":0,"delta":"think-a"}`)},
		{Type: string(StreamEventTypeReasoningTextDelta), Data: []byte(`{"type":"response.reasoning_text.delta","item_id":"rs_1","output_index":0,"content_index":0,"delta":"think-b"}`)},
		{Type: string(StreamEventTypeReasoningTextDone), Data: []byte(`{"type":"response.reasoning_text.done","item_id":"rs_1","output_index":0,"content_index":0,"text":"think-athink-b"}`)},
		{Type: string(StreamEventTypeOutputItemDone), Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","status":"completed","content":[{"type":"reasoning_text","text":"think-athink-b"}]}}`)},
		{Type: string(StreamEventTypeResponseCompleted), Data: []byte(`{"type":"response.completed","response":{"id":"resp_rt","object":"response","created_at":1700000000,"model":"gpt-5.1","status":"completed","output":[{"id":"rs_1","type":"reasoning","status":"completed","content":[{"type":"reasoning_text","text":"think-athink-b"}]}]}}`)},
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	stream, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	var gotReasoning strings.Builder
	var sawPreferText bool
	for stream.Next() {
		resp := stream.Current()
		require.NotNil(t, resp)
		if resp.TransformerMetadata != nil {
			if v, ok := resp.TransformerMetadata[responsesReasoningPreferTextStreamTransformerMetadataKey].(bool); ok && v {
				sawPreferText = true
			}
		}
		if len(resp.Choices) == 0 || resp.Choices[0].Delta == nil || resp.Choices[0].Delta.ReasoningContent == nil {
			continue
		}
		gotReasoning.WriteString(*resp.Choices[0].Delta.ReasoningContent)
	}
	require.NoError(t, stream.Err())
	require.Equal(t, "think-athink-b", gotReasoning.String())
	require.True(t, sawPreferText, "outbound stream must mark prefer-text for production re-emit")
}

func TestResponsesAggregatorReasoningTextContent(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Type: string(StreamEventTypeResponseCreated), Data: []byte(`{"type":"response.created","response":{"id":"resp_agg","object":"response","created_at":1,"model":"gpt-5.1","status":"in_progress","output":[]}}`)},
		{Type: string(StreamEventTypeOutputItemAdded), Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_agg","type":"reasoning","status":"in_progress"}}`)},
		{Type: string(StreamEventTypeReasoningTextDelta), Data: []byte(`{"type":"response.reasoning_text.delta","item_id":"rs_agg","output_index":0,"content_index":0,"delta":"alpha"}`)},
		{Type: string(StreamEventTypeReasoningTextDone), Data: []byte(`{"type":"response.reasoning_text.done","item_id":"rs_agg","output_index":0,"content_index":0,"text":"alpha"}`)},
		{Type: string(StreamEventTypeOutputItemDone), Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_agg","type":"reasoning","status":"completed","content":[{"type":"reasoning_text","text":"alpha"}]}}`)},
		{Type: string(StreamEventTypeResponseCompleted), Data: []byte(`{"type":"response.completed","response":{"id":"resp_agg","object":"response","created_at":1,"model":"gpt-5.1","status":"completed","output":[]}}`)},
	}

	body, meta, err := AggregateStreamChunks(t.Context(), chunks)
	require.NoError(t, err)
	require.NotEmpty(t, body)

	var resp Response
	require.NoError(t, json.Unmarshal(body, &resp))
	require.NotEmpty(t, resp.Output)
	require.Equal(t, "reasoning", resp.Output[0].Type)
	require.Len(t, resp.Output[0].ReasoningContent, 1)
	require.Equal(t, "reasoning_text", resp.Output[0].ReasoningContent[0].Type)
	require.Equal(t, "alpha", resp.Output[0].ReasoningContent[0].Text)
	_ = meta
}
