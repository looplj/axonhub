package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapableAPIFormats(t *testing.T) {
	t.Parallel()

	chat := CapableAPIFormats(RequestTypeChat)
	require.Contains(t, chat, APIFormatOpenAIChatCompletion.String())
	require.Contains(t, chat, APIFormatOpenAIResponse.String())
	require.NotContains(t, chat, APIFormatOpenAIEmbedding.String())

	embedding := CapableAPIFormats(RequestTypeEmbedding)
	require.Contains(t, embedding, APIFormatOpenAIEmbedding.String())
	require.NotContains(t, embedding, APIFormatOpenAIChatCompletion.String())

	require.Nil(t, CapableAPIFormats(RequestType("unknown")))
}

func TestRequestTypeForModelType(t *testing.T) {
	t.Parallel()

	require.Equal(t, RequestTypeChat, RequestTypeForModelType("chat"))
	require.Equal(t, RequestTypeEmbedding, RequestTypeForModelType("embedding"))
	require.Equal(t, RequestTypeRerank, RequestTypeForModelType("rerank"))
	require.Equal(t, RequestTypeImage, RequestTypeForModelType("image_generation"))
	require.Equal(t, RequestTypeVideo, RequestTypeForModelType("video_generation"))
	require.Empty(t, RequestTypeForModelType("unknown"))
}
