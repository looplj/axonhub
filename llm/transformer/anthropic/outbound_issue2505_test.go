package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

// A developer-only request leaves no conversation turn after the developer
// message is promoted to the system prompt. Anthropic rejects an empty
// messages array, so the conversion must fail with a clear error.
func TestOutboundTransformer_DeveloperOnlyMessage(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	result, err := outbound.TransformRequest(t.Context(), &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		Messages: []llm.Message{
			{
				Role:    "developer",
				Content: llm.MessageContent{Content: lo.ToPtr("instruction text")},
			},
		},
	})

	require.Error(t, err)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Nil(t, result)
}

// A developer message must still be promoted to the system prompt when the
// request also carries a real turn.
func TestOutboundTransformer_DeveloperMessageBecomesSystem(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	result, err := outbound.TransformRequest(t.Context(), &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		Messages: []llm.Message{
			{
				Role:    "developer",
				Content: llm.MessageContent{Content: lo.ToPtr("instruction text")},
			},
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello!")},
			},
		},
	})
	require.NoError(t, err)

	var anthropicReq MessageRequest
	require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
	require.NotNil(t, anthropicReq.System)
	require.Equal(t, "instruction text", lo.FromPtr(anthropicReq.System.Prompt))
	require.Len(t, anthropicReq.Messages, 1)
	require.Equal(t, "user", anthropicReq.Messages[0].Role)
}

// input_audio has no Anthropic equivalent. Dropping the part silently would
// send "content": null, so the request must be rejected with an error that
// names the part type.
func TestOutboundTransformer_InputAudioContentPart(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	result, err := outbound.TransformRequest(t.Context(), &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type:       "input_audio",
							InputAudio: &llm.InputAudio{Format: "wav", Data: "UklGRiQAAABXQVZF"},
						},
					},
				},
			},
		},
	})

	require.Error(t, err)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Contains(t, err.Error(), "input_audio")
	require.Nil(t, result)
}

// input_audio is not the only part type without an Anthropic representation.
// A document part (produced e.g. by the Responses API input_file item) or a
// video_url part is dropped by convertMultiplePartContent just the same, so a
// message made up only of such parts would be sent as "content": null. The
// request must fail instead of losing the content silently.
func TestOutboundTransformer_AllPartsDropped(t *testing.T) {
	tests := []struct {
		name string
		part llm.MessageContentPart
	}{
		{
			name: "document",
			part: llm.MessageContentPart{
				Type: "document",
				Document: &llm.DocumentURL{
					URL:      "data:application/pdf;base64,JVBERi0=",
					Filename: "report.pdf",
					MIMEType: "application/pdf",
				},
			},
		},
		{
			name: "video_url",
			part: llm.MessageContentPart{
				Type:     "video_url",
				VideoURL: &llm.VideoURL{URL: "https://example.com/clip.mp4"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
			require.NoError(t, err)

			result, err := outbound.TransformRequest(t.Context(), &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{tt.part}},
					},
				},
			})

			require.Error(t, err)
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
			require.Contains(t, err.Error(), tt.part.Type)
			require.Nil(t, result)
		})
	}
}

// input_audio must be rejected even when the message also carries text, so the
// request fails up front rather than sending a message with the audio silently
// missing.
func TestOutboundTransformer_TextWithInputAudioContentPart(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	result, err := outbound.TransformRequest(t.Context(), &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("transcribe this")},
						{Type: "input_audio", InputAudio: &llm.InputAudio{Format: "wav", Data: "UklGRiQAAABXQVZF"}},
					},
				},
			},
		},
	})

	require.Error(t, err)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Contains(t, err.Error(), "input_audio")
	require.Nil(t, result)
}

// Regression: a tool turn legitimately has no text. The assistant tool_use and
// the tool_result blocks are built explicitly, so the content validation must
// not reject them.
func TestOutboundTransformer_ToolTurnWithoutText(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	result, err := outbound.TransformRequest(t.Context(), &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("What is the weather in Paris?")},
			},
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"city":"Paris"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_1"),
				Content:    llm.MessageContent{Content: lo.ToPtr("sunny")},
			},
		},
	})
	require.NoError(t, err)

	var anthropicReq MessageRequest
	// MessageContent.UnmarshalJSON rejects null content, so a successful
	// unmarshal also proves that no message is sent as "content": null.
	require.NoError(t, json.Unmarshal(result.Body, &anthropicReq))
	require.Len(t, anthropicReq.Messages, 3)

	require.Equal(t, "user", anthropicReq.Messages[0].Role)
	require.Equal(t, "What is the weather in Paris?", lo.FromPtr(anthropicReq.Messages[0].Content.Content))

	assistant := anthropicReq.Messages[1]
	require.Equal(t, "assistant", assistant.Role)
	require.Len(t, assistant.Content.MultipleContent, 1)
	require.Equal(t, "tool_use", assistant.Content.MultipleContent[0].Type)
	require.Equal(t, "call_1", assistant.Content.MultipleContent[0].ID)

	toolResult := anthropicReq.Messages[2]
	require.Equal(t, "user", toolResult.Role)
	require.Len(t, toolResult.Content.MultipleContent, 1)
	require.Equal(t, "tool_result", toolResult.Content.MultipleContent[0].Type)
	require.Equal(t, "call_1", lo.FromPtr(toolResult.Content.MultipleContent[0].ToolUseID))
	require.Equal(t, "sunny", lo.FromPtr(toolResult.Content.MultipleContent[0].Content.Content))
}
