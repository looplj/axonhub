package anthropic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

// TestOutboundTransformer_FinishReason_AlwaysIncludesDelta is a regression test
// that verifies the finish_reason event always includes a Delta field, even if empty.
//
// CRITICAL: The openai-go client (and potentially other OpenAI-compatible clients)
// expects ALL streaming chunks to have a "delta" field present in the JSON.
// When a chunk contains "finish_reason" without "delta", it causes JSON unmarshalling
// errors in the openai-go client's streaming parser.
//
// OpenAI's actual streaming format includes "delta": {} (empty object) in the
// finish_reason chunk, like this:
//
//	{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}
//
// Without the delta field, clients see:
//
//	{"choices":[{"index":0,"finish_reason":"stop"}]}
//
// This breaks the openai-go client's expectation that delta is always present,
// causing errors like: "json: cannot unmarshal object into Go struct field..."
//
// Regression test for: Missing delta field in finish_reason events causes
// OpenAI client compatibility issues.
func TestOutboundTransformer_FinishReason_AlwaysIncludesDelta(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.anthropic.com", string(PlatformDirect))
	require.NoError(t, err)

	// Load actual Anthropic stream test data
	streamEvents, err := xtest.LoadStreamChunks(t, "anthropic-stop.stream.jsonl")
	require.NoError(t, err)

	mockStream := streams.SliceStream(streamEvents)
	transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
	require.NoError(t, err)

	var responses []*llm.Response

	for transformedStream.Next() {
		resp := transformedStream.Current()
		responses = append(responses, resp)
	}

	require.NoError(t, transformedStream.Err())

	// Find the response with finish_reason
	var finishReasonResponse *llm.Response

	for _, resp := range responses {
		if len(resp.Choices) > 0 && resp.Choices[0].FinishReason != nil {
			finishReasonResponse = resp
			break
		}
	}

	require.NotNil(t, finishReasonResponse, "Should have a response with finish_reason")
	require.Len(t, finishReasonResponse.Choices, 1, "Should have exactly one choice")

	choice := finishReasonResponse.Choices[0]

	// CRITICAL ASSERTION: Delta must be present (even if empty) when finish_reason is set
	// This is required for OpenAI client compatibility
	assert.NotNil(t, choice.Delta,
		"Delta must not be nil when finish_reason is present (required for openai-go client compatibility)")
	assert.Equal(t, "stop", *choice.FinishReason, "Finish reason should be 'stop'")
}

// TestOutboundTransformer_AllStreamingChunks_HaveDelta verifies that every
// streaming chunk with choices includes a Delta field. This is a comprehensive
// test to ensure we never regress on the delta field requirement.
func TestOutboundTransformer_AllStreamingChunks_HaveDelta(t *testing.T) {
	testCases := []struct {
		name       string
		streamFile string
	}{
		{"stop finish reason", "anthropic-stop.stream.jsonl"},
		{"tool calls", "anthropic-tool.stream.jsonl"},
		{"thinking", "anthropic-think.stream.jsonl"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer("https://api.anthropic.com", string(PlatformDirect))
			require.NoError(t, err)

			streamEvents, err := xtest.LoadStreamChunks(t, tc.streamFile)
			require.NoError(t, err)

			mockStream := streams.SliceStream(streamEvents)
			transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
			require.NoError(t, err)

			var responses []*llm.Response

			for transformedStream.Next() {
				resp := transformedStream.Current()
				responses = append(responses, resp)
			}

			require.NoError(t, transformedStream.Err())

			// Verify every response with choices has Delta
			for i, resp := range responses {
				// Skip [DONE] marker and empty choice responses
				if resp.Object == "[DONE]" || len(resp.Choices) == 0 {
					continue
				}

				choice := resp.Choices[0]
				assert.NotNil(t, choice.Delta,
					"Response %d in %s: Choice must have Delta field (required for OpenAI client compatibility)",
					i, tc.streamFile)
			}
		})
	}
}
