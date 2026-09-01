package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsOpenAIResponsesFormat(t *testing.T) {
	require.True(t, IsOpenAIResponsesFormat(APIFormatOpenAIResponse))
	require.True(t, IsOpenAIResponsesFormat(APIFormatOpenAIResponseCompact))
	require.False(t, IsOpenAIResponsesFormat(APIFormatOpenAIChatCompletion))
	require.False(t, IsOpenAIResponsesFormat(APIFormatAnthropicMessage))
}
