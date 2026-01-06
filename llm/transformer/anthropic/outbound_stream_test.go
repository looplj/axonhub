package anthropic

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

func TestOutboundTransformer_StreamTransformation_WithTestData(t *testing.T) {
	tests := []struct {
		name         string
		streamFile   string
		expectedFile string
		platformType PlatformType
	}{
		{
			name:         "response with stop finish reason",
			streamFile:   "anthropic-stop.stream.jsonl",
			expectedFile: "llm-stop.stream.jsonl",
			platformType: PlatformDirect,
		},
		{
			name:         "response with tool calls",
			streamFile:   "anthropic-tool.stream.jsonl",
			expectedFile: "llm-tool.stream.jsonl",
			platformType: PlatformDirect,
		},
		{
			name:         "response with thinking",
			streamFile:   "anthropic-think.stream.jsonl",
			expectedFile: "llm-think.stream.jsonl",
			platformType: PlatformDirect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, _ := NewOutboundTransformer("https://example.com", string(tt.platformType))

			streamEvents, err := xtest.LoadStreamChunks(t, tt.streamFile)
			require.NoError(t, err)

			mockStream := streams.SliceStream(streamEvents)

			transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
			require.NoError(t, err)

			var actualResponses []*llm.Response

			for transformedStream.Next() {
				resp := transformedStream.Current()
				actualResponses = append(actualResponses, resp)
			}

			require.NoError(t, transformedStream.Err())

			expectedResponses, err := xtest.LoadLlmResponses(t, tt.expectedFile)
			require.NoError(t, err)

			for i, expected := range expectedResponses {
				actual := actualResponses[i]

				require.Equal(t, expected.ID, actual.ID, "Response %d: ID should match", i)
				require.Equal(t, expected.Object, actual.Object, "Response %d: Object should match", i)
				require.Equal(t, expected.Model, actual.Model, "Response %d: Model should match", i)
				require.Equal(t, expected.Created, actual.Created, "Response %d: Created should match", i)

				require.Equal(t, len(expected.Choices), len(actual.Choices), "Response %d: Number of choices should match", i)

				if len(expected.Choices) > 0 && len(actual.Choices) > 0 {
					expectedChoice := expected.Choices[0]
					actualChoice := actual.Choices[0]

					require.Equal(t, expectedChoice.Index, actualChoice.Index, "Response %d: Choice index should match", i)
					require.Equal(t, expectedChoice.FinishReason, actualChoice.FinishReason, "Response %d: Finish reason should match", i)

					if !xtest.Equal(expectedChoice.Delta, actualChoice.Delta) {
						t.Fatalf("diff: %s  at index %d", cmp.Diff(expectedChoice.Delta, actualChoice.Delta), i)
					}
				}

				if !xtest.Equal(expected.Usage, actual.Usage) {
					t.Fatalf("diff: %s  at index %d", cmp.Diff(expected.Usage, actual.Usage), i)
				}
			}
		})
	}
}
