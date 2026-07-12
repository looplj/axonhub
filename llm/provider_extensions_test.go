package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloneProviderExtensions_RawStreamOptionsDeepCopy(t *testing.T) {
	srcBytes := []byte(`{"include_obfuscation":false,"reasoning_summary_delivery":"sequential_cutoff"}`)
	src := &ProviderExtensions{
		OpenAIResponses: &OpenAIResponsesProviderExtensions{
			Request: &OpenAIResponsesRequestExtensions{
				RawStreamOptions: append(json.RawMessage(nil), srcBytes...),
			},
		},
	}

	cloned := CloneProviderExtensions(src)
	require.NotNil(t, cloned)
	require.NotNil(t, cloned.OpenAIResponses)
	require.NotNil(t, cloned.OpenAIResponses.Request)
	require.JSONEq(t, string(srcBytes), string(cloned.OpenAIResponses.Request.RawStreamOptions))

	// Mutating the clone must not alias into the source buffer.
	require.True(t, len(cloned.OpenAIResponses.Request.RawStreamOptions) > 0)
	cloned.OpenAIResponses.Request.RawStreamOptions[2] = 'X'
	require.NotEqual(t,
		string(src.OpenAIResponses.Request.RawStreamOptions),
		string(cloned.OpenAIResponses.Request.RawStreamOptions),
		"RawStreamOptions clone must be a deep copy, not an alias",
	)
	require.JSONEq(t, string(srcBytes), string(src.OpenAIResponses.Request.RawStreamOptions))
}
