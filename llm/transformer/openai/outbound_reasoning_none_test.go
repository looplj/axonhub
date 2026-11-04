package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
)

func newTestTransformer(t *testing.T, omitNone bool) *OutboundTransformer {
	t.Helper()
	tr, err := NewOutboundTransformerWithConfig(&Config{
		PlatformType:            PlatformOpenAI,
		BaseURL:                 "https://example.com/v1",
		APIKeyProvider:          auth.NewStaticKeyProvider("k"),
		OmitReasoningEffortNone: omitNone,
	})
	require.NoError(t, err)
	return tr.(*OutboundTransformer)
}

func TestOutbound_OmitsUnifiedReasoningEffortNoneWhenEnabled(t *testing.T) {
	tr := newTestTransformer(t, true)

	wire, err := tr.TransformRequest(context.Background(), &llm.Request{
		Model: "mm-default",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("hi")},
		}},
		ReasoningEffort: llm.ReasoningEffortNone,
	})
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.NotContains(t, body, "reasoning_effort")
}

func TestOutbound_PreservesReasoningEffortNoneByDefault(t *testing.T) {
	tr := newTestTransformer(t, false)

	wire, err := tr.TransformRequest(context.Background(), &llm.Request{
		Model: "mm-default",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("hi")},
		}},
		ReasoningEffort: llm.ReasoningEffortNone,
	})
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.Equal(t, llm.ReasoningEffortNone, body["reasoning_effort"])
}

func TestOutbound_KeepsSupportedReasoningEffort(t *testing.T) {
	tr := newTestTransformer(t, true)

	wire, err := tr.TransformRequest(context.Background(), &llm.Request{
		Model: "mm-default",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("hi")},
		}},
		ReasoningEffort: llm.ReasoningEffortHigh,
	})
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.Equal(t, llm.ReasoningEffortHigh, body["reasoning_effort"])
}
