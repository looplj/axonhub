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

func TestStreamDeltaMarshalJSON_PreservesEmptyThinkingSignatureForThinkingBlock(t *testing.T) {
	data, err := json.Marshal(StreamDelta{
		Type:      lo.ToPtr("thinking"),
		Thinking:  lo.ToPtr(""),
		Signature: lo.ToPtr(""),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"thinking","thinking":"","signature":""}`, string(data))
}

func TestStreamDeltaMarshalJSON_PreservesNilThinkingSignatureForThinkingBlock(t *testing.T) {
	data, err := json.Marshal(StreamDelta{
		Type: lo.ToPtr("thinking"),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"thinking","thinking":"","signature":""}`, string(data))
}

func TestStreamDeltaMarshalJSON_PreservesNilThinkingForThinkingDelta(t *testing.T) {
	data, err := json.Marshal(StreamDelta{
		Type: lo.ToPtr("thinking_delta"),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"thinking_delta","thinking":""}`, string(data))
}

func TestStreamDeltaMarshalJSON_PreservesEmptySignatureForSignatureDelta(t *testing.T) {
	data, err := json.Marshal(StreamDelta{
		Type:      lo.ToPtr("signature_delta"),
		Signature: lo.ToPtr(""),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"signature_delta","signature":""}`, string(data))
}

func TestStreamDeltaMarshalJSON_PreservesNilSignatureForSignatureDelta(t *testing.T) {
	data, err := json.Marshal(StreamDelta{
		Type: lo.ToPtr("signature_delta"),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"signature_delta","signature":""}`, string(data))
}
