package transformer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
	openairesponses "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type responsesCapabilityOverride struct {
	transformer.Outbound

	capabilities transformer.ResponsesRequestCapabilities
}

func (t responsesCapabilityOverride) ResponsesRequestCapabilities(*llm.Request) transformer.ResponsesRequestCapabilities {
	return t.capabilities
}

func TestResponsesRequestCapabilitiesOf(t *testing.T) {
	native, err := openairesponses.NewOutboundTransformer("https://responses.example.com", "test-key")
	require.NoError(t, err)

	require.True(t, transformer.ResponsesRequestCapabilitiesOf(native, &llm.Request{}).NativeResponses)

	overridden := responsesCapabilityOverride{
		Outbound:     native,
		capabilities: transformer.ResponsesRequestCapabilities{ChatToolLifecycle: true},
	}
	got := transformer.ResponsesRequestCapabilitiesOf(overridden, &llm.Request{})
	require.False(t, got.NativeResponses)
	require.True(t, got.ChatToolLifecycle)
}
