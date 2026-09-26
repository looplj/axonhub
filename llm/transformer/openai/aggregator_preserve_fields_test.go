package openai

import (
	"context"
	"encoding/json"
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

// TestAggregateStreamChunks_PreservesRefusal ensures a refusal streamed as
// delta.refusal chunks survives aggregation into the non-streaming response.
// On stream-required channels the aggregated body is what the client receives;
// dropping the refusal turns a model refusal into an empty successful message.
func TestAggregateStreamChunks_PreservesRefusal(t *testing.T) {
	chunks := streamEvents(
		`{"id":"chatcmpl-ref","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		`{"id":"chatcmpl-ref","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"refusal":"I'm sorry, but I "},"finish_reason":null}]}`,
		`{"id":"chatcmpl-ref","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"refusal":"can't help with that."},"finish_reason":"stop"}]}`,
		`{"id":"chatcmpl-ref","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":8,"total_tokens":18}}`,
		`[DONE]`,
	)

	body, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var resp Response
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message)
	require.Equal(t, "I'm sorry, but I can't help with that.", resp.Choices[0].Message.Refusal)
	require.NotNil(t, resp.Choices[0].FinishReason)
	require.Equal(t, "stop", *resp.Choices[0].FinishReason)
}

// TestAggregateStreamChunks_PreservesAudio ensures audio output streamed as
// delta.audio chunks (id/expires_at on the first chunk, data and transcript
// split across chunks) is reassembled into message.audio.
func TestAggregateStreamChunks_PreservesAudio(t *testing.T) {
	chunks := streamEvents(
		`{"id":"chatcmpl-aud","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o-audio-preview","choices":[{"index":0,"delta":{"role":"assistant","audio":{"id":"audio_abc","expires_at":1730000000,"data":"QUJD","transcript":"Hello"}},"finish_reason":null}]}`,
		`{"id":"chatcmpl-aud","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o-audio-preview","choices":[{"index":0,"delta":{"audio":{"data":"REVG","transcript":" world"}},"finish_reason":"stop"}]}`,
		`[DONE]`,
	)

	body, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var resp Response
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message)
	require.NotNil(t, resp.Choices[0].Message.Audio)
	require.Equal(t, "audio_abc", resp.Choices[0].Message.Audio.ID)
	require.Equal(t, "QUJDREVG", resp.Choices[0].Message.Audio.Data)
	require.Equal(t, int64(1730000000), resp.Choices[0].Message.Audio.ExpiresAt)
	require.Equal(t, "Hello world", resp.Choices[0].Message.Audio.Transcript)
}

// TestAggregateStreamChunks_PreservesLogprobs ensures per-chunk logprobs are
// concatenated instead of dropped when a client requested logprobs on a
// stream-required channel.
func TestAggregateStreamChunks_PreservesLogprobs(t *testing.T) {
	chunks := streamEvents(
		`{"id":"chatcmpl-lp","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"logprobs":{"content":[{"token":"Hi","logprob":-0.1,"bytes":[72,105]}]},"finish_reason":null}]}`,
		`{"id":"chatcmpl-lp","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" there"},"logprobs":{"content":[{"token":" there","logprob":-0.2,"bytes":[32,116,104,101,114,101],"top_logprobs":[{"token":" here","logprob":-1.5}]}]},"finish_reason":"stop"}]}`,
		`[DONE]`,
	)

	body, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var resp Response
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Logprobs)
	require.Len(t, resp.Choices[0].Logprobs.Content, 2)
	require.Equal(t, "Hi", resp.Choices[0].Logprobs.Content[0].Token)
	require.Equal(t, " there", resp.Choices[0].Logprobs.Content[1].Token)
	require.Len(t, resp.Choices[0].Logprobs.Content[1].TopLogprobs, 1)
	require.Equal(t, " here", resp.Choices[0].Logprobs.Content[1].TopLogprobs[0].Token)

	require.NotNil(t, resp.Choices[0].Message)
	require.NotNil(t, resp.Choices[0].Message.Content.Content)
	require.Equal(t, "Hi there", *resp.Choices[0].Message.Content.Content)
}

// TestAggregateStreamChunks_PreservesReasoningAndServiceTier ensures the
// reasoning field (used by providers such as Synthetic instead of
// reasoning_content) and the response-level service_tier survive aggregation.
func TestAggregateStreamChunks_PreservesReasoningAndServiceTier(t *testing.T) {
	chunks := streamEvents(
		`{"id":"chatcmpl-rea","object":"chat.completion.chunk","created":1720000000,"model":"hf:synthetic-model","service_tier":"default","choices":[{"index":0,"delta":{"role":"assistant","reasoning":"thinking "},"finish_reason":null}]}`,
		`{"id":"chatcmpl-rea","object":"chat.completion.chunk","created":1720000000,"model":"hf:synthetic-model","service_tier":"default","choices":[{"index":0,"delta":{"reasoning":"hard","content":"done"},"finish_reason":"stop"}]}`,
		`[DONE]`,
	)

	body, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var resp Response
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Equal(t, "default", resp.ServiceTier)
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message)
	require.NotNil(t, resp.Choices[0].Message.Reasoning)
	require.Equal(t, "thinking hard", *resp.Choices[0].Message.Reasoning)
}
