package openrouter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openrouter"
	"github.com/looplj/axonhub/llm/transformer/responseschat"
)

func TestOutboundTransformer_ResponsesRequestCapabilities(t *testing.T) {
	outbound, err := openrouter.NewOutboundTransformer("https://api.example.com/v1", "test-api-key")
	require.NoError(t, err)

	provider, ok := outbound.(*openrouter.OutboundTransformer)
	require.True(t, ok)
	require.True(t, provider.ResponsesRequestCapabilities(&llm.Request{}).ChatToolLifecycle)
	require.False(t, provider.ResponsesRequestCapabilities(&llm.Request{RequestType: llm.RequestTypeCompact}).ChatToolLifecycle)
}

func TestOutboundTransformer_ResponsesToolLifecycle(t *testing.T) {
	outbound, err := openrouter.NewOutboundTransformer("https://api.example.com/v1", "test-api-key")
	require.NoError(t, err)

	responseschat.RequireToolLifecycle(t, outbound, "openrouter/auto")
}
