package gemini

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

// A developer-only request is promoted into systemInstruction, leaving
// contents empty. Gemini requires at least one content entry, so the
// conversion must fail with a clear error.
func TestOutboundTransformer_DeveloperOnlyMessage(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-key")
	require.NoError(t, err)

	result, err := outbound.TransformRequest(t.Context(), &llm.Request{
		Model: "gemini-2.5-flash",
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

// A developer message must reach Gemini as a systemInstruction, matching the
// Anthropic path, while real turns stay in contents.
func TestOutboundTransformer_DeveloperMessageBecomesSystemInstruction(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-key")
	require.NoError(t, err)

	result, err := outbound.TransformRequest(t.Context(), &llm.Request{
		Model: "gemini-2.5-flash",
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

	var geminiReq GenerateContentRequest
	require.NoError(t, json.Unmarshal(result.Body, &geminiReq))
	require.NotNil(t, geminiReq.SystemInstruction)
	require.Len(t, geminiReq.SystemInstruction.Parts, 1)
	require.Equal(t, "instruction text", geminiReq.SystemInstruction.Parts[0].Text)
	require.Len(t, geminiReq.Contents, 1)
	require.Equal(t, "user", geminiReq.Contents[0].Role)
}

// The conversion itself promotes a developer-only message into
// systemInstruction; only the resulting empty contents is invalid, which
// TransformRequest rejects above.
func TestConvertLLMToGeminiRequest_DeveloperOnlyMessageBecomesSystemInstruction(t *testing.T) {
	result := convertLLMToGeminiRequest(&llm.Request{
		Model: "gemini-2.5-flash",
		Messages: []llm.Message{
			{
				Role:    "developer",
				Content: llm.MessageContent{Content: lo.ToPtr("instruction text")},
			},
		},
	})

	require.NotNil(t, result.SystemInstruction)
	require.Len(t, result.SystemInstruction.Parts, 1)
	require.Equal(t, "instruction text", result.SystemInstruction.Parts[0].Text)
	require.Empty(t, result.Contents)
}
