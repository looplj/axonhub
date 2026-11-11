package objects

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestPluginHookScopesAcceptLLMRegistries(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"enabled": true, "requestTypes": llm.AllRequestTypes()})
	require.NoError(t, err)
	var request PluginRequestHook
	require.NoError(t, json.Unmarshal(raw, &request))
	require.Equal(t, llm.AllRequestTypes(), request.RequestTypes)
	raw, err = json.Marshal(map[string]any{"enabled": true, "apiFormats": llm.AllAPIFormats()})
	require.NoError(t, err)
	var attempt PluginAttemptHook
	require.NoError(t, json.Unmarshal(raw, &attempt))
	require.Equal(t, llm.AllAPIFormats(), attempt.APIFormats)
	// Declaration does not imply JSON encoding: image edits may be JSON or multipart.
	require.True(t, attempt.Matches(llm.APIFormatOpenAIImageEdit))
	require.True(t, attempt.Matches(llm.APIFormatOpenAITranscription))
}
