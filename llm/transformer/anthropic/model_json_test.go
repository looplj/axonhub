package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestMessageContentBlockMarshalJSON_PreservesEmptyThinkingSignature(t *testing.T) {
	data, err := json.Marshal(MessageContentBlock{
		Type:      "thinking",
		Thinking:  lo.ToPtr(""),
		Signature: lo.ToPtr(""),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"thinking","thinking":"","signature":""}`, string(data))
}

func TestMessageContentBlockMarshalJSON_PreservesNilThinkingSignature(t *testing.T) {
	data, err := json.Marshal(MessageContentBlock{
		Type: "thinking",
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"thinking","thinking":"","signature":""}`, string(data))
}

func TestStreamDeltaMarshalJSON_OmitsSignatureForThinkingDelta(t *testing.T) {
	data, err := json.Marshal(StreamDelta{
		Type:      lo.ToPtr("thinking_delta"),
		Thinking:  lo.ToPtr("Thinking..."),
		Signature: lo.ToPtr(""),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"thinking_delta","thinking":"Thinking..."}`, string(data))
}
