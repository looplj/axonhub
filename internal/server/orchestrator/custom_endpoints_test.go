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
}
