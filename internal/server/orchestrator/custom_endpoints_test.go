package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestSelectAPIFormatForRequestType(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
		{APIFormat: "openai/embeddings"},
		{APIFormat: "openai/image_generation"},
	}

	require.Equal(t, "openai/responses", SelectAPIFormatForRequestType(endpoints, llm.RequestTypeChat))
	require.Equal(t, "openai/embeddings", SelectAPIFormatForRequestType(endpoints, llm.RequestTypeEmbedding))
	require.Equal(t, "openai/image_generation", SelectAPIFormatForRequestType(endpoints, llm.RequestTypeImage))

	geminiEndpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatGeminiContents.String()},
		{APIFormat: llm.APIFormatGeminiEmbedding.String()},
	}

	require.Equal(t, llm.APIFormatGeminiContents.String(), SelectAPIFormatForRequestType(geminiEndpoints, llm.RequestTypeChat))
	require.Equal(t, llm.APIFormatGeminiEmbedding.String(), SelectAPIFormatForRequestType(geminiEndpoints, llm.RequestTypeEmbedding))
	require.Equal(t, llm.APIFormatGeminiContents.String(), SelectAPIFormatForRequestType(geminiEndpoints, llm.RequestTypeImage))
}

func TestSelectAPIFormatForRequest_PrefersInboundResponsesFormat(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatOpenAIResponse.String()},
	}

	req := &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIResponse,
	}

	require.Equal(t, llm.APIFormatOpenAIResponse.String(), SelectAPIFormatForRequest(endpoints, req))
}

func TestSelectAPIFormatForRequest_NonResponsesKeepsRequestTypeSelection(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatOpenAIResponse.String()},
	}

	req := &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIChatCompletion,
	}

	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), SelectAPIFormatForRequest(endpoints, req))
}
