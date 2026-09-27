package openai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

// streamEvents builds stream events from raw SSE data payloads.
func streamEvents(payloads ...string) []*httpclient.StreamEvent {
	events := make([]*httpclient.StreamEvent, 0, len(payloads))
	for _, p := range payloads {
		events = append(events, &httpclient.StreamEvent{Data: []byte(p)})
	}

	return events
}

// TestAggregateStreamChunks_CitationsTopLevel ensures citations carried on
// stream chunks (Perplexity/OpenRouter style) are projected back to the
// top-level `citations` field of the aggregated chat completion — the same
// wire position used by streaming chunks and by the direct non-streaming
// path (ResponseFromLLM) — instead of leaking the internal
// `transformer_metadata` envelope, which is documented as never serialized.
func TestAggregateStreamChunks_CitationsTopLevel(t *testing.T) {
	chunks := streamEvents(
		`{"id":"chatcmpl-cit","object":"chat.completion.chunk","created":1720000000,"model":"sonar-pro","citations":["https://example.com/c","https://example.com/b"],"choices":[{"index":0,"delta":{"role":"assistant","content":"Answer "},"finish_reason":null}]}`,
		`{"id":"chatcmpl-cit","object":"chat.completion.chunk","created":1720000000,"model":"sonar-pro","citations":["https://example.com/b","https://example.com/a"],"choices":[{"index":0,"delta":{"content":"here."},"finish_reason":"stop"}]}`,
		`{"id":"chatcmpl-cit","object":"chat.completion.chunk","created":1720000000,"model":"sonar-pro","choices":[],"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16}}`,
		`[DONE]`,
	)

	body, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	// The internal envelope must not leak into the client-facing body.
	require.NotContains(t, strings.ToLower(string(body)), "transformer_metadata")

	var resp Response
	require.NoError(t, json.Unmarshal(body, &resp))

	// Citations are deduplicated, sorted, and top-level — matching the shape a
	// streaming client observes on the chunks themselves.
	require.Equal(t, []string{
		"https://example.com/a",
		"https://example.com/b",
		"https://example.com/c",
	}, resp.Citations)

	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message)
	require.NotNil(t, resp.Choices[0].Message.Content.Content)
	require.Equal(t, "Answer here.", *resp.Choices[0].Message.Content.Content)
	require.NotNil(t, resp.Choices[0].FinishReason)
	require.Equal(t, "stop", *resp.Choices[0].FinishReason)
	require.NotNil(t, resp.Usage)
	require.Equal(t, int64(16), resp.Usage.TotalTokens)
}

// TestAggregateStreamChunks_NoCitationsUnchanged guards the no-citations path:
// the aggregated body must stay byte-identical in shape (no citations key, no
// transformer_metadata key) for streams that never carried citations.
func TestAggregateStreamChunks_NoCitationsUnchanged(t *testing.T) {
	chunks := streamEvents(
		`{"id":"chatcmpl-plain","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}]}`,
		`[DONE]`,
	)

	body, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)
	require.NotContains(t, strings.ToLower(string(body)), "transformer_metadata")
	require.NotContains(t, string(body), "citations")
	require.Equal(t, `{"id":"chatcmpl-plain","choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"object":"chat.completion","created":1720000000,"model":"gpt-4o"}`, string(body))
}
