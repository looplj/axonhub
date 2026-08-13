package cerebras

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
)

func TestOutboundTransformer_ResponsesRequestCapabilities(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        DefaultBaseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	})
	require.NoError(t, err)

	provider, ok := outbound.(*OutboundTransformer)
	require.True(t, ok)
	require.True(t, provider.ResponsesRequestCapabilities(&llm.Request{}).ChatToolLifecycle)
	require.False(t, provider.ResponsesRequestCapabilities(&llm.Request{RequestType: llm.RequestTypeCompact}).ChatToolLifecycle)
}
