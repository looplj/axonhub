package responses

import (
	"context"
	"encoding/json"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	chatoutbound "github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// G14b: known stream_options must live on the dedicated RawStreamOptions
// sidecar, never inflate RawTopLevelFields / UnknownTopLevelFieldCount.
func TestG14b_KnownStreamOptionsDoNotPolluteRawTopLevelFields(t *testing.T) {
	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model": "o3",
		"input": "hello",
		"stream": true,
		"stream_options": {
			"include_obfuscation": false,
			"reasoning_summary_delivery": "sequential_cutoff"
		}
	}`)})
	require.NoError(t, err)
	require.NotNil(t, llmReq.ProviderExtensions)
	require.NotNil(t, llmReq.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, llmReq.ProviderExtensions.OpenAIResponses.Request)

	requestExt := llmReq.ProviderExtensions.OpenAIResponses.Request
	require.NotEmpty(t, requestExt.RawStreamOptions)
	var rawStreamOptions map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(requestExt.RawStreamOptions, &rawStreamOptions))
	require.JSONEq(t, `false`, string(rawStreamOptions["include_obfuscation"]))
	require.JSONEq(t, `"sequential_cutoff"`, string(rawStreamOptions["reasoning_summary_delivery"]))

	_, hasStreamOptionsInTop := requestExt.RawTopLevelFields["stream_options"]
	require.False(t, hasStreamOptionsInTop,
		"stream_options is a known Responses field and must not live in RawTopLevelFields")
	require.Empty(t, requestExt.RawTopLevelFields,
		"known-only stream_options request must leave RawTopLevelFields empty")
}

// G14b public/cross-protocol: a Responses request that only carries known
// stream_options must not raise UnknownTopLevelFieldCount or a false
// LossyDowngrade solely because of that known field when converting to Chat.
func TestG14b_KnownStreamOptionsDoNotTriggerFalseUnknownTopLevelLossyDowngrade(t *testing.T) {
	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model": "gpt-4o",
		"input": "hello",
		"stream": true,
		"stream_options": {
			"include_obfuscation": false,
			"reasoning_summary_delivery": "sequential_cutoff"
		}
	}`)})
	require.NoError(t, err)
	llmReq.Model = "gpt-4o"

	chatOut, err := chatoutbound.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	_, err = chatOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	diagnosticsPtr := llm.ResponsesLossySummaryOf(llmReq)
	ok := diagnosticsPtr != nil
	var diagnostics shared.ResponsesLossyDowngradeDiagnostics
	if ok {
		diagnostics = *diagnosticsPtr
	}
	if ok {
		require.Equal(t, 0, diagnostics.UnknownTopLevelFieldCount,
			"known stream_options must not count as unknown top-level")
		// LossyDowngrade may still be false entirely; if present for other
		// reasons it must not be caused by UnknownTopLevelFieldCount alone.
		if diagnostics.LossyDowngrade {
			require.True(t,
				diagnostics.ClientMetadataCount > 0 ||
					diagnostics.NamespaceToolCount > 0 ||
					diagnostics.ToolSearchToolCount > 0 ||
					diagnostics.UnknownToolCount > 0 ||
					diagnostics.RawOnlyToolCount > 0 ||
					diagnostics.AdditionalToolsCount > 0 ||
					diagnostics.RawInputItemCount > 0,
				"LossyDowngrade must not be triggered solely by known stream_options",
			)
		}
	} else {
		// No diagnostics entry is the expected happy path when nothing is lossy.
		require.False(t, ok)
	}
}

// G14b defensive G9 merge: a non-object typed stream_options value must not
// cause the raw overlay to be discarded.
func TestG14b_MergeStreamOptions_KeepsRawWhenTypedIsNotObject(t *testing.T) {
	obj := map[string]json.RawMessage{
		"stream_options": json.RawMessage(`"not-an-object"`),
	}
	raw := json.RawMessage(`{"reasoning_summary_delivery":"sequential_cutoff","future_nested":{"x":1}}`)

	mergeOpenAIResponsesStreamOptions(obj, raw)

	require.Contains(t, obj, "stream_options")
	var merged map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(obj["stream_options"], &merged))
	require.JSONEq(t, `"sequential_cutoff"`, string(merged["reasoning_summary_delivery"]))
	require.JSONEq(t, `{"x":1}`, string(merged["future_nested"]))
}

// G14b clone non-alias proof at the public ProviderExtensions boundary after
// inbound capture of stream_options.
func TestG14b_RawStreamOptionsCloneIsNotAliasAfterInbound(t *testing.T) {
	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model": "o3",
		"input": "hello",
		"stream": true,
		"stream_options": {
			"include_obfuscation": false,
			"reasoning_summary_delivery": "sequential_cutoff"
		}
	}`)})
	require.NoError(t, err)

	src := llmReq.ProviderExtensions
	require.NotNil(t, src)
	require.NotEmpty(t, src.OpenAIResponses.Request.RawStreamOptions)

	cloned := llm.CloneProviderExtensions(src)
	require.NotNil(t, cloned)
	require.JSONEq(t,
		string(src.OpenAIResponses.Request.RawStreamOptions),
		string(cloned.OpenAIResponses.Request.RawStreamOptions),
	)

	srcPtr := unsafe.SliceData(src.OpenAIResponses.Request.RawStreamOptions)
	clonedPtr := unsafe.SliceData(cloned.OpenAIResponses.Request.RawStreamOptions)
	require.False(t, srcPtr == clonedPtr, "clone must not share the underlying array")

	original := append(json.RawMessage(nil), src.OpenAIResponses.Request.RawStreamOptions...)
	cloned.OpenAIResponses.Request.RawStreamOptions[2] = 'Z'
	require.JSONEq(t, string(original), string(src.OpenAIResponses.Request.RawStreamOptions))
}
