package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

// TestStream_RedactedThinkingPreserved covers #9: a streaming
// content_block_start of type redacted_thinking must surface its Data on the
// assistant delta's RedactedReasoningContent, instead of being dropped.
func TestStream_RedactedThinkingPreserved(t *testing.T) {
	redactedData := "encrypted-redacted-thinking-blob"

	cbData, err := json.Marshal(StreamEvent{
		Type:  "content_block_start",
		Index: lo.ToPtr(int64(0)),
		ContentBlock: &MessageContentBlock{
			Type: "redacted_thinking",
			Data: redactedData,
		},
	})
	require.NoError(t, err)

	event := &httpclient.StreamEvent{
		Type: "content_block_start",
		Data: cbData,
	}

	s := newOutboundStream(nil, PlatformDirect)
	resp, err := s.transformStreamChunk(event)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Delta)
	require.NotNil(t, resp.Choices[0].Delta.RedactedReasoningContent)
	require.Equal(t, redactedData, *resp.Choices[0].Delta.RedactedReasoningContent)
}

// TestStream_PauseTurnFinishReason covers #17: streaming and non-streaming
// must map pause_turn consistently. Non-streaming maps pause_turn -> "stop";
// streaming previously left it as the raw "pause_turn" (inconsistent).
func TestStream_PauseTurnFinishReason(t *testing.T) {
	stopReason := "pause_turn"
	deltaData, err := json.Marshal(StreamEvent{
		Type: "message_delta",
		Delta: &StreamDelta{
			StopReason: &stopReason,
		},
	})
	require.NoError(t, err)

	event := &httpclient.StreamEvent{
		Type: "message_delta",
		Data: deltaData,
	}

	s := newOutboundStream(nil, PlatformDirect)
	resp, err := s.transformStreamChunk(event)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].FinishReason)
	require.Equal(t, "stop", *resp.Choices[0].FinishReason)
}
