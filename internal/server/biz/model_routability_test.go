package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestHasCapableEndpointForModel(t *testing.T) {
	t.Parallel()

	chatModel := &ent.Model{Type: model.TypeChat}
	embeddingModel := &ent.Model{Type: model.TypeEmbedding}

	openaiChannel := &ent.Channel{Type: channel.TypeOpenai}
	responsesOnlyChannel := &ent.Channel{
		Type:      channel.TypeOpenaiResponses,
		Endpoints: []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	}
	embeddingOnlyChannel := &ent.Channel{
		Type:      channel.TypeOpenai,
		Endpoints: []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIEmbedding.String()}},
	}

	require.False(t, hasCapableEndpointForModel(nil, []*ModelChannelConnection{{Channel: openaiChannel}}))
	require.False(t, hasCapableEndpointForModel(chatModel, nil))
	require.True(t, hasCapableEndpointForModel(chatModel, []*ModelChannelConnection{{Channel: openaiChannel}}))
	require.True(t, hasCapableEndpointForModel(chatModel, []*ModelChannelConnection{{Channel: responsesOnlyChannel}}))
	require.False(t, hasCapableEndpointForModel(embeddingModel, []*ModelChannelConnection{{Channel: responsesOnlyChannel}}))
	require.True(t, hasCapableEndpointForModel(embeddingModel, []*ModelChannelConnection{{Channel: embeddingOnlyChannel}}))
	require.True(t, hasCapableEndpointForModel(chatModel, []*ModelChannelConnection{
		{Channel: embeddingOnlyChannel},
		{Channel: openaiChannel},
	}))
}
