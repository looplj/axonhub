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

	alphaSearch := CapableAPIFormats(RequestTypeAlphaSearch)
	require.Contains(t, alphaSearch, APIFormatOpenAIAlphaSearch.String())
	require.NotContains(t, alphaSearch, APIFormatOpenAIResponse.String())

	systemOne := CapableAPIFormats(RequestTypeSystemOne)
	require.NotNil(t, systemOne)
	require.Contains(t, systemOne, APIFormatTypeSafeSystemOne.String())
	require.NotContains(t, systemOne, APIFormatOpenAIChatCompletion.String())

	require.Nil(t, CapableAPIFormats(RequestType("unknown")))
}

func TestCapableAPIFormats_IncludesModelScopeImage(t *testing.T) {
	t.Parallel()

	image := CapableAPIFormats(RequestTypeImage)
	require.Contains(t, image, APIFormatModelScopeImage.String())

	chat := CapableAPIFormats(RequestTypeChat)
	require.NotContains(t, chat, APIFormatModelScopeImage.String())

	video := CapableAPIFormats(RequestTypeVideo)
	require.NotContains(t, video, APIFormatModelScopeImage.String())
}

func TestRequestTypeForModelType(t *testing.T) {
	t.Parallel()

	require.Equal(t, RequestTypeChat, RequestTypeForModelType("chat"))
	require.Equal(t, RequestTypeEmbedding, RequestTypeForModelType("embedding"))
	require.Equal(t, RequestTypeRerank, RequestTypeForModelType("rerank"))
	require.Equal(t, RequestTypeImage, RequestTypeForModelType("image_generation"))
	require.Equal(t, RequestTypeVideo, RequestTypeForModelType("video_generation"))
	require.Equal(t, RequestTypeSystemOne, RequestTypeForModelType("systemone"))
	require.Equal(t, RequestTypeSystemOne, RequestTypeForModelType("system_one"))
	require.Empty(t, RequestTypeForModelType("unknown"))
}
